package translit

import "testing"

func TestLatin(t *testing.T) {
	for r, want := range map[rune]string{'Щ': "sch", 'щ': "sch", 'Ё': "yo", 'ь': "", 'Ж': "zh"} {
		if got, ok := Latin(r); !ok || got != want {
			t.Errorf("%c: %q %v, want %q", r, got, ok, want)
		}
	}
	if _, ok := Latin('q'); ok {
		t.Error("a Latin letter is in the table")
	}
}
