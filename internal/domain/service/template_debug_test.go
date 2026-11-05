package service

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// Template debugging: BEGIN/END around a define of elements (with its
// file), none around one of text inside a script, the suggestions tried
// before a teaser; without it, no comments.
func TestTemplateDebug(t *testing.T) {
	root := t.TempDir()
	page := `{{define "sg_page"}}<main>{{template "sg_card" .}}<script>var u='{{template "sg_url"}}';</script>{{renderTeaser .item}}</main>{{end}}
{{define "sg_card"}}
{{/* a card */}}
<div class="card">card</div>{{end}}
{{define "sg_url"}}https://example.com/a.css{{end}}
{{define "teaser_default"}}<article>teaser</article>{{end}}`
	p := filepath.Join(root, "web", "templates", "sg.html")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	data := map[string]any{"item": entity.TeaserItem{Node: entity.Node{Type: "article"}}}

	render := func(debug bool) string {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		svc.SetDebug(debug)
		var b strings.Builder
		if err := svc.RenderNamed(&b, "sg_page", data, "sg.html"); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	out := render(true)
	for _, want := range []string{
		"<!-- BEGIN sg_card - " + filepath.Join("web", "templates", "sg.html") + " -->",
		"<!-- END sg_card -->",
		"<!-- BEGIN sg_page",
		"<!-- SUGGESTIONS: teaser_article, teaser_node, teaser_default ✓ -->",
		"var u='https://example.com/a.css';",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("debug: no %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "BEGIN sg_url") {
		t.Errorf("a define of text wrapped:\n%s", out)
	}
	if plain := render(false); strings.Contains(plain, "<!--") {
		t.Errorf("comments without debugging:\n%s", plain)
	}
}

// The page's comment goes after the doctype.
func TestDebugPageComment(t *testing.T) {
	got := string(debugPageComment([]byte("<!DOCTYPE html>\n<html></html>"), "index.html", themeScope{}))
	if !strings.HasPrefix(got, "<!DOCTYPE html>\n<!-- PAGE: index.html - theme: ") || !strings.Contains(got, "(engine)") {
		t.Errorf("page comment: %q", got)
	}
}
