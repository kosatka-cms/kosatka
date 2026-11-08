package service

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode"

	"github.com/kosatka-cms/kosatka/pkg/embedtag"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/textcut"
)

// FormatHumanFileSize formats a byte count as "245 KB", "3.1 MB" (binary
// 1024 units, one decimal place above KB, none below).
func FormatHumanFileSize(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

var (
	reHTMLTags = regexp.MustCompile(`(?s)<[^>]*>`)
	reWSPlain  = regexp.MustCompile(`\s+`)
	// reTag - an opening or closing tag: its slash and name (getPreview).
	reTag = regexp.MustCompile(`<(/?)(\w+)[^>]*>`)
)

// GetPlainPreview returns plain text without HTML (htmlsanitize.
// StripToText: tags, entities and embed tags gone, blocks apart) and
// without its code blocks (htmlsanitize.WithoutCode) - the part before "<!--break-->"
// when there is one - cut to limit characters with "…"; limit <= 0 - the
// whole text.
func GetPlainPreview(s string, limit int) string {
	if idx := strings.Index(s, "<!--break-->"); idx != -1 {
		s = s[:idx]
	}
	return textcut.Ellipsis(htmlsanitize.StripToText(htmlsanitize.WithoutCode(s)), limit)
}

// truncateText - plain text s (not HTML: tags are text too) cut to at
// most limit runes with "…"; a string, so templates escape it - unlike
// truncate, whose input must be sanitized HTML.
func truncateText(s string, limit int) string { return textcut.Ellipsis(s, limit) }

// PlainExcerpt - HTML s as plain text (htmlsanitize.StripToText: blocks
// apart, entities decoded) without its code blocks (htmlsanitize.WithoutCode), cut to
// at most limit runes at a word boundary with "…" (a View field's
// TrimLength, a teaser's text); limit <= 0 - the whole text.
func PlainExcerpt(s string, limit int) string {
	return textcut.AtWord(htmlsanitize.StripToText(htmlsanitize.WithoutCode(s)), limit)
}

// PlainLead - the first block of HTML s (its first paragraph, heading,
// list item…; code blocks left out, htmlsanitize.WithoutCode) as plain text, cut to at
// most limit runes at a word boundary with "…" (limit <= 0 - the whole
// block): a card's short text from a body that goes on with headings and
// lists. "" - no text.
func PlainLead(s string, limit int) string {
	lines := htmlsanitize.StripToLines(embedtag.Strip(htmlsanitize.WithoutCode(s)))
	if len(lines) == 0 {
		return ""
	}
	return textcut.AtWord(lines[0], limit)
}

// getPreview returns a preview of s (without its embed tags, embedtag): if
// s contains "<!--break-->", the part before it as-is; otherwise s
// truncated to limit non-space text runes with "…", keeping the HTML
// structure (unclosed tags are closed).
func getPreview(s string, limit int) string {
	s = embedtag.Strip(s)
	const breakMarker = "<!--break-->"
	if idx := strings.Index(s, breakMarker); idx != -1 {
		return strings.TrimSpace(s[:idx])
	}

	if limit <= 0 {
		return s
	}

	// Measure length on the text without tags.
	plainText := reHTMLTags.ReplaceAllString(s, "")
	plainText = html.UnescapeString(plainText)
	plainText = strings.TrimSpace(plainText)
	plainText = reWSPlain.ReplaceAllString(plainText, " ")

	runesPlain := []rune(plainText)

	// Short enough: return the original.
	if len(runesPlain) <= limit {
		return s
	}

	// Truncate while keeping the HTML structure.
	var result strings.Builder
	var textCount int
	inTag := false
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Track HTML tags.
		if r == '<' {
			inTag = true
		}

		result.WriteRune(r)

		if r == '>' {
			inTag = false
			continue
		}

		// Count only non-space text outside tags.
		if !inTag && !unicode.IsSpace(r) {
			textCount++
			if textCount >= limit {
				// Limit reached.
				result.WriteString("…")
				break
			}
		}
	}

	// Close any tags left open.
	return closeUnclosedTags(result.String())
}

// closeUnclosedTags appends closing tags for every unclosed HTML tag.
func closeUnclosedTags(s string) string {
	// Stack of open tags.
	var stack []string

	// Void elements.
	selfClosing := map[string]bool{
		"img": true, "br": true, "hr": true, "input": true,
		"meta": true, "link": true, "area": true, "base": true,
		"col": true, "embed": true, "param": true, "source": true,
		"track": true, "wbr": true,
	}

	// Matches opening and closing tags.
	matches := reTag.FindAllStringSubmatch(s, -1)

	for _, match := range matches {
		isClosing := match[1] == "/"
		tagName := strings.ToLower(match[2])

		if selfClosing[tagName] {
			continue
		}

		if isClosing {
			// Closing tag: pop from the stack.
			if len(stack) > 0 && stack[len(stack)-1] == tagName {
				stack = stack[:len(stack)-1]
			}
		} else {
			// Opening tag: push onto the stack.
			stack = append(stack, tagName)
		}
	}

	// Close remaining tags in reverse order.
	var closing strings.Builder
	for i := len(stack) - 1; i >= 0; i-- {
		closing.WriteString("</")
		closing.WriteString(stack[i])
		closing.WriteString(">")
	}

	return s + closing.String()
}
