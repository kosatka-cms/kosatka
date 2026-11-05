//go:build dbintegration

package app

import (
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestThemesRender - the Appearance page shows every theme of the engine
// (themes/) as a card with its description; each, previewed
// by the administrator, renders the demo site's front page and
// an article with its own styles and no template error; an inactive
// theme's settings (kosatka's, the base theme active) changed on its page
// show in its preview, reset - its defaults.
func TestThemesRender(t *testing.T) {
	entries, err := os.ReadDir("../../themes")
	if err != nil {
		t.Fatal(err)
	}
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, true)
		s.signIn()
		_, home := s.get("/")
		article := regexp.MustCompile(`href="(/article/[^"]+)"`).FindStringSubmatch(home)
		if article == nil {
			t.Fatal("no article on the front page")
		}
		// The Appearance page: a card per theme, with its screenshot and
		// description (theme.yaml).
		if code, page := s.get("/admin/config/appearance"); code != http.StatusOK ||
			strings.Count(page, `class="card h-100 theme-card`) < len(entries) ||
			!strings.Contains(page, "/static/themes/kosatka/screenshot.jpg") || !strings.Contains(page, "lists in rows of glass") {
			t.Errorf("GET the Appearance page: %d, the cards %d, kosatka's screenshot %v, its description %v", code,
				strings.Count(page, `class="card h-100 theme-card`), strings.Contains(page, "/static/themes/kosatka/screenshot.jpg"), strings.Contains(page, "lists in rows of glass"))
		}
		token := s.csrf("/admin/config/appearance")
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			theme := e.Name()
			// The active theme has no preview: the previous one ends first.
			s.post("/admin/config/appearance/preview/stop", url.Values{"csrf_token": {token}})
			if resp := s.post("/admin/config/appearance/preview", url.Values{"theme": {theme}, "csrf_token": {token}}); resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("%s: starting the preview: %d", theme, resp.StatusCode)
			}
			for _, path := range []string{"/", article[1]} {
				code, page := s.get(path)
				if code != http.StatusOK || !strings.Contains(page, "/static/themes/"+theme+"/") || !strings.Contains(page, "</html>") {
					t.Errorf("%s: GET %s: %d, its styles %v, the whole page %v", theme, path, code,
						strings.Contains(page, "/static/themes/"+theme+"/"), strings.Contains(page, "</html>"))
				}
			}
		}

		// An inactive theme's settings in its preview: the base theme
		// active, kosatka previewed with its accent and scheme changed on
		// its page; reset - its defaults again.
		s.post("/admin/config/appearance/preview/stop", url.Values{"csrf_token": {token}})
		if resp := s.post("/admin/config/appearance", url.Values{"theme": {""}, "csrf_token": {token}}); !strings.Contains(resp.Header.Get("Location"), "saved=1") {
			t.Fatalf("choosing the base theme: %s", resp.Header.Get("Location"))
		}
		s.post("/admin/config/appearance/preview", url.Values{"theme": {"kosatka"}, "csrf_token": {token}})
		settings := "/admin/config/appearance/theme/kosatka"
		if resp := s.post(settings, url.Values{"csrf_token": {s.csrf(settings)}, "action": {"save"}, "setting_accent": {"#aa0000"},
			"setting_accent_dark": {"#22d3ee"}, "setting_scheme": {"light"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("kosatka: saving its settings: %d", resp.StatusCode)
		}
		if _, page := s.get("/"); !strings.Contains(page, "admin-top-bar-preview") || !strings.Contains(page, "--kosatka-accent: #aa0000;") || !strings.Contains(page, `f="light"`) {
			t.Errorf("kosatka's preview after its settings: the preview %v, the accent %v, the scheme %v", strings.Contains(page, "admin-top-bar-preview"),
				strings.Contains(page, "--kosatka-accent: #aa0000;"), strings.Contains(page, `f="light"`))
		}
		s.post(settings, url.Values{"csrf_token": {s.csrf(settings)}, "action": {"reset"}})
		if _, page := s.get("/"); !strings.Contains(page, "--kosatka-accent: #0e7490;") || !strings.Contains(page, `f="dark"`) {
			t.Error("kosatka's preview after the reset: not its defaults")
		}
		s.post("/admin/config/appearance/preview/stop", url.Values{"csrf_token": {token}})
		s.checkLog()
	})
}
