package searchtext

import (
	"fmt"
	"strings"
	"testing"
)

// TestWords - an index's words: folded, split at what isn't a letter or
// a digit, one-letter ones left out, stemmed in the language, each once,
// cut to MaxWordLen at a letter.
func TestWords(t *testing.T) {
	for _, c := range []struct{ text, lang, want string }{
		{"Синие коты, синего кота!", "ru", "[син кот]"},
		{"Ёлка и елки", "ru", "[елк]"},
		{"Running cats - a C++ guide", "en", "[run cat guid]"},
		{"Running cats", "", "[running cats]"},
		{"Running cats", "pt-BR", "[running cats]"},
		{"кот-воркотун 2026", "xx", "[кот воркотун 2026]"},
	} {
		if got := fmt.Sprint(Words(c.text, c.lang)); got != c.want {
			t.Errorf("Words(%q, %q) = %s, want %s", c.text, c.lang, got, c.want)
		}
	}
	long := strings.Repeat("я", 40)
	if w := Words(long, ""); len(w) != 1 || len(w[0]) != MaxWordLen {
		t.Errorf("a long word: %q", w)
	}
}

// TestParseQuery - a query's words with their forms in the site's
// languages, each once, at most MaxQueryWords.
func TestParseQuery(t *testing.T) {
	q := ParseQuery("  Синие   коты и cats ", []string{"ru", "en"})
	if q.Phrase != "синие коты и cats" {
		t.Errorf("phrase %q", q.Phrase)
	}
	if got := fmt.Sprint(q.Words); got != "[[синие син] [коты кот] [cats cat]]" {
		t.Errorf("words %s", got)
	}
	if q := ParseQuery("a1 b2 c3 d4 e5 f6 g7 h8 i9 a1", nil); len(q.Words) != MaxQueryWords {
		t.Errorf("%d words, want %d", len(q.Words), MaxQueryWords)
	}
	if q := ParseQuery("a b", nil); len(q.Words) != 0 {
		t.Errorf("one-letter words: %v", q.Words)
	}
}

// TestQueryMatches - every word in the indexed ones, by a form; the last
// one as a beginning too.
func TestQueryMatches(t *testing.T) {
	indexed := Words("Синий кот спит на котлетах", "ru")
	for query, want := range map[string]bool{
		"синего кота":  true,
		"кот синий":    true,
		"синий котле":  true,
		"котле синий":  false,
		"синий собака": false,
		"":             false,
	} {
		if got := ParseQuery(query, []string{"ru"}).Matches(indexed); got != want {
			t.Errorf("%q matches %v: %v, want %v", query, indexed, got, want)
		}
	}
}
