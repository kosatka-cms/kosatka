package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/themes"
)

// catalogCard - the catalog page's entry (admin.catalogEntry's fields).
type catalogCard struct {
	themes.CatalogTheme
	Compatible bool
	Installed  string
	Update     bool
	Taken      bool
	Command    string
}

// TestAppearanceCatalogTemplate - the catalog page: a card per theme with
// the command installing or updating it, the warnings of a theme too new
// or whose directory is taken; no catalog configured - the notice.
func TestAppearanceCatalogTemplate(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	render := func(data map[string]any) string {
		t.Helper()
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, "content", data, "admin/config/appearance_catalog.html"); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	out := render(map[string]any{"entries": []catalogCard{
		{CatalogTheme: themes.CatalogTheme{Name: "sand", Title: "Sand", Version: "v1.2.0", Source: "https://github.com/example/kosatka-theme-sand", Stars: 12,
			Screenshot: "https://raw.githubusercontent.com/example/kosatka-theme-sand/v1.2.0/static/screenshot.jpg"},
			Compatible: true, Command: "app theme add https://github.com/example/kosatka-theme-sand"},
		{CatalogTheme: themes.CatalogTheme{Name: "night", Title: "Night", Version: "v2.0.0", Source: "https://github.com/example/night", MinVersion: "9.0.0"},
			Installed: "v1.0.0", Update: true, Command: "app theme update night"},
		{CatalogTheme: themes.CatalogTheme{Name: "kosatka", Title: "Other", Version: "v1.0.0", Source: "https://github.com/example/kosatka"},
			Compatible: true, Taken: true, Command: "app theme add https://github.com/example/kosatka -name <name>"},
	}})
	for _, want := range []string{
		"app theme add https://github.com/example/kosatka-theme-sand", "★ 12", `referrerpolicy="no-referrer"`,
		"app theme update night", "installed v1.0.0", "The theme needs Kosatka 9.0.0 or newer",
		"themes/kosatka is another theme's", "-name &lt;name&gt;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in the catalog page", want)
		}
	}
	if strings.Contains(out, "updated ") {
		t.Error("a theme without its last update shows an empty one")
	}
	if out := render(map[string]any{"notConfigured": true}); !strings.Contains(out, "No themes catalog is configured") {
		t.Errorf("no catalog: %s", out)
	}
}
