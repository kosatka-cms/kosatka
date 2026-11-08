package demoset

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// TestSetsLoad - every set in every language loads and checks, the
// embedded ones and the tests' (testdata); a language the set hasn't and
// a set there isn't are errors.
func TestSetsLoad(t *testing.T) {
	loadAll := func() {
		t.Helper()
		for _, name := range Names() {
			for _, lang := range Languages(name) {
				if _, err := Load(name, lang); err != nil {
					t.Errorf("%s/%s: %v", name, lang, err)
				}
			}
		}
	}
	loadAll()
	defer UseSets(os.DirFS("testdata"))()
	if !slices.Equal(Names(), []string{"fixture"}) || !slices.Equal(Languages("fixture"), []string{"en"}) {
		t.Fatalf("the tests' sets: %v %v", Names(), Languages("fixture"))
	}
	loadAll()
	if _, err := Load("fixture", "xx"); err == nil || !strings.Contains(err.Error(), "has no") {
		t.Errorf("a language the set hasn't: %v", err)
	}
	if _, err := Load("missing", "en"); err == nil || !strings.Contains(err.Error(), "no demo set") {
		t.Errorf("a set there isn't: %v", err)
	}
}

// TestParseChecks - what the check refuses: unknown keys, repeated
// names, references to what the set hasn't.
func TestParseChecks(t *testing.T) {
	for name, c := range map[string]struct{ yaml, want string }{
		"unknown key":       {"site: {theme: x}\nwhatever: 1\n", "whatever"},
		"repeated user":     {"users: [a, a]\n", `user "a"`},
		"no vocabulary":     {"field_storages: [{name: f, type: taxonomy, vocabulary: nope}]\n", `no vocabulary "nope"`},
		"list no options":   {"field_storages: [{name: f, type: list}]\n", "without options"},
		"bad ago":           {"nodes: [{key: n, type: page, title: T, ago: yesterday}]\n", `ago "yesterday"`},
		"comment author":    {"nodes: [{key: n, type: page, title: T, comments: [{author: ghost, body: hi}]}]\n", `no user "ghost"`},
		"author":            {"nodes: [{key: n, type: page, title: T, author: ghost}]\n", `no user "ghost"`},
		"block two kinds":   {"blocks: [{region: sidebar, view: v, plugin: search_form}]\n", "one of view, plugin or body"},
		"block no region":   {"blocks: [{body: hi}]\n", "a region"},
		"menu node":         {"menus: [{name: m, links: [{title: T, node: nope}]}]\n", `no node "nope"`},
		"menu term":         {"vocabularies: [{name: v, label: V, terms: [a]}]\nmenus: [{name: m, links: [{title: T, term: v/b}]}]\n", `no term "v/b"`},
		"node term":         {"vocabularies: [{name: v, label: V, terms: [a]}]\nfield_storages: [{name: f, type: taxonomy, vocabulary: v}]\nnodes: [{key: n, type: t, title: T, fields: {f: [a, b]}}]\n", `node "n": f: no term "b" in vocabulary "v"`},
		"view field":        {"views: [{name: v, fields: [{builtin: title, storage: f}]}]\n", "a storage or a built-in"},
		"view builtin":      {"views: [{name: v, fields: [{builtin: nope}]}]\n", `no built-in field "nope"`},
		"filter builtin":    {"views: [{name: v, filters: [{builtin: nope}]}]\n", `no built-in filter "nope"`},
		"sort builtin":      {"views: [{name: v, sorts: [{builtin: view_count}]}]\n", `no built-in sort "view_count"`},
		"sort no label":     {"views: [{name: v, sorts: [{builtin: title, exposed: t}]}]\n", `sort offered as "t"`},
		"sort key twice":    {"views: [{name: v, sorts: [{builtin: title, exposed: t, exposed_label: A}, {builtin: views, exposed: t, exposed_label: B}]}]\n", `sort offered as "t"`},
		"view vocabulary":   {"views: [{name: v, term_vocabularies: [nope]}]\n", `no vocabulary "nope"`},
		"type image style":  {"content_types: [{name: t, comments: open, fields: [{storage: f, list_image_style: nope}]}]\n", `no image style "nope"`},
		"format the core's": {"text_formats: [{name: basic_html, label: B}]\n", "the core's"},
		"format name":       {"text_formats: [{name: Code-HTML, label: C}]\n", "a machine name"},
		"format no label":   {"text_formats: [{name: code_html}]\n", "no label"},
		"format script":     {"text_formats: [{name: code_html, label: C, allowed_tags: p SCRIPT}]\n", "no script or style"},
		"format twice":      {"text_formats: [{name: c, label: C}, {name: c, label: D}]\n", `text format "c"`},
		"type format":       {"content_types: [{name: t, comments: open, fields: [{storage: f, text_formats: [basic_html, nope]}]}]\n", `no text format "nope"`},
	} {
		_, err := Parse([]byte(c.yaml))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
}

// TestTermScalarOrMapping - a term is its name, or a mapping with a
// description.
func TestTermScalarOrMapping(t *testing.T) {
	s, err := Parse([]byte("vocabularies: [{name: v, label: V, terms: [a, {name: b, description: Bee}]}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	terms := s.Vocabularies[0].Terms
	if len(terms) != 2 || terms[0].Name != "a" || terms[1].Name != "b" || terms[1].Description != "Bee" {
		t.Errorf("terms: %+v", terms)
	}
}

// TestTypePreview - a type's form previews unless "preview: false".
func TestTypePreview(t *testing.T) {
	s, err := Parse([]byte("content_types: [{name: a, label: A, comments: open}, {name: b, label: B, comments: open, preview: false}, {name: c, label: C, comments: open, preview: true}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := []bool{s.ContentTypes[0].NoPreview(), s.ContentTypes[1].NoPreview(), s.ContentTypes[2].NoPreview()}; !slices.Equal(got, []bool{false, true, false}) {
		t.Errorf("NoPreview: %v", got)
	}
}

// TestParseAgo - days, hours and minutes before the install.
func TestParseAgo(t *testing.T) {
	for in, want := range map[string]string{"": "0s", "3d": "72h0m0s", "2d5h": "53h0m0s", "45m": "45m0s"} {
		d, err := parseAgo(in)
		if err != nil || d.String() != want {
			t.Errorf("%q: %v %v, want %s", in, d, err, want)
		}
	}
	if _, err := parseAgo("5 days"); err == nil {
		t.Error("5 days accepted")
	}
}

// TestDate - a date value: absolute, with a time, as YAML reads it (a
// time), relative to the install in days, years or hours.
func TestDate(t *testing.T) {
	a := &applier{Target: Target{Now: time.Date(2026, 10, 29, 14, 25, 0, 0, time.UTC)}}
	for raw, want := range map[any]entity.DateValue{
		"2020-09-17":       {Start: time.Date(2020, 9, 17, 0, 0, 0, 0, time.UTC)},
		"2026-11-07 20:00": {Start: time.Date(2026, 11, 7, 20, 0, 0, 0, time.UTC), WithTime: true},
		time.Date(1995, 3, 11, 0, 0, 0, 0, time.UTC): {Start: time.Date(1995, 3, 11, 0, 0, 0, 0, time.UTC)},
		"+12d": {Start: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)},
		"-2y":  {Start: time.Date(2024, 10, 29, 0, 0, 0, 0, time.UTC)},
		"+5h":  {Start: time.Date(2026, 10, 29, 19, 0, 0, 0, time.UTC), WithTime: true},
	} {
		got, err := a.date(raw)
		if err != nil || !got.Start.Equal(want.Start) || got.WithTime != want.WithTime {
			t.Errorf("%v: %+v %v, want %+v", raw, got, err, want)
		}
	}
	for _, bad := range []string{"tomorrow", "+3m", "2026-13-01"} {
		if _, err := a.date(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// TestSetPictures - an image field's {file: …} and a module's content's:
// a path inside the set's directory in a picture's format; Load wants
// the file there and an image; Pictures lists them.
func TestSetPictures(t *testing.T) {
	for _, bad := range []string{"../out.jpg", "/abs.png", "notes.txt", "images/x.gif"} {
		_, err := Parse([]byte("nodes: [{key: a, type: page, title: A, fields: {field_image: {file: " + bad + "}}}]\n"))
		if err == nil || !strings.Contains(err.Error(), "picture") {
			t.Errorf("%q: %v", bad, err)
		}
	}
	defer UseSets(os.DirFS("testdata"))()
	s, err := Load("fixture", "en")
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Pictures(); !slices.Equal(got, []string{"images/river.png"}) {
		t.Errorf("pictures (a node's and a gallery's, once): %v", got)
	}
	if _, err := Parse([]byte("modules: {galleries: {pictures: [{title: T, file: ../x.jpg}]}}\n")); err == nil || !strings.Contains(err.Error(), "modules: galleries: picture") {
		t.Errorf("a module's picture outside the set: %v", err)
	}
	missing := fstest.MapFS{"images/river.png": {Data: []byte("not a picture")}}
	if errs := s.CheckPictures(missing); len(errs) != 1 || !strings.Contains(errs[0].Error(), "not an image") {
		t.Errorf("a file that isn't an image: %v", errs)
	}
	if errs := s.CheckPictures(fstest.MapFS{}); len(errs) != 1 || !strings.Contains(errs[0].Error(), "not in the set's directory") {
		t.Errorf("a missing file: %v", errs)
	}
}

// TestThemeSet - a theme's own demo set (<theme>/demo/<lang>.yaml): listed
// by the theme's name, put in that theme unless it names one, marked as
// a theme's (its absent modules skipped on install - Absent lists them);
// an embedded set of the same name hides it; Regions - where its blocks
// and menus go.
func TestThemeSet(t *testing.T) {
	defer UseSets(os.DirFS("testdata"))()
	themesDir := t.TempDir()
	paths.SetThemesDir(themesDir)
	t.Cleanup(func() { paths.SetThemesDir("") })
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(themesDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sand/theme.yaml", "name: Sand\n")
	write("sand/demo/en.yaml", "blocks:\n  - {region: hero, body: <p>Hi</p>}\n  - {region: sidebar, body: <p>Side</p>}\nmenus:\n  - {name: m, label: M, region: hero}\n")
	write("sand/demo/de.yaml", "site: {theme: other}\n")
	write("sand/demo/es.yaml", "modules:\n  forms: {title: Hola}\n  forum: {topics: []}\n")
	write("plain/theme.yaml", "name: Plain\n")
	write("fixture/demo/en.yaml", "whatever: 1\n")

	if got := Names(); !slices.Equal(got, []string{"fixture", "sand"}) {
		t.Fatalf("names: %v", got)
	}
	if got := Languages("sand"); !slices.Equal(got, []string{"de", "en", "es"}) {
		t.Errorf("languages: %v", got)
	}
	en, err := Load("sand", "en")
	if err != nil {
		t.Fatal(err)
	}
	if en.Site.Theme != "sand" {
		t.Errorf("a theme's set's theme: %q, want sand", en.Site.Theme)
	}
	if got := en.Regions(); !slices.Equal(got, []string{"hero", "sidebar"}) {
		t.Errorf("regions: %v", got)
	}
	if de, err := Load("sand", "de"); err != nil || de.Site.Theme != "other" {
		t.Errorf("a set naming its theme: %+v %v", de, err)
	}
	if !en.FromTheme {
		t.Error("a theme's set isn't marked as one")
	}
	es, err := Load("sand", "es")
	if err != nil {
		t.Fatal(err)
	}
	if got := es.Absent(func(m string) bool { return m == "forms" }); !slices.Equal(got, []string{"forum"}) {
		t.Errorf("absent modules: %v, want [forum]", got)
	}
	fixture, err := Load("fixture", "en")
	if err != nil {
		t.Errorf("the embedded fixture, not the theme's broken one: %v", err)
	} else if fixture.FromTheme {
		t.Error("an engine's set marked as a theme's")
	}
	if _, err := Load("plain", "en"); err == nil {
		t.Error("a theme without a demo set loads")
	}
}
