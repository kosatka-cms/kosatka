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
