package starter

import (
	"strings"
	"testing"
)

// TestLoad - the starter set parses and its references hold.
func TestLoad(t *testing.T) {
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if s.Site.Theme == "" || len(s.ContentTypes) == 0 || len(s.Views) == 0 {
		t.Fatalf("an empty set: %+v", s)
	}
}

// TestParseRejects - an unknown key, a reference to something the set
// doesn't have and a repeated name are refused.
func TestParseRejects(t *testing.T) {
	for name, src := range map[string]string{
		"unknown key": "site: {theme: t}\ncolor: red\n",
		"no storage":  "site: {theme: t}\ncontent_types:\n  - {name: a, comments: open, fields: [{storage: field_x}]}\n",
		"no style":    "site: {theme: t}\nviews:\n  - {name: v, fields: [{builtin: title, image_style: card}]}\n",
		"no view":     "site: {theme: t, front_page_view: home}\n",
		"repeated":    "site: {theme: t}\nmenus: [{name: main}, {name: main}]\n",
		"no theme":    "menus: [{name: main}]\n",
		"bad mode":    "site: {theme: t}\ncontent_types: [{name: a, comments: maybe}]\n",
	} {
		if _, err := parse([]byte(src)); err == nil || !strings.Contains(err.Error(), "starter set") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
