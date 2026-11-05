package embedtag

import "testing"

func TestParse(t *testing.T) {
	for in, want := range map[string]Tag{
		"block:footer_links": {"block", "footer_links"},
		"[[view:popular]]":   {"view", "popular"},
		" gallery:42 ":       {"gallery", "42"},
	} {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %+v %v, want %+v", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "block", "block:", "Block:x", "block:x y", "[[block:x]] more", "block:<b>"} {
		if got, ok := Parse(in); ok {
			t.Errorf("Parse(%q) = %+v, want none", in, got)
		}
	}
}

func TestReplaceAndStrip(t *testing.T) {
	s := "<p>A [[block:one]] and [[note:two]], [[view:three]]</p>"
	got := Replace(s, func(tag Tag) (string, bool) {
		if tag.Kind == "note" {
			return "", false
		}
		return "<" + tag.Kind + "=" + tag.Name + ">", true
	})
	if want := "<p>A <block=one> and [[note:two]], <view=three></p>"; got != want {
		t.Errorf("Replace = %q, want %q", got, want)
	}
	if got := Strip(s); got != "<p>A  and , </p>" {
		t.Errorf("Strip = %q", got)
	}
	if (Tag{"block", "x"}).String() != "[[block:x]]" {
		t.Error("String")
	}
}
