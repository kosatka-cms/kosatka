//go:build dbintegration

package app

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestLanguageSiteTexts - an English site with Russian: the site's name
// and slogan in Russian show on /ru/ (the header, the window's title), as
// is on /; a term's page lists its addresses in both languages
// (hreflang).
func TestLanguageSiteTexts(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, true)
		t.Cleanup(func() { entity.SetSiteLanguages("en", nil) })
		s.signIn()
		form := url.Values{"csrf_token": {s.csrf("/admin/config/modules")}}
		for _, m := range entity.Modules {
			form.Set("module_"+m.Name, "1")
		}
		if resp := s.post("/admin/config/modules", form); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("modules: %d", resp.StatusCode)
		}
		if resp := s.post("/admin/config/languages", url.Values{"csrf_token": {s.csrf("/admin/config/languages")}, "code": {"ru"}}); !strings.Contains(resp.Header.Get("Location"), "done=added") {
			t.Fatalf("adding Russian: %s", resp.Header.Get("Location"))
		}

		const info = "/admin/config/system/site-information"
		if _, page := s.get(info); !strings.Contains(page, `name="site_name_ru"`) {
			t.Fatal("Site information has no Russian name")
		}
		var name, slogan, email, feed, tz, lang string
		if err := db.DB.QueryRow(`SELECT site_name, slogan, site_email, front_feed_view, COALESCE(timezone, ''), COALESCE(default_language, '') FROM site_information`).Scan(&name, &slogan, &email, &feed, &tz, &lang); err != nil {
			t.Fatal(err)
		}
		if tz == "" {
			tz = "UTC"
		}
		resp := s.postMultipart(info, url.Values{"csrf_token": {s.csrf(info)}, "site_name": {name}, "slogan": {slogan}, "site_email": {email},
			"front_feed_view": {feed}, "timezone": {tz}, "default_language": {lang},
			"site_name_ru": {"Морская станция"}, "site_slogan_ru": {"Наблюдения за побережьем"}})
		if !strings.Contains(resp.Header.Get("Location"), "saved=1") {
			t.Fatalf("saving Site information: %s\n%s", resp.Header.Get("Location"), s.log.String())
		}

		guest := s.visitor()
		_, ru := guest.get("/ru/")
		if !strings.Contains(ru, "Морская станция") {
			t.Error("/ru/ has no Russian site name")
		}
		if _, en := guest.get("/"); strings.Contains(en, "Морская станция") || !strings.Contains(en, name) {
			t.Error("/ has not the English site name")
		}

		var termSlug string
		if err := db.DB.QueryRow(`SELECT t.slug FROM taxonomy_term_data t JOIN taxonomy_vocabulary v ON v.id = t.vid WHERE v.public_term_pages = true ORDER BY t.id LIMIT 1`).Scan(&termSlug); err != nil {
			t.Fatal(err)
		}
		_, term := guest.get("/ru/" + termSlug)
		for _, want := range []string{`hreflang="en" href="` + guest.base + `/` + termSlug + `"`, `hreflang="ru" href="` + guest.base + `/ru/` + termSlug + `"`, `hreflang="x-default"`} {
			if !strings.Contains(term, want) {
				t.Errorf("the term's page has no %s", want)
			}
		}
		if _, page := guest.get("/user/login"); strings.Contains(page, `<link rel="alternate" hreflang=`) {
			t.Error("the login page has hreflang")
		}
		s.checkLog()
	})
}
