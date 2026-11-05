package viewquery

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestBuildGlossaryFromValues covers the pure grouping core of
// GlossaryForView, which works on a loaded title map and needs no DB.
func TestBuildGlossaryFromValues(t *testing.T) {
	t.Run("empty map yields empty glossary", func(t *testing.T) {
		got := buildGlossaryFromValues(map[int]string{})
		if len(got) != 0 {
			t.Fatalf("expected an empty GlossaryData, got %v", got)
		}
	})

	t.Run("mixed case groups under the uppercase letter", func(t *testing.T) {
		got := buildGlossaryFromValues(map[int]string{1: "banana", 2: "Beach"})
		want := entity.GlossaryData{{Letter: "B", Count: 2}}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("blank/whitespace-only titles are skipped", func(t *testing.T) {
		got := buildGlossaryFromValues(map[int]string{1: "", 2: "   ", 3: "House"})
		want := entity.GlossaryData{{Letter: "H", Count: 1}}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	// Cyrillic titles: the first letter is a multi-byte rune, not a byte.
	t.Run("unicode first rune is handled correctly", func(t *testing.T) {
		got := buildGlossaryFromValues(map[int]string{1: "Юрий", 2: "Ярость"})
		want := entity.GlossaryData{{Letter: "Ю", Count: 1}, {Letter: "Я", Count: 1}}
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("letters are sorted", func(t *testing.T) {
		got := buildGlossaryFromValues(map[int]string{1: "Z", 2: "A", 3: "M"})
		var letters []string
		for _, item := range got {
			letters = append(letters, item.Letter)
		}
		want := []string{"A", "M", "Z"}
		if len(letters) != len(want) {
			t.Fatalf("got %v, want %v", letters, want)
		}
		for i := range want {
			if letters[i] != want[i] {
				t.Fatalf("got %v, want %v", letters, want)
			}
		}
	})
}
