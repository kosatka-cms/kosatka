package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/themes"
)

// TestAppearanceTemplate - the Appearance page lists themes, marks the
// active one and offers to switch to the others.
func TestAppearanceTemplate(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	data := map[string]any{
		"themes": []themes.Option{{Dir: "", Name: "Engine", Regions: 5, Compatible: true}, {Dir: "site", Name: "Site", Regions: 9, Compatible: true, Source: "https://example.com/site-theme.git", Version: "1.4.0"},
			{Dir: "newer", Name: "Newer", Regions: 5, MinVersion: "9.0.0"}},
		"activeTheme": "site",
	}
	var buf strings.Builder
	if err := svc.RenderNamed(&buf, "content", data, "admin/config/appearance.html", "admin/alerts.html"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "themes/site") || !strings.Contains(out, `name="theme" value=""`) || strings.Contains(out, `name="theme" value="site"`) {
		t.Errorf("appearance page: %s", out)
	}
	// An installed theme: its source and release.
	if !strings.Contains(out, "https://example.com/site-theme.git") || !strings.Contains(out, "Version 1.4.0") {
		t.Errorf("an installed theme's card: %s", out)
	}
	// A theme the engine is too old for: a warning, no preview, no
	// activation.
	if strings.Contains(out, `name="theme" value="newer"`) || !strings.Contains(out, "The theme needs Kosatka 9.0.0 or newer") {
		t.Errorf("an incompatible theme's card: %s", out)
	}
}
