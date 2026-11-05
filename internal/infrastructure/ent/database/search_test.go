package database

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/lib/searchtext"
)

// TestTitleRankOf - the phrase in the title, then all the words (by their
// stems in the title's language), then the rest.
func TestTitleRankOf(t *testing.T) {
	q := searchtext.ParseQuery("синий кот", []string{"ru"})
	for title, want := range map[string]int{
		"Синий кот и другие": 0,
		"Коты, синие как ёж": 1,
		"Кот":                2,
	} {
		if got := titleRankOf(title, "ru", q); got != want {
			t.Errorf("titleRankOf(%q) = %d, want %d", title, got, want)
		}
	}
}
