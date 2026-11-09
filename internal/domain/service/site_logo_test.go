package service

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/paths"
)

// TestSiteLogoFor - the logo uploaded on Site information in every theme;
// without one, the theme's (theme.yaml "logo"); none in a theme without
// one and in the engine's templates.
func TestSiteLogoFor(t *testing.T) {
	zzChdirToProjectRoot(t)
	if got := SiteLogoFor("kosatka", ""); got != "/static/images/kosatka-logo.svg" {
		t.Errorf("kosatka's own: %q", got)
	}
	if got := SiteLogoFor("", ""); got != "" {
		t.Errorf("the engine's templates: %q, want none", got)
	}
	zzUseFixtureTheme(t)
	if got := SiteLogoFor(zzFixtureTheme, ""); got != "" {
		t.Errorf("a theme without a logo: %q, want none", got)
	}
	want := paths.PublicFilesURL() + "site/logo/mark.png"
	for _, theme := range []string{"", "kosatka", zzFixtureTheme} {
		if got := SiteLogoFor(theme, "public://site/logo/mark.png"); got != want {
			t.Errorf("%q with the site's logo: %q, want %q", theme, got, want)
		}
	}
}
