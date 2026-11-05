package imagestyle

import (
	"testing"

	"github.com/disintegration/imaging"
)

// TestSplitDouble - "@2x/<path>" is a 2x derivative's path; anything
// else a path.
func TestSplitDouble(t *testing.T) {
	if got := DoublePath("content/a.webp"); got != "@2x/content/a.webp" {
		t.Errorf("DoublePath = %q", got)
	}
	cases := map[string]bool{
		"@2x/content/a.webp": true,
		"content/a.webp":     false,
		"@3x/a.webp":         false,
		"@2x/":               false,
		"@2xa/b.webp":        false,
	}
	for path, want := range cases {
		double, rest := SplitDouble(path)
		if double != want || (!want && rest != path) || (want && rest != path[len("@2x/"):]) {
			t.Errorf("SplitDouble(%q) = %v, %q", path, double, rest)
		}
	}
}

// TestDouble - a HiDPI style at double size (a 0 height stays 0); a
// style without it - none.
func TestDouble(t *testing.T) {
	style, err := NewImageStyle("card", "Card", 720, 405, ResizeModeFill, imaging.Center, 80)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := style.Double(); ok {
		t.Error("a style without 2x")
	}
	style.SetHiDPI(true)
	if v, ok := style.Double(); !ok || v.Width() != 1440 || v.Height() != 810 || v.Name() != "card" {
		t.Errorf("Double = %v %v", v, ok)
	}
	free, _ := NewImageStyle("wide", "Wide", 800, 0, ResizeModeFit, imaging.Center, 80)
	free.SetHiDPI(true)
	if v, ok := free.Double(); !ok || v.Width() != 1600 || v.Height() != 0 {
		t.Errorf("a style keeping proportions: %v %v", v, ok)
	}
}
