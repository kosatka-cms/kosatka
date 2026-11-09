package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// TestSiteIconFor - the uploaded icon in every theme (a PNG also the home
// screen's); without one, the theme's favicon, none for the engine's
// templates.
func TestSiteIconFor(t *testing.T) {
	zzChdirToProjectRoot(t)
	t.Cleanup(func() { SetSiteIcon("") })

	SetSiteIcon("")
	if got := SiteIconFor("kosatka"); got.URL != "/static/images/kosatka-icon.svg" || got.Type != "image/svg+xml" || got.Touch {
		t.Errorf("kosatka's own: %+v", got)
	}
	zzUseFixtureTheme(t)
	if got := SiteIconFor(zzFixtureTheme); got.Type != "image/x-icon" {
		t.Errorf("an .ico: %+v", got)
	}
	if got := SiteIconFor(""); got != (SiteIcon{}) {
		t.Errorf("the engine's templates: %+v, want none", got)
	}

	SetSiteIcon("public://site/icon/mark.png")
	want := SiteIcon{URL: paths.PublicFilesURL() + "site/icon/mark.png", Type: "image/png", Touch: true}
	for _, theme := range []string{"", "kosatka", zzFixtureTheme} {
		if got := SiteIconFor(theme); got != want {
			t.Errorf("%q with the site's icon: %+v, want %+v", theme, got, want)
		}
	}
}

// TestThemeFaviconChecked - a manifest's favicon and logo are paths on
// the site; another host's ("//…", "https://…") makes the manifest
// invalid.
func TestThemeFaviconChecked(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "web", "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	for favicon, ok := range map[string]bool{
		"/static/x/favicon.ico":      true,
		"//evil.example/favicon.ico": false,
		"https://example.com/x.png":  false,
		"static/relative.png":        false,
		"/static/a\" onload=\"x.png": false,
	} {
		for _, key := range []string{"favicon", "logo"} {
			body := "regions:\n  - id: top\n    label: Top\n" + key + ": '" + strings.ReplaceAll(favicon, "'", "''") + "'\n"
			if err := os.WriteFile(filepath.Join(dir, "web", "templates", themes.InfoFile), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := themes.LoadInfoFor(""); (err == nil) != ok {
				t.Errorf("%s %q: %v, want valid %v", key, favicon, err, ok)
			}
		}
	}
}
