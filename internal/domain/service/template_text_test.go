package service

import "testing"

// TestGetPlainPreview - a feed item's text: the part before
// <!--break-->, blocks apart, entities decoded, cut to characters with "…".
func TestGetPlainPreview(t *testing.T) {
	for _, c := range []struct {
		in    string
		limit int
		want  string
	}{
		{"<p>Первый абзац</p><p>второй &amp; третий</p>", 0, "Первый абзац второй & третий"},
		{"<p>Начало</p><!--break--><p>скрытое</p>", 0, "Начало"},
		{"<p>Первый абзац</p>", 6, "Первый…"},
	} {
		if got := GetPlainPreview(c.in, c.limit); got != c.want {
			t.Errorf("GetPlainPreview(%q, %d) = %q, want %q", c.in, c.limit, got, c.want)
		}
	}
}

// TestPlainLead - the first block of a text as plain text: a body's
// opening paragraph without the headings and lists after it, cut at a
// word; embeds and empty blocks left out; a plain text's first line.
func TestPlainLead(t *testing.T) {
	for _, tc := range []struct {
		in    string
		limit int
		want  string
	}{
		{"<p>Apps for iOS &amp; Android.</p><h2>What's included</h2><ul><li>Design</li></ul>", 0, "Apps for iOS & Android."},
		{"[[block:7]]<p></p><p>First words of a long opening paragraph</p>", 20, "First words of a…"},
		{"Plain text\nsecond line", 0, "Plain text"},
		{"<h2>A heading first</h2><p>Text</p>", 0, "A heading first"},
		{"", 10, ""},
	} {
		if got := PlainLead(tc.in, tc.limit); got != tc.want {
			t.Errorf("PlainLead(%q, %d) = %q, want %q", tc.in, tc.limit, got, tc.want)
		}
	}
}
