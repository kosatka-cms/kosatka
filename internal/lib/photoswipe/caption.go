// Package photoswipe builds markup for the PhotoSwipe gallery viewer.
package photoswipe

import (
	"html/template"
	"strings"
)

// CaptionHTML builds the escaped HTML for data-pswp-caption-html: a title and
// a description, or just the title when both are equal.
func CaptionHTML(title, description string) string {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title != "" && description != "" && title == description {
		description = ""
	}
	if title == "" && description == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="pswp-caption-block">`)
	if title != "" {
		b.WriteString(`<div class="pswp-caption-title">`)
		b.WriteString(template.HTMLEscapeString(title))
		b.WriteString(`</div>`)
	}
	if description != "" {
		b.WriteString(`<div class="pswp-caption-description">`)
		b.WriteString(template.HTMLEscapeString(description))
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
