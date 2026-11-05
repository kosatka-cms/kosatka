// Package fsname sanitizes arbitrary (including Cyrillic) strings into safe
// filesystem path segments and file name stems.
package fsname

import (
	"path/filepath"
	"strings"
	"unicode"

	"github.com/kosatka-cms/kosatka/pkg/translit"
)

// NormalizeFolderSegment transliterates Cyrillic to Latin, maps spaces to underscores,
// keeps [a-z0-9_-], collapses repeats. Result length is capped at 64 characters.
func NormalizeFolderSegment(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(raw) * 2)
	var prevUnderscore bool

	appendUnderscore := func() {
		if b.Len() == 0 || prevUnderscore {
			return
		}
		b.WriteByte('_')
		prevUnderscore = true
	}

	for _, r := range raw {
		if r == ' ' || r == '\t' {
			appendUnderscore()
			continue
		}
		if t, ok := translit.Latin(r); ok {
			prevUnderscore = false
			b.WriteString(t)
			continue
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			prevUnderscore = false
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			prevUnderscore = false
			b.WriteRune(unicode.ToLower(r))
		case r == '_' || r == '-':
			prevUnderscore = false
			b.WriteRune(r)
		default:
			appendUnderscore()
		}
	}

	s := b.String()
	s = collapseUnderscores(s)
	s = strings.Trim(s, "_-")
	if len(s) > 64 {
		s = s[:64]
		s = strings.TrimRight(s, "_-")
	}
	return s
}

func collapseUnderscores(s string) string {
	var b strings.Builder
	var prevUnderscore bool
	for _, r := range s {
		if r == '_' {
			if prevUnderscore {
				continue
			}
			prevUnderscore = true
			b.WriteRune(r)
		} else {
			prevUnderscore = false
			b.WriteRune(r)
		}
	}
	return b.String()
}

const maxImageStemLen = 200

// SanitizeImageStem turns the original file base name into a safe stem for storage:
// transliterates Cyrillic, keeps ASCII letters (case preserved) and digits, allows _ and -;
// spaces become underscores; other runes become underscores; collapses repeats.
// Empty or invalid result becomes "image". Length is capped at 200 runes.
func SanitizeImageStem(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "image"
	}
	raw = filepath.Base(raw)
	extPart := filepath.Ext(raw)
	if extPart != "" {
		raw = strings.TrimSuffix(raw, extPart)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "image"
	}

	var b strings.Builder
	b.Grow(len(raw) * 2)
	var prevUnderscore bool

	appendUnderscore := func() {
		if b.Len() == 0 || prevUnderscore {
			return
		}
		b.WriteByte('_')
		prevUnderscore = true
	}

	for _, r := range raw {
		if t, ok := translit.Latin(r); ok {
			prevUnderscore = false
			b.WriteString(t)
			continue
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			prevUnderscore = false
			b.WriteRune(r)
		case r == '_' || r == '-':
			prevUnderscore = false
			b.WriteRune(r)
		case r == ' ' || r == '\t':
			appendUnderscore()
		default:
			appendUnderscore()
		}
	}

	s := b.String()
	s = collapseUnderscores(s)
	s = strings.Trim(s, "_-")
	if s == "" {
		s = "image"
	}
	rs := []rune(s)
	if len(rs) > maxImageStemLen {
		rs = rs[:maxImageStemLen]
		s = string(rs)
		s = strings.TrimRight(s, "_-")
		if s == "" {
			s = "image"
		}
	}
	return s
}

// maxFileStemLen caps an upload's file name stem (runes).
const maxFileStemLen = 100

// FileStem turns an uploaded file's name into a URL-friendly stem:
// transliteration, lower case and whitespace → "-": "My Poster.JPG" → "my-poster". The extension
// is dropped; fallback is returned when nothing usable remains.
func FileStem(raw, fallback string) string {
	raw = filepath.Base(strings.TrimSpace(raw))
	raw = strings.TrimSuffix(raw, filepath.Ext(raw))
	var b strings.Builder
	prevDash := false
	dash := func() {
		if b.Len() > 0 && !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	for _, r := range raw {
		if t, ok := translit.Latin(r); ok {
			if t != "" {
				b.WriteString(t)
				prevDash = false
			}
			continue
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			prevDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(unicode.ToLower(r))
			prevDash = false
		default:
			dash()
		}
	}
	s := strings.Trim(b.String(), "-_")
	if rs := []rune(s); len(rs) > maxFileStemLen {
		s = strings.TrimRight(string(rs[:maxFileStemLen]), "-_")
	}
	if s == "" {
		return fallback
	}
	return s
}
