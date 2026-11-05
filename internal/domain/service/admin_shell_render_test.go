package service

import (
	"html/template"
	"strings"
	"testing"
)

// TestAdminShellIgnoresSiteTheme - admin pages use the admin shell's own
// header and footer: no theme stylesheets, no theme footer scripts, even
// with a site theme active; only the site's icon (the theme's favicon
// here).
func TestAdminShellIgnoresSiteTheme(t *testing.T) {
	zzChdirToProjectRoot(t)
	zzUseFixtureTheme(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("admin/modules/index.html", []string{"admin", "modules", "index.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("admin/modules/index.html", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	data := map[string]any{"isAdmin": true, "siteName": "Example", "pendingPeopleCount": 0, "pageSEO": map[string]any{"Title": "Modules"}}
	if err := set.ExecuteTemplate(&buf, "index.html", data); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, unwanted := range []string{"/themes/fixture/css/", "fixture-header"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("admin page contains %q", unwanted)
		}
	}
	for _, want := range []string{`class="admin-shell"`, `class="admin-brand"`, "Example", `id="admin-sidebar"`, `class="admin-topbar"`,
		`<link rel="icon" href="/static/themes/fixture/favicon.ico" type="image/x-icon">`} {
		if !strings.Contains(out, want) {
			t.Errorf("admin page lacks %q", want)
		}
	}
}
