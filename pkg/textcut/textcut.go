// Package textcut cuts text to a length in characters (runes): a letter
// is never split, whatever its script.
package textcut

import (
	"strings"
	"unicode/utf8"
)

// Runes - s cut to at most n characters.
func Runes(s string, n int) string {
	if n < 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// Ellipsis - s cut to n characters with "…" after them; s as it is when
// it is no longer (or n <= 0).
func Ellipsis(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// AtWord - s cut to at most n characters at the last space before them,
// the punctuation the cut leaves at the end dropped, with "…"; s as it is
// when it is no longer (or n <= 0).
func AtWord(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	cut := string([]rune(s)[:n])
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:\u2014-") + "…" // \u2014: an em dash
}
