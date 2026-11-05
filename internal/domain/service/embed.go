package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/embedtag"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// Embeds: a tag [[<kind>:<name>]] in a text whose format has the "Embeds"
// filter on, or {{embed "<kind>:<name>"}} in a template, shows in its
// place what the kind renders for the name (a block, a View's list). The
// text is processed when shown, but without the request (the template
// functions are the template set's), so it only marks the place
// (embedMarker); the page (TemplateService.Render) fills the marks in
// the request's context - the visitor's access, language and zone
// (ResolveEmbeds). A fragment written as it is (a lazy block) calls
// ResolveEmbeds itself.

// EmbedKind - a kind of embedded thing.
type EmbedKind struct {
	// Name - the tag's kind ("block", "view"); unique.
	Name string
	// Module - the module the kind belongs to ("" - the core): off with it.
	Module string
	// Render - what name embeds, for ctx's visitor (their access,
	// language, zone; the request's); "" - nothing for them. A missing
	// name is an entity.ErrEntityNotFound error.
	Render func(ctx context.Context, name string) (template.HTML, error)

	// The editor's "Insert" window and the cards over the tags in a text
	// (all three set - the kind is offered there):
	// Label - the kind's name (i18n.N);
	// Search - the things matching query (by title; "" - any), at most
	// limit, as ctx's visitor (the editor) may see them: Name, Title, Meta
	// and a thumbnail at most;
	// Describe - name's thing in full, for the window's preview and the
	// card; a missing one - entity.ErrEntityNotFound.
	Label    string
	Search   func(ctx context.Context, query string, limit int) ([]EmbedOption, error)
	Describe func(ctx context.Context, name string) (EmbedOption, error)
}

// EmbedOption - a thing to embed as the editor shows it: in the "Insert"
// window's list and preview, and in the card over its tag.
type EmbedOption struct {
	// Name - the tag's name ([[kind:<Name>]]).
	Name  string `json:"name"`
	Title string `json:"title"`
	// Meta - a line under the title ("12 photos · From "The trip"").
	Meta string `json:"meta,omitempty"`
	// Thumbs - pictures of it (the list shows the first).
	Thumbs []string `json:"thumbs,omitempty"`
	// Lines - a few lines of what it shows: a block's text, a list's
	// titles, a poll's options, a form's questions; More - how many more
	// than Thumbs and Lines show.
	Lines []string `json:"lines,omitempty"`
	More  int      `json:"more,omitempty"`
	// Note - who sees it, or why nobody does (unpublished).
	Note string `json:"note,omitempty"`
}

// EmbedsFilter - the text filter's name.
const EmbedsFilter = "embeds"

// maxEmbedDepth - how deep embeds embed: a block in a block in a block.
const maxEmbedDepth = 3

var (
	embedKindsMu sync.RWMutex
	embedKinds   = map[string]EmbedKind{}
	// embedOrder - the kinds' names in registration order (the window's).
	embedOrder []string
	// embedMarkerPrefix - the start of a mark, with a nonce of the process:
	// a text can't hold one of its own.
	embedMarkerPrefix = []byte("<!--kosatka-embed:" + embedNonce() + ":")
	embedMarkerSuffix = []byte("-->")
)

func embedNonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // never fails (crypto/rand)
	return hex.EncodeToString(b)
}

func init() {
	RegisterTextFilter(TextFilter{
		Name:        EmbedsFilter,
		Label:       i18n.N("Embeds"),
		Description: i18n.N("[[block:<name>]] shows a block in its place, [[view:<name>]] a View block's list (by their machine names), and the modules' kinds theirs; each as its visitor may see it."),
		Apply:       markEmbeds,
		// The "Insert…" button: the window choosing what to embed, and the
		// cards over the tags (web/static/js/embeds_editor.js).
		EditorButton: "kosatkaembed",
		EditorPlugin: "embeds_editor",
	})
}

// RegisterEmbedKind adds a kind of embedded thing; one of the same name
// replaces it (the tests build the app more than once).
func RegisterEmbedKind(k EmbedKind) {
	embedKindsMu.Lock()
	defer embedKindsMu.Unlock()
	if _, ok := embedKinds[k.Name]; !ok {
		embedOrder = append(embedOrder, k.Name)
	}
	embedKinds[k.Name] = k
}

// EmbedPickerKind - a kind the editor's "Insert" window offers.
type EmbedPickerKind struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// EmbedPickerKinds - the kinds the editor's window offers (EmbedKind's
// Label, Search and Describe set), of the enabled modules, in the order
// registered; Label translated for ctx.
func EmbedPickerKinds(ctx context.Context) []EmbedPickerKind {
	embedKindsMu.RLock()
	defer embedKindsMu.RUnlock()
	var out []EmbedPickerKind
	for _, name := range embedOrder {
		k := embedKinds[name]
		if k.Label == "" || k.Search == nil || k.Describe == nil || !ModuleEnabled(k.Module) {
			continue
		}
		out = append(out, EmbedPickerKind{Name: k.Name, Label: i18n.T(ctx, k.Label)})
	}
	return out
}

// SearchEmbeds - the things of kind matching query, for the editor's
// window (EmbedKind.Search); an unknown kind or one not offered - none.
func SearchEmbeds(ctx context.Context, kind, query string, limit int) ([]EmbedOption, error) {
	k, ok := embedKind(kind)
	if !ok || k.Search == nil {
		return nil, nil
	}
	opts, err := k.Search(ctx, strings.TrimSpace(query), limit)
	if err != nil {
		return nil, fmt.Errorf("searching the %s embeds: %w", kind, err)
	}
	return opts, nil
}

// DescribeEmbed - what tag ("kind:name") embeds, for the editor's card
// and preview (EmbedKind.Describe); ok=false - nothing: an invalid tag,
// an unknown kind, a missing thing.
func DescribeEmbed(ctx context.Context, tag string) (EmbedOption, bool, error) {
	t, ok := embedtag.Parse(tag)
	if !ok {
		return EmbedOption{}, false, nil
	}
	k, ok := embedKind(t.Kind)
	if !ok || k.Describe == nil {
		return EmbedOption{}, false, nil
	}
	opt, err := k.Describe(ctx, t.Name)
	if entity.IsNotFound(err) {
		return EmbedOption{}, false, nil
	}
	if err != nil {
		return EmbedOption{}, false, fmt.Errorf("describing %s: %w", t, err)
	}
	opt.Name = t.Name
	return opt, true, nil
}

// embedKind - the kind named name, of an enabled module.
func embedKind(name string) (EmbedKind, bool) {
	embedKindsMu.RLock()
	k, ok := embedKinds[name]
	embedKindsMu.RUnlock()
	return k, ok && ModuleEnabled(k.Module)
}

// embedMarker - the mark of tag's place, filled in by ResolveEmbeds.
func embedMarker(tag embedtag.Tag) string {
	return string(embedMarkerPrefix) + tag.Kind + ":" + tag.Name + string(embedMarkerSuffix)
}

// embedAloneRe - a paragraph holding only a tag: the embedded thing (a
// block's <div>) takes the paragraph's place, not a place inside it.
var embedAloneRe = regexp.MustCompile(`<p(?:\s[^>]*)?>(?:\s|&nbsp;|&#160;)*(\[\[[a-z][a-z0-9_]*:[a-z0-9][a-z0-9_-]*\]\])(?:\s|&nbsp;|&#160;)*</p>`)

// markEmbeds - the "Embeds" filter: html with the tags of the known kinds
// marked (embedMarker); a tag inside a tag's attribute, or of a kind
// nobody registered, stays as written.
func markEmbeds(_ context.Context, html string) string {
	if !strings.Contains(html, "[[") {
		return html
	}
	mark := func(tag embedtag.Tag) (string, bool) {
		if _, ok := embedKind(tag.Kind); !ok {
			return "", false
		}
		return embedMarker(tag), true
	}
	html = embedAloneRe.ReplaceAllStringFunc(html, func(p string) string {
		tag, _ := embedtag.Parse(embedAloneRe.FindStringSubmatch(p)[1])
		if m, ok := mark(tag); ok {
			return m
		}
		return p
	})
	return outsideTags(html, func(text string) string { return embedtag.Replace(text, mark) })
}

// outsideTags - html with f applied to its text between the tags (never
// inside a tag, its attributes).
func outsideTags(html string, f func(string) string) string {
	var b strings.Builder
	b.Grow(len(html))
	for html != "" {
		i := strings.IndexByte(html, '<')
		if i < 0 {
			b.WriteString(f(html))
			break
		}
		b.WriteString(f(html[:i]))
		j := strings.IndexByte(html[i:], '>')
		if j < 0 {
			b.WriteString(html[i:])
			break
		}
		b.WriteString(html[i : i+j+1])
		html = html[i+j+1:]
	}
	return b.String()
}

// EmbedHTML - the template function embed: the mark of what tag
// ("<kind>:<name>") embeds, filled in on the page; an invalid tag or an
// unknown kind - nothing.
func EmbedHTML(tag string) template.HTML {
	t, ok := embedtag.Parse(tag)
	if !ok {
		return ""
	}
	if _, ok := embedKind(t.Kind); !ok {
		return ""
	}
	return template.HTML(embedMarker(t))
}

// ResolveEmbeds fills page's embed marks in with what they embed for
// ctx's visitor; what fails or is missing - nothing (a missing thing
// logged at debug).
func ResolveEmbeds(ctx context.Context, page []byte) []byte {
	return resolveEmbeds(ctx, page, nil)
}

// resolveEmbeds - ResolveEmbeds inside the embeds of chain ("kind:name",
// outermost first): one embedding itself, or deeper than maxEmbedDepth,
// is left out.
func resolveEmbeds(ctx context.Context, page []byte, chain []string) []byte {
	if !bytes.Contains(page, embedMarkerPrefix) {
		return page
	}
	var out bytes.Buffer
	out.Grow(len(page))
	rest := page
	for {
		i := bytes.Index(rest, embedMarkerPrefix)
		if i < 0 {
			out.Write(rest)
			break
		}
		end := bytes.Index(rest[i+len(embedMarkerPrefix):], embedMarkerSuffix)
		if end < 0 {
			out.Write(rest)
			break
		}
		out.Write(rest[:i])
		key := string(rest[i+len(embedMarkerPrefix) : i+len(embedMarkerPrefix)+end])
		rest = rest[i+len(embedMarkerPrefix)+end+len(embedMarkerSuffix):]
		if len(chain) >= maxEmbedDepth || slices.Contains(chain, key) {
			continue
		}
		out.Write(resolveEmbeds(ctx, renderEmbed(ctx, key), append(slices.Clone(chain), key)))
	}
	return out.Bytes()
}

// renderEmbed - what key ("kind:name") embeds for ctx's visitor; nothing
// on a failure.
func renderEmbed(ctx context.Context, key string) []byte {
	tag, ok := embedtag.Parse(key)
	if !ok {
		return nil
	}
	kind, ok := embedKind(tag.Kind)
	if !ok || kind.Render == nil {
		return nil
	}
	html, err := kind.Render(ctx, tag.Name)
	if err != nil {
		LogFallback(slog.Default(), err, "embedding (nothing in its place)", "tag", tag.String())
		return nil
	}
	return []byte(html)
}

// EmbedTemplate renders an embedded thing: the template define of files
// (none - the engine's field templates and the modules' blocks/*.html
// alone) with data, in the zone, language and theme of ctx's request.
func EmbedTemplate(ctx context.Context, templates TemplateService, define string, data map[string]any, files ...string) (template.HTML, error) {
	data[RequestLocationKey] = sitetime.FromContext(ctx)
	data[RequestLanguageKey] = i18n.FromContext(ctx)
	data[RequestThemeKey] = RequestTheme(ctx)
	var buf bytes.Buffer
	if err := templates.RenderNamed(&buf, define, data, files...); err != nil {
		return "", fmt.Errorf("rendering %s: %w", define, err)
	}
	return template.HTML(buf.String()), nil
}

// BlockEmbed - the kind "block": a block by its machine name
// (entity.BlockDef.MachineName), as a region shows it
// (partials/embed.html, embed_block); an unpublished one - nothing.
func BlockEmbed(blocks BlockService, templates TemplateService) EmbedKind {
	return EmbedKind{Name: "block", Label: i18n.N("Block"), Search: func(ctx context.Context, query string, limit int) ([]EmbedOption, error) {
		found, err := blocks.SearchBlocks(ctx, query, limit)
		if err != nil {
			return nil, err
		}
		out := make([]EmbedOption, 0, len(found))
		for _, b := range found {
			if b.MachineName != "" {
				out = append(out, EmbedOption{Name: b.MachineName, Title: b.Title, Meta: b.MachineName})
			}
		}
		return out, nil
	}, Describe: func(ctx context.Context, name string) (EmbedOption, error) {
		b, err := blocks.BlockByMachineName(ctx, name)
		if err != nil {
			return EmbedOption{}, err
		}
		opt := EmbedOption{Title: b.Title, Meta: b.MachineName}
		if text := PlainExcerpt(b.Body, 200); text != "" {
			opt.Lines = []string{text}
		}
		if !b.Status {
			opt.Note = i18n.T(ctx, "Not published: nothing shows in its place.")
		}
		return opt, nil
	}, Render: func(ctx context.Context, name string) (template.HTML, error) {
		b, err := blocks.BlockByMachineName(ctx, name)
		if err != nil {
			return "", err
		}
		if !b.Status {
			return "", nil
		}
		return EmbedTemplate(ctx, templates, "embed_block", map[string]any{"Block": b}, "partials/embed.html")
	}}
}
