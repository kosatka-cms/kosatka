package videoembed

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// embedTemplate - the module's template of a video in a text (a theme
// may have its own), and its block.
const (
	embedTemplate = "video_embed/embed.html"
	embedBlock    = "video_embed"
)

// embedView - what the template shows: the link, the player's address
// (Player - starting at once, after a click on the preview), the
// service's name, the title and the preview, the proportions.
type embedView struct {
	URL         string
	Player      string
	Provider    string
	Title       string
	Preview     string
	Width       int
	Height      int
	Portrait    bool
	ClickToPlay bool
}

// Filter turns the video links standing alone in a paragraph into the
// videos (the module's text filter).
type Filter struct {
	service   *Service
	templates modulekit.TemplateService
	logger    *slog.Logger
}

// NewFilter creates a Filter.
func NewFilter(service *Service, templates modulekit.TemplateService, logger *slog.Logger) *Filter {
	return &Filter{service: service, templates: templates, logger: logger}
}

// TextFilter - the module's text filter, with the editor's "Video"
// button (modulekit.AddTextFilter).
func (f *Filter) TextFilter() modulekit.TextFilter {
	return modulekit.TextFilter{
		Name: manifest.Name, Module: manifest.Name,
		Label:        i18n.N("Video links as players"),
		Description:  i18n.N("A link to a video on its own line shows as its player (Configuration → Video embeds). The editor gets the \"Video\" button."),
		Apply:        f.Apply,
		EditorButton: "kosatkavideo",
		EditorPlugin: "video_embed_editor",
	}
}

// standaloneLink - the link a paragraph holds and nothing else (its text,
// or a link element; spaces and a <br> aside): "" - none.
func standaloneLink(p *html.Node) string {
	link := ""
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		switch {
		case c.Type == html.TextNode && strings.TrimSpace(c.Data) == "":
		case c.Type == html.ElementNode && c.DataAtom == atom.Br:
		case link != "":
			return ""
		case c.Type == html.TextNode:
			link = strings.TrimSpace(c.Data)
		case c.Type == html.ElementNode && c.DataAtom == atom.A:
			for _, a := range c.Attr {
				if a.Key == "href" {
					link = strings.TrimSpace(a.Val)
				}
			}
			if link == "" {
				return ""
			}
		default:
			return ""
		}
	}
	return link
}

// Apply returns s with the video links standing alone in a paragraph
// made the videos; s itself when it has none.
func (f *Filter) Apply(ctx context.Context, s string) string {
	if !strings.Contains(s, "http") {
		return s
	}
	settings := f.service.Settings(ctx)
	on := settings.On()
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(s), body)
	if err != nil {
		return s
	}
	// The fragment's nodes come without a parent: under body, a paragraph
	// of the top level has one to be replaced in.
	for _, n := range nodes {
		body.AppendChild(n)
	}
	type found struct {
		p *html.Node
		m Match
	}
	var videos []found
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.P {
			if m, ok := MatchURL(standaloneLink(n), on); ok {
				videos = append(videos, found{n, m})
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(body)
	if len(videos) == 0 {
		return s
	}

	matches := make([]Match, len(videos))
	for i, v := range videos {
		matches[i] = v.m
	}
	embeds := f.service.Embeds(ctx, matches)
	rendered := make([]string, len(videos))
	for i, v := range videos {
		out, err := f.render(ctx, v.m, embeds[v.m.URL], settings)
		if err != nil {
			modulekit.LogFallback(f.logger, err, "rendering a video embed (the link shown)", "url", v.m.URL)
			continue
		}
		rendered[i] = out
		// The paragraph's place: a marker, the video put there after the
		// fragment is rendered back.
		v.p.Parent.InsertBefore(&html.Node{Type: html.CommentNode, Data: fmt.Sprintf("video-embed:%d", i)}, v.p)
		v.p.Parent.RemoveChild(v.p)
	}
	var buf bytes.Buffer
	for n := body.FirstChild; n != nil; n = n.NextSibling {
		if err := html.Render(&buf, n); err != nil {
			return s
		}
	}
	out := buf.String()
	for i, r := range rendered {
		if r != "" {
			out = strings.Replace(out, fmt.Sprintf("<!--video-embed:%d-->", i), r, 1)
		}
	}
	return out
}

// render - the video m as the template shows it.
func (f *Filter) render(ctx context.Context, m Match, e Embed, settings Settings) (string, error) {
	view := videoView(m, e, settings, settings.ClickToPlay)
	var buf bytes.Buffer
	data := map[string]any{"Video": view, modulekit.RequestLanguageKey: i18n.FromContext(ctx)}
	if err := f.templates.RenderNamed(&buf, embedBlock, data, embedTemplate); err != nil {
		return "", fmt.Errorf("rendering %s: %w", embedTemplate, err)
	}
	return buf.String(), nil
}

// videoView - the video m (what's known of it: e) as the template shows
// it: a preview starting the player on a click (clickToPlay), else the
// player.
func videoView(m Match, e Embed, settings Settings, clickToPlay bool) embedView {
	view := embedView{URL: m.URL, Player: m.Player, Provider: m.Provider.Label, Title: e.Title,
		Width: e.Width, Height: e.Height, ClickToPlay: clickToPlay}
	if m.Width > 0 && m.Height > 0 {
		view.Width, view.Height = m.Width, m.Height
	}
	if view.Width <= 0 || view.Height <= 0 {
		view.Width, view.Height = 16, 9
	}
	view.Portrait = view.Height > view.Width
	if clickToPlay {
		view.Player = m.PlayerAutoplay()
	}
	switch {
	case e.ThumbnailFile != "":
		view.Preview = modulekit.StyledImage(modulekit.File{URI: "public://" + e.ThumbnailFile}, manifest.PreviewStyle).URI
	case e.ThumbnailURL != "" && !settings.LocalPreviews:
		view.Preview = e.ThumbnailURL
	}
	return view
}
