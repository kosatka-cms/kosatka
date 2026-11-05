package searchtext

import (
	"fmt"
	"strings"
	"testing"
)

// show - a snippet as text, the matches in [ ], the cuts as "...".
func show(s Snippet) string {
	var b strings.Builder
	if s.Before {
		b.WriteString("...")
	}
	for _, p := range s.Parts {
		if p.Match {
			fmt.Fprintf(&b, "[%s]", p.Text)
		} else {
			b.WriteString(p.Text)
		}
	}
	if s.After {
		b.WriteString("...")
	}
	return b.String()
}

// TestSnippet - the words of the query marked by their stems, the last
// one as a beginning too; a long text cut between words around the first
// match; none - false.
func TestSnippet(t *testing.T) {
	q := ParseQuery("синий котле", []string{"ru"})
	for _, c := range []struct {
		text string
		want string
	}{
		{"Синие котлеты на обед.", "[Синие] [котлеты] на обед."},
		{"Сначала про обед, потом синего цвета котлеты.", "Сначала про обед, потом [синего] цвета [котлеты]."},
		{strings.Repeat("слово ", 20) + "синий кот " + strings.Repeat("дальше ", 40),
			"..." + strings.TrimSpace(strings.Repeat("слово ", 10)) + " [синий] кот " + strings.TrimSpace(strings.Repeat("дальше ", 13)) + "..."},
		{`"Синий" - кот`, `"[Синий]" - кот`},
	} {
		s, ok := q.Snippet(c.text, "ru", 160)
		if got := show(s); !ok || got != c.want {
			t.Errorf("Snippet(%q) = %q %v,\nwant %q", c.text, got, ok, c.want)
		}
	}
	if _, ok := q.Snippet("Ничего похожего", "ru", 160); ok {
		t.Error("a text without the words has a snippet")
	}
}
