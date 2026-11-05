package i18nextract

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestExtractJS checks Kosatka.t/tc/formatPlural extraction from static JS
// (skipping minified and bundled libraries) and templates' inline scripts,
// on disk and in a module's embedded files.
func TestExtractJS(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("web/static/js/app.js", `alert(Kosatka.t('Saved.')); x = Kosatka.tc("verb", 'Search');
// Kosatka.t('Commented out')
y = Kosatka.formatPlural(n, '@count file', '@count files'); z = Kosatka.t('It\'s "ok"');`)
	write("web/static/js/app.min.js", `Kosatka.t("Minified")`)
	write("web/static/js/tinymce/x.js", `Kosatka.t("Library")`)
	write("web/templates/page.html", `<script>confirm(Kosatka.t('Delete?'))</script>{{t "Server side"}}`)
	// A module's embedded files (a site has no module sources on disk).
	module := fstest.MapFS{
		"static/vote.js":           {Data: []byte(`Kosatka.t('Vote sent')`)},
		"static/vote.min.js":       {Data: []byte(`Kosatka.t('Minified vote')`)},
		"templates/poll/form.html": {Data: []byte(`<script>Kosatka.t('Pick one')</script>`)},
	}
	keys, err := ExtractJS(root, map[string]fs.FS{"polls": module})
	if err != nil {
		t.Fatal(err)
	}
	want := []i18n.JSKey{{Source: "Delete?"}, {Source: "Saved."}, {Context: "verb", Source: "Search"}, {Source: "@count file"}, {Source: `It's "ok"`}, {Source: "Vote sent"}, {Source: "Pick one"}}
	for _, w := range want {
		if !slices.Contains(keys, w) {
			t.Errorf("missing %+v in %+v", w, keys)
		}
	}
	if len(keys) != len(want) {
		t.Errorf("keys = %+v", keys)
	}
}

// TestExtractManifestLabels checks that the names and region labels of the
// themes' manifests are extracted, tagged with their theme.
func TestExtractManifestLabels(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("web/templates/theme.yaml", "name: Engine\nregions:\n  # label: commented\n  - id: header\n    label: Header\n")
	write("themes/demo/theme.yaml", "regions:\n  - id: side\n    label: \"Side column\"\n")
	msgs, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range msgs {
		got = append(got, m.Theme+":"+m.Source)
	}
	// By file: themes/ before web/.
	if want := []string{"demo:Side column", ":Engine", ":Header"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExtractStarterTexts - the starter set's labels, descriptions and
// titles are extracted; its names and comments aren't.
func TestExtractStarterTexts(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, filepath.FromSlash(starterSet))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "# label: commented\ncontent_types:\n  - name: article\n    label: Article\n    description: \"News: posts\"\nmenus:\n  - links:\n      - title: Home\n"
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	msgs, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range msgs {
		got = append(got, m.Source)
	}
	if want := []string{"Article", "News: posts", "Home"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSources - the source strings' list: a string once, the core's over
// a module's, a theme's once per theme; a text only the starter set has -
// marked Starter, one the code has too - the code's.
func TestSources(t *testing.T) {
	starter := "internal/app/starter/starter.yaml"
	got := Sources([]Message{
		{File: "web/templates/a.html", Source: "Save"},
		{File: "internal/modules/polls/x.go", Module: "polls", Source: "Save"},
		{File: "internal/modules/polls/x.go", Module: "polls", Source: "Vote", Plural: ""},
		{File: "themes/alpha/templates/h.html", Theme: "alpha", Source: "READY."},
		{File: "themes/beta/templates/h.html", Theme: "beta", Source: "READY."},
		{File: starter, Source: "Front page"},
		{File: starter, Source: "Save"},
	})
	want := []i18n.Source{
		{Source: "Front page", Starter: true},
		{Source: "READY.", Theme: "alpha"},
		{Source: "READY.", Theme: "beta"},
		{Source: "Save"},
		{Source: "Vote", Module: "polls"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}
