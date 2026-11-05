package entity

import (
	"testing"
)

func TestViewDisplayAttachmentHelpers(t *testing.T) {
	d := ViewDisplayDef{DisplayType: ViewDisplayTypeAttachment, AttachContentTypes: " article, news ,"}
	for typ, want := range map[string]bool{"article": true, "news": true, "video": false, "": false} {
		if got := d.MatchesContentType(typ); got != want {
			t.Fatalf("MatchesContentType(%q) = %v, want %v", typ, got, want)
		}
	}
	for typ, want := range map[ViewDisplayType]bool{ViewDisplayTypePage: true, ViewDisplayTypeBlock: false, ViewDisplayTypeAttachment: false, ViewDisplayTypeTerm: false} {
		if got := typ.HasURL(); got != want {
			t.Fatalf("%s.HasURL() = %v, want %v", typ, got, want)
		}
	}
}

func TestViewDisplayTermVocabularies(t *testing.T) {
	d := ViewDisplayDef{DisplayType: ViewDisplayTypeTerm, TermVocabularies: " 3, x,11,"}
	for vid, want := range map[int]bool{3: true, 11: true, 4: false} {
		if got := d.MatchesVocabulary(vid); got != want {
			t.Fatalf("MatchesVocabulary(%d) = %v, want %v", vid, got, want)
		}
	}
	if !(ViewDisplayDef{}).MatchesVocabulary(42) {
		t.Fatal("empty TermVocabularies must match every vocabulary")
	}
}

func TestMatchURLArgument(t *testing.T) {
	for _, tc := range []struct {
		pattern, path, arg string
		ok                 bool
	}{
		{"posts/%", "posts/ishmael", "ishmael", true},
		{"posts/%", "posts", "", false},
		{"posts/%", "posts/ishmael/x", "", false},
		{"posts/%", "news/ishmael", "", false},
		{"%/post", "ishmael/post", "ishmael", true},
		{"post", "post", "", false}, // no argument segment
	} {
		arg, ok := MatchURLArgument(tc.pattern, tc.path)
		if arg != tc.arg || ok != tc.ok {
			t.Errorf("%q vs %q: %q %v, want %q %v", tc.pattern, tc.path, arg, ok, tc.arg, tc.ok)
		}
	}
}
