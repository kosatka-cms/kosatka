package entity

import (
	"strings"
	"testing"
)

// TestCommentSummary - the subject when there is one, else the start of
// the text as plain text, cut at a word.
func TestCommentSummary(t *testing.T) {
	subject := "A subject"
	blank := "  "
	long := "<p>" + strings.Repeat("word ", 20) + "</p>"
	for _, tc := range []struct {
		c    Comment
		want string
	}{
		{Comment{Subject: &subject, Content: "<p>text</p>"}, "A subject"},
		{Comment{Subject: &blank, Content: "<p>Short <b>text</b></p>"}, "Short text"},
		{Comment{Content: "<p>Short text</p>"}, "Short text"},
		{Comment{Content: long}, strings.TrimSpace(strings.Repeat("word ", 12)) + "…"},
	} {
		if got := tc.c.Summary(); got != tc.want {
			t.Errorf("Summary(%q) = %q, want %q", tc.c.Content, got, tc.want)
		}
	}
}
