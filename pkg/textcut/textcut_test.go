package textcut

import "testing"

func TestCut(t *testing.T) {
	for _, c := range []struct {
		fn, in string
		n      int
		want   string
	}{
		{"Runes", "привет", 3, "при"},
		{"Runes", "hi", 5, "hi"},
		{"Ellipsis", "привет мир", 6, "привет…"},
		{"Ellipsis", "short", 10, "short"},
		{"Ellipsis", "anything", 0, "anything"},
		{"AtWord", "один два три", 9, "один два…"},
		{"AtWord", "один, два", 6, "один…"},
		{"AtWord", "словобезпробелов", 5, "слово…"},
		{"AtWord", "fits", 10, "fits"},
	} {
		got := map[string]func(string, int) string{"Runes": Runes, "Ellipsis": Ellipsis, "AtWord": AtWord}[c.fn](c.in, c.n)
		if got != c.want {
			t.Errorf("%s(%q, %d) = %q, want %q", c.fn, c.in, c.n, got, c.want)
		}
	}
}
