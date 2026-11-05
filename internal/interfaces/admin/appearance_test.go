package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

// TestAppearanceSwitchTheme - a listed theme is saved and applied; an
// unknown one is rejected without touching anything.
func TestAppearanceSwitchTheme(t *testing.T) {
	for _, c := range []struct {
		theme    string
		redirect string
		applied  bool
	}{
		{"kosatka", "saved=1", true},
		{"", "saved=1", true},
		{"no-such-theme", "error=1", false},
		{"../../etc", "error=1", false},
	} {
		site := &stubSiteInformation{site: entity.SiteInformation{}}
		var applied *string
		ac := testController(interfaces.BaseDeps{Logger: testLogger()}, func(c *Controller) { c.siteInformationRepo = site })
		ac.switchTheme = func(dir string) { applied = &dir }
		req := httptest.NewRequest(http.MethodPost, "/admin/config/appearance", strings.NewReader(url.Values{"theme": {c.theme}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		ac.AppearancePage(rec, req)
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.redirect) {
			t.Errorf("%q: redirect %q", c.theme, loc)
		}
		if (applied != nil) != c.applied || (c.applied && (*applied != c.theme || site.site.Theme == nil || *site.site.Theme != c.theme)) {
			t.Errorf("%q: applied %v, stored %v", c.theme, applied, site.site.Theme)
		}
	}
}

// TestBlocksTheme - the blocks admin works on an available theme from the
// "theme" parameter, else on the active one.
func TestBlocksTheme(t *testing.T) {
	for param, want := range map[string]string{
		"kosatka":      "kosatka",
		"default":      "default",
		"no-such":      "default",
		"../templates": "default",
		"":             "default",
	} {
		r := httptest.NewRequest(http.MethodGet, "/admin/config/blocks?theme="+url.QueryEscape(param), nil)
		if got := blocksTheme(r); got != want {
			t.Errorf("theme=%q: got %q, want %q", param, got, want)
		}
	}
	if got := blocksURL("site", "success=1"); got != "/admin/config/blocks?theme=site&success=1" {
		t.Errorf("blocksURL: %s", got)
	}
}
