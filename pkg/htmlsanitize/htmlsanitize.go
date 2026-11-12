// Package htmlsanitize sanitizes user-submitted HTML (comments, forum posts,
// private messages, profile signatures) before it is stored, stripping
// dangerous markup (<script>, event handlers, javascript: URLs, <iframe>,
// etc.) while keeping the common safe formatting tags a rich-text editor
// produces: bold/italic/underline, links, images, lists, and so on.
package htmlsanitize

import (
	"html"
	"regexp"
	"strings"

	"github.com/kosatka-cms/kosatka/pkg/embedtag"
	"github.com/microcosm-cc/bluemonday"
)

// policy is bluemonday's "user generated content" policy: a broad allowlist
// of formatting tags (b, strong, i, em, u, a, img, p, br, ul/ol/li,
// blockquote, headings, tables, ...) with href/src schemes validated and
// rel="nofollow noopener" added to links. It is safe for concurrent use.
// "class" and "style" are deliberately not allowed: bluemonday doesn't parse
// CSS, and end users must not inject arbitrary styling.
var policy = bluemonday.UGCPolicy()

// cssLengthOrAuto - one CSS length (optionally negative/decimal; the unit is
// optional so a bare "0" matches) or "auto". Nothing else (no url(),
// expression()) can match.
const cssLengthOrAuto = `-?\d+(?:\.\d+)?(?:px|%|em)?|auto`

// imgAppearanceStyle matches "style" values made of any combination, order
// and repetition of TinyMCE's alignment output (text-align on blocks;
// float or display:block + auto margins on images), pixel/percent
// width/height (resize handles), and margins (shorthand or longhand, typed
// in the image dialog's Style field). These usually arrive together, e.g.
// "width: 220px; height: 335px; float: right; margin: 0 15px;", and
// bluemonday requires a full match, so the pattern accepts any sequence of
// clauses while rejecting any other property or value.
var imgAppearanceStyle = regexp.MustCompile(`(?i)^(?:\s*(?:` +
	`text-align\s*:\s*(?:left|right|center|justify)` +
	`|float\s*:\s*(?:left|right)` +
	`|display\s*:\s*block\s*;\s*margin-left\s*:\s*auto\s*;\s*margin-right\s*:\s*auto` +
	`|width\s*:\s*\d+(?:\.\d+)?(?:px|%)` +
	`|height\s*:\s*\d+(?:\.\d+)?(?:px|%)` +
	`|margin\s*:\s*(?:` + cssLengthOrAuto + `)(?:\s+(?:` + cssLengthOrAuto + `)){0,3}` +
	`|margin-(?:top|right|bottom|left)\s*:\s*(?:` + cssLengthOrAuto + `)` +
	// border:0 - emitted by third-party counter/informer image widgets.
	`|border\s*:\s*0` +
	`)\s*;?)*\s*$`)

// targetBlank - the one target a link keeps: a new tab (the editor's
// "Open link in new window"); bluemonday adds rel="noopener" to it.
var targetBlank = regexp.MustCompile(`^_blank$`)

// adminPolicy - policy plus "class" (AllowStyling) and a "style" attribute
// restricted to imgAppearanceStyle. Only for rich text authored by trusted
// admins/moderators (e.g. Views' HeaderText/FooterText), where the editor's
// "code" plugin allows raw HTML.
var adminPolicy = newAdminPolicy()

func newAdminPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowStyling()
	p.AllowAttrs("style").Matching(imgAppearanceStyle).Globally()
	// <kbd> - keyboard-shortcut text; no attributes, no security surface.
	p.AllowElements("kbd")
	// target="_blank" on <a> (links opening in a new tab); exactly
	// "_blank", and rel="nofollow" is still forced on every link.
	p.AllowAttrs("target").Matching(targetBlank).OnElements("a")
	// aria-label - accessibility text (e.g. icon-only links); values are
	// always escaped.
	p.AllowAttrs("aria-label").Globally()
	// data-cid/data-lang on <img> - read by counter/informer widgets;
	// restricted to a narrow alphanumeric charset.
	p.AllowAttrs("data-cid", "data-lang").Matching(regexp.MustCompile(`^[a-zA-Z0-9_-]{0,32}$`)).OnElements("img")
	return p
}

// Sanitize strips unsafe HTML from s, keeping common formatting tags with
// their URLs validated. Safe to call concurrently.
func Sanitize(s string) string {
	return policy.Sanitize(s)
}

// stripPolicy strips every tag, keeping only text.
var stripPolicy = bluemonday.StrictPolicy()

// StripToText renders already-sanitized HTML s as plain text: tags removed
// (block ones - paragraphs, headings, list items, line breaks… - leave a
// space, so their texts don't run together), entities decoded, so
// html/template's auto-escaping doesn't double-encode them ("&amp;" shown
// literally), runs of white space collapsed, the embed tags ([[block:…]],
// embedtag) left out: what they embed isn't the text's. Never use the
// result as HTML. Safe to call concurrently.
func StripToText(s string) string {
	s = blockTagRe.ReplaceAllString(embedtag.Strip(s), " $0")
	return strings.Join(strings.Fields(html.UnescapeString(stripPolicy.Sanitize(s))), " ")
}

// StripToLines is StripToText keeping the paragraphs: each block (a
// paragraph, a heading, a list item, a line after <br>…) is one line, its
// white space collapsed; empty ones are dropped. Never use the result as
// HTML. Safe to call concurrently.
func StripToLines(s string) []string {
	s = blockTagRe.ReplaceAllString(s, "\n$0")
	var lines []string
	for _, line := range strings.Split(html.UnescapeString(stripPolicy.Sanitize(s)), "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// WithoutCode - HTML s without its code blocks (<pre> to its </pre>, any
// case): a preview of a text (an excerpt, a teaser's text, a description)
// is its prose - code run into one line reads as noise. A text that is all
// code stays as it is. The result is s's own HTML, as safe as s was.
func WithoutCode(s string) string {
	if !strings.Contains(strings.ToLower(s), "<pre") {
		return s
	}
	rest := preBlockRe.ReplaceAllString(s, " ")
	if StripToText(rest) == "" {
		return s
	}
	return rest
}

// preBlockRe - a code block, <pre> to its </pre> (WithoutCode).
var preBlockRe = regexp.MustCompile(`(?is)<pre\b[^>]*>.*?</pre\s*>`)

// blockTagRe - an opening or closing block tag, or <br>.
var blockTagRe = regexp.MustCompile(`(?i)<(?:br|/?(?:p|div|h[1-6]|li|ul|ol|dl|dt|dd|pre|blockquote|table|tr|td|th|section|article|header|footer|figure|figcaption|hr))\b[^>]*>`)

// SanitizeAdmin is like Sanitize, but for rich text only admins/moderators
// can author: it also keeps "class" and a restricted "style" attribute
// (see adminPolicy). Safe to call concurrently.
func SanitizeAdmin(s string) string {
	return adminPolicy.Sanitize(s)
}

// editorPolicy - adminPolicy with https embeds (allowIframes).
var editorPolicy = func() *bluemonday.Policy {
	p := newAdminPolicy()
	allowIframes(p)
	return p
}()

// SanitizeEditor sanitizes the HTML of a visual editor that has no text
// format to decide its tags (a text field's legacy editor widget):
// SanitizeAdmin's allowlist plus https embeds, the "read more" cut kept.
// Safe to call concurrently.
func SanitizeEditor(s string) string {
	return keepBreakMarker(s, editorPolicy.Sanitize)
}

// RowTemplateURLToken - the item path token of a Views row template, kept
// by SanitizeAdminRowTemplate even inside href.
const RowTemplateURLToken = "{{url}}"

// rowURLPlaceholder stands in for RowTemplateURLToken while the policy
// runs: plain letters, a valid relative URL the policy keeps as is
// (the braces would come back percent-encoded inside href).
const rowURLPlaceholder = "cmsrowurl5d81c0aa"

// rowTemplatePolicy - adminPolicy for Views row templates: their links
// point at the site's own items, so only fully qualified (external)
// links get rel="nofollow".
var rowTemplatePolicy = func() *bluemonday.Policy {
	p := newAdminPolicy()
	p.RequireNoFollowOnLinks(false)
	p.RequireNoFollowOnFullyQualifiedLinks(true)
	return p
}()

// SanitizeAdminRowTemplate sanitizes a Views row template like
// SanitizeAdmin, keeping the {{url}} token (also as an href value) and
// the {{field:…}} tokens (plain text to the policy); internal links
// stay without rel="nofollow". Safe to call concurrently.
func SanitizeAdminRowTemplate(s string) string {
	s = strings.ReplaceAll(s, rowURLPlaceholder, "")
	out := rowTemplatePolicy.Sanitize(strings.ReplaceAll(s, RowTemplateURLToken, rowURLPlaceholder))
	return strings.ReplaceAll(out, rowURLPlaceholder, RowTemplateURLToken)
}

// SanitizeWithFormat sanitizes s per an entity.TextFormatDef config:
// allowedTags is a space-separated, case-insensitive list of tag names
// (blank means plain text, every tag stripped); allowClass permits "class".
// A fresh policy is built per call, which is fine since formats are applied
// on save, not on render. a/img/list/table tags use bluemonday's dedicated
// helpers (URL validation, rel="nofollow", structural attributes) rather
// than a bare AllowElements. Safe to call concurrently.
func SanitizeWithFormat(s string, allowedTags string, allowClass bool) string {
	tags := strings.Fields(strings.ToLower(allowedTags))
	if len(tags) == 0 {
		return bluemonday.StrictPolicy().Sanitize(s)
	}
	return keepBreakMarker(s, formatPolicy(tags, allowClass).Sanitize)
}

// BreakMarker - the "read more" cut (<!--break-->): the editor's
// button inserts it and teasers are cut at it, so every HTML format keeps
// it, though comments in general are stripped.
const BreakMarker = "<!--break-->"

// breakPlaceholder stands in for BreakMarker while the policy runs -
// plain letters no policy touches (a NUL would come back as U+FFFD from
// the HTML tokenizer).
const breakPlaceholder = "cmsbreakmarker7f3a91c2"

func keepBreakMarker(s string, sanitize func(string) string) string {
	if !strings.Contains(s, BreakMarker) {
		return sanitize(s)
	}
	s = strings.ReplaceAll(s, breakPlaceholder, "")
	out := sanitize(strings.ReplaceAll(s, BreakMarker, breakPlaceholder))
	return strings.ReplaceAll(out, breakPlaceholder, BreakMarker)
}

// iframeSrc - embeds over https only (video players and the like).
var iframeSrc = regexp.MustCompile(`(?i)^https://[^\s"'<>]+$`)

// iframeAllow - the iframe "allow" feature list players use.
var iframeAllow = regexp.MustCompile(`(?i)^[a-z0-9\-;\s=*'()":/.]*$`)

var cssSizeAttr = regexp.MustCompile(`^\d+(?:%|px)?$`)

// allowIframes lets p keep embeds (a trailer player…): https src only,
// their size and the players' own attributes, nothing scriptable.
func allowIframes(p *bluemonday.Policy) {
	p.AllowElements("iframe")
	p.AllowAttrs("src").Matching(iframeSrc).OnElements("iframe")
	p.AllowAttrs("width", "height").Matching(cssSizeAttr).OnElements("iframe")
	p.AllowAttrs("frameborder").Matching(regexp.MustCompile(`^[01]$`)).OnElements("iframe")
	p.AllowAttrs("allow").Matching(iframeAllow).OnElements("iframe")
	p.AllowAttrs("allowfullscreen").Matching(regexp.MustCompile(`(?i)^(|true|allowfullscreen)$`)).OnElements("iframe")
	p.AllowAttrs("title").OnElements("iframe")
	p.AllowAttrs("loading").Matching(regexp.MustCompile(`^(lazy|eager)$`)).OnElements("iframe")
	p.AllowAttrs("referrerpolicy").Matching(regexp.MustCompile(`^[a-z\-]+$`)).OnElements("iframe")
}

func formatPolicy(tags []string, allowClass bool) *bluemonday.Policy {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	p := bluemonday.NewPolicy()
	var generic []string
	for t := range tagSet {
		switch t {
		case "a", "img", "iframe", "ul", "ol", "li", "table", "thead", "tbody", "tfoot", "tr", "td", "th":
			// handled by bluemonday's own dedicated helpers below.
		default:
			generic = append(generic, t)
		}
	}
	if len(generic) > 0 {
		p.AllowElements(generic...)
	}
	if tagSet["a"] {
		p.AllowAttrs("href").OnElements("a")
		p.AllowAttrs("target").Matching(targetBlank).OnElements("a")
		p.AllowStandardURLs()
		p.RequireNoFollowOnLinks(true)
	}
	if tagSet["img"] {
		p.AllowImages()
		// Image size/float/alignment as inline "style" (as the editor's
		// resize handle writes it) - always allowed, unlike allowClass;
		// AllowImages already permits the equivalent HTML attributes.
		p.AllowAttrs("style").Matching(imgAppearanceStyle).OnElements("img")
	}
	if tagSet["iframe"] {
		allowIframes(p)
	}
	if tagSet["ul"] || tagSet["ol"] || tagSet["li"] {
		p.AllowLists()
	}
	if tagSet["table"] || tagSet["thead"] || tagSet["tbody"] || tagSet["tfoot"] || tagSet["tr"] || tagSet["td"] || tagSet["th"] {
		p.AllowTables()
	}
	if allowClass {
		p.AllowAttrs("class").Globally()
	}
	if tagSet["script"] || tagSet["style"] {
		// Without AllowUnsafe, bluemonday HTML-escapes the text inside
		// <script>/<style>, mangling the JS/CSS. Only formats where an
		// admin explicitly lists "script"/"style" in AllowedTags get this.
		p.AllowUnsafe(true)
		// "id" - embedded scripts/styles usually address their own markup
		// by id; gated with script/style to keep the same trust level.
		p.AllowAttrs("id").Globally()
	}
	return p
}
