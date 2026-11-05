//go:build dbintegration

package app

import (
	"database/sql"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestLanguagePrefixes - an English site, the languages module enabled,
// Russian added on the Languages page at /ru/ (a prefix an address of the
// site has, or malformed, refused): the nodes from before are English
// (the data step); an article created at /ru/create/article is Russian,
// at /ru/<its path> in Russian, its address without the prefix redirects
// there, an English one under /ru/ redirects to its own; /ru/ is the
// front page in Russian, the front page lists its addresses in both
// (hreflang); the switcher is in the header; the sitemap lists the Russian one with its prefix. The
// module off - no prefixes; uninstalled - the languages forgotten.
func TestLanguagePrefixes(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, true)
		t.Cleanup(func() { entity.SetSiteLanguages("en", nil) })
		s.signIn()
		setModules := func(on bool) {
			t.Helper()
			form := url.Values{"csrf_token": {s.csrf("/admin/config/modules")}}
			for _, m := range entity.Modules {
				if m.Name != "languages" || on {
					form.Set("module_"+m.Name, "1")
				}
			}
			if resp := s.post("/admin/config/modules", form); resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("modules: %d", resp.StatusCode)
			}
		}
		setModules(true)
		addLanguage := func(code, prefix string) string {
			t.Helper()
			resp := s.post("/admin/config/languages", url.Values{"csrf_token": {s.csrf("/admin/config/languages")}, "code": {code}, "prefix": {prefix}})
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("adding %s: %d", code, resp.StatusCode)
			}
			return resp.Header.Get("Location")
		}
		for prefix, want := range map[string]string{"admin": "error=prefix-taken", "Ru!": "error=prefix"} {
			if got := addLanguage("ru", prefix); !strings.Contains(got, want) {
				t.Errorf("the prefix %q: %s, want %s", prefix, got, want)
			}
		}
		if got := addLanguage("ru", ""); !strings.Contains(got, "done=added") {
			t.Fatalf("adding Russian: %s\n%s", got, s.log.String())
		}

		var others int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM nodes WHERE langcode <> 'en'`).Scan(&others); err != nil || others != 0 {
			t.Fatalf("the nodes from before: %d not English (%v)", others, err)
		}
		_, form := s.get("/ru/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /ru/create/article")
		}
		if resp := s.postMultipart("/ru/create/article", url.Values{"csrf_token": {s.csrf("/ru/create/article")}, "title": {"Privet"}, body[1]: {"<p>Тест.</p>"}, "status": {"1"}}); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		var slug, lang string
		if err := db.DB.QueryRow(`SELECT slug, langcode FROM nodes WHERE title = 'Privet'`).Scan(&slug, &lang); err != nil || lang != "ru" {
			t.Fatalf("the new article: %q %v", lang, err)
		}
		guest := s.visitor()
		code, page := guest.get("/ru/" + slug)
		if code != http.StatusOK || !strings.Contains(page, `<html lang="ru">`) || !strings.Contains(page, "Тест.") {
			t.Errorf("/ru/%s: %d", slug, code)
		}
		if resp, err := guest.client.Get(guest.base + "/" + slug); err != nil || resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != "/ru/"+slug {
			t.Errorf("without the prefix: %v %v", resp, err)
		}
		var english string
		if err := db.DB.QueryRow(`SELECT slug FROM nodes WHERE langcode = 'en' AND type = 'article' ORDER BY id LIMIT 1`).Scan(&english); err != nil {
			t.Fatal(err)
		}
		if resp, err := guest.client.Get(guest.base + "/ru/" + english); err != nil || resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != "/"+english {
			t.Errorf("an English article under /ru/: %v %v", resp, err)
		}
		if code, page := guest.get("/ru/"); code != http.StatusOK || !strings.Contains(page, `<html lang="ru">`) || !strings.Contains(page, `hreflang="en" href="`+guest.base+`/"`) || !strings.Contains(page, `hreflang="ru" href="`+guest.base+`/ru/"`) {
			t.Errorf("/ru/: %d", code)
		}
		// The page's own canonical address: under its language's prefix,
		// as the article's.
		for path, want := range map[string]string{"/ru/": guest.base + "/ru/", "/ru/" + slug: guest.base + "/ru/" + slug} {
			if _, page := guest.get(path); !strings.Contains(page, `<link rel="canonical" href="`+want+`"`) {
				t.Errorf("%s: no canonical %s", path, want)
			}
		}
		// The switcher, placed in the header on enabling: on the Russian
		// article the English front page.
		if _, page := guest.get("/ru/" + slug); !strings.Contains(page, `class="language-switcher"`) || !strings.Contains(page, `<a class="nav-link" href="/" lang="en" hreflang="en"`) {
			t.Error("no switcher on the Russian article")
		}
		if _, page := s.get("/admin/config/languages"); !strings.Contains(page, "<code>/ru/</code>") {
			t.Error("the Languages page has no /ru/")
		}
		if _, sitemap := guest.get("/sitemap/nodes/1"); !strings.Contains(sitemap, "/ru/"+slug+"<") {
			t.Error("the sitemap has no /ru/" + slug)
		}

		setModules(false)
		if code, _ := guest.get("/ru/"); code != http.StatusNotFound {
			t.Errorf("the module off, /ru/: %d", code)
		}
		if code, _ := guest.get("/" + slug); code != http.StatusOK {
			t.Errorf("the module off, /%s: %d", slug, code)
		}
		if resp := s.post("/admin/config/modules/languages/uninstall", url.Values{"csrf_token": {s.csrf("/admin/config/modules/languages/uninstall")},
			"confirm": {"languages"}}); resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "uninstalled=languages") {
			t.Fatalf("uninstall: %d %s\n%s", resp.StatusCode, resp.Header.Get("Location"), s.log.String())
		}
		var saved sql.NullString
		if err := db.DB.QueryRow(`SELECT languages FROM site_information`).Scan(&saved); err != nil || (saved.Valid && saved.String != "null" && saved.String != "[]") {
			t.Errorf("uninstalled, the languages: %v %v", saved, err)
		}
		s.checkLog()
	})
}
