// Package metadescription builds a plain-text meta description from HTML content.
package metadescription

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/pkg/embedtag"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
)

// The patterns Generate strips with, compiled once: scripts and styles
// with their content (two patterns: Go's regexp has no backreferences),
// tags, white space, numeric entities.
var (
	scriptRegex     = regexp.MustCompile(`(?i)<script[^>]*>[\s\S]*?</script>`)
	styleRegex      = regexp.MustCompile(`(?i)<style[^>]*>[\s\S]*?</style>`)
	htmlTagRegex    = regexp.MustCompile(`<[^>]+>`)
	whitespaceRegex = regexp.MustCompile(`\s+`)
	numericRegex    = regexp.MustCompile(`&#(\d+);`)
)

// Generate builds a plain-text meta description from HTML content - its
// prose, the code blocks left out (htmlsanitize.WithoutCode) - truncated
// at a word boundary to maxLength runes (160 when 0).
func Generate(content string, maxLength int) string {
	if maxLength == 0 {
		maxLength = 160 // recommended meta description length
	}
	content = PlainText(htmlsanitize.WithoutCode(content))

	if utf8.RuneCountInString(content) > maxLength {
		runes := []rune(content)

		truncated := string(runes[:maxLength])

		// Cut at the last space unless it is too far back.
		lastSpace := strings.LastIndex(truncated, " ")
		if lastSpace > maxLength-30 {
			truncated = truncated[:lastSpace]
		}

		content = truncated + "..."
	}

	return content
}

// PlainText - the text of HTML content: without embed tags, scripts,
// styles and tags, the entities decoded, white space collapsed.
func PlainText(content string) string {
	// The embed tags ([[block:…]]): what they embed isn't the text's.
	content = embedtag.Strip(content)

	// Drop script and style elements with their content.
	content = scriptRegex.ReplaceAllString(content, " ")
	content = styleRegex.ReplaceAllString(content, " ")

	content = htmlTagRegex.ReplaceAllString(content, " ")

	content = decodeHTMLEntities(content)

	// Collapse whitespace.
	content = whitespaceRegex.ReplaceAllString(content, " ")

	return strings.TrimSpace(content)
}

// decodeHTMLEntities decodes common named and numeric HTML entities.
func decodeHTMLEntities(s string) string {
	replacements := map[string]string{
		"&nbsp;":   " ",
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   "\"",
		"&#39;":    "'",
		"&apos;":   "'",
		"&ndash;":  "-",
		"&mdash;":  "-",
		"&hellip;": "...",
		"&rsquo;":  "'",
		"&lsquo;":  "'",
		"&rdquo;":  "\"", // plain double quote
		"&ldquo;":  "\"", // plain double quote
	}

	for entity, char := range replacements {
		s = strings.ReplaceAll(s, entity, char)
	}

	// Numeric entities (e.g. &#123;).
	s = numericRegex.ReplaceAllStringFunc(s, func(match string) string {
		var code int
		_, _ = fmt.Sscanf(match, "&#%d;", &code)
		return string(rune(code))
	})

	return s
}
