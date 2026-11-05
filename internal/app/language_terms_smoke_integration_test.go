//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestLanguageTerms - an English site with Russian: a term named and
// described in Russian shows so on its /ru/ page and as a tag of a
// Russian article, as is on its English page; the search on /ru/ finds
// the article's Russian translation and not its original, on / the
// original only.
func TestLanguageTerms(t *testing.T) {
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
		if resp := s.post("/admin/config/languages/settings", url.Values{"csrf_token": {s.csrf("/admin/config/languages")}, "translatable": {"article"}, "untranslated": {"hide"}}); !strings.Contains(resp.Header.Get("Location"), "done=saved") {
			t.Fatalf("settings: %s", resp.Header.Get("Location"))
		}

		var term, vid, nid int
		var name, termSlug, title string
		if err := db.DB.QueryRow(`SELECT t.id, t.vid, t.name, t.slug, n.id, n.title FROM taxonomy_term_data t
			JOIN taxonomy_vocabulary v ON v.id = t.vid JOIN taxonomy_index i ON i.tid = t.id JOIN nodes n ON n.id = i.nid
			WHERE v.public_term_pages = true AND n.type = 'article' AND n.status = 1 ORDER BY n.id LIMIT 1`).Scan(&term, &vid, &name, &termSlug, &nid, &title); err != nil {
			t.Fatal(err)
		}
		edit := fmt.Sprintf("/admin/config/taxonomy/%d/terms/%d/edit", vid, term)
		if _, page := s.get(edit); !strings.Contains(page, `name="term_name_ru"`) {
			t.Fatal("the term's form has no Russian name")
		}
		resp := s.post(fmt.Sprintf("/admin/config/taxonomy/%d/terms/%d", vid, term), url.Values{"csrf_token": {s.csrf(edit)}, "name": {name}, "slug": {termSlug},
			"term_name_ru": {"Раздел по-русски"}, "term_description_ru": {"<p>Описание по-русски.</p><script>alert(1)</script>"}})
		if !strings.Contains(resp.Header.Get("Location"), "success=1") {
			t.Fatalf("saving the term: %s\n%s", resp.Header.Get("Location"), s.log.String())
		}

		tab := fmt.Sprintf("/node/%d/translations", nid)
		resp = s.post(tab+"/ru", url.Values{"csrf_token": {s.csrf(tab)}})
		var tid int
		if _, err := fmt.Sscanf(resp.Header.Get("Location"), "/node/%d/edit", &tid); err != nil {
			t.Fatalf("translating: %s", resp.Header.Get("Location"))
		}
		var tslug string
		_ = db.DB.QueryRow(db.Dialect.Rebind(`SELECT slug FROM nodes WHERE id = ?`), tid).Scan(&tslug)
		if resp := s.post(fmt.Sprintf("/node/%d/publish", tid), url.Values{"csrf_token": {s.csrf(tab)}}); resp.StatusCode >= 400 {
			t.Fatalf("publishing: %d", resp.StatusCode)
		}

		guest := s.visitor()
		code, ru := guest.get("/ru/" + termSlug)
		if code != http.StatusOK || !strings.Contains(ru, "Раздел по-русски") || !strings.Contains(ru, "<p>Описание по-русски.</p>") || strings.Contains(ru, "<script>alert(1)") {
			t.Errorf("/ru/%s: %d, the Russian name and description", termSlug, code)
		}
		if _, en := guest.get("/" + termSlug); !strings.Contains(en, name) || strings.Contains(en, "Раздел по-русски") {
			t.Errorf("/%s: not the English name", termSlug)
		}
		if _, article := guest.get("/ru/" + tslug); !strings.Contains(article, ">Раздел по-русски</a>") {
			t.Error("the Russian article's tag isn't in Russian")
		}

		q := url.QueryEscape(title)
		if _, found := guest.get("/ru/search?q=" + q); !strings.Contains(found, "/ru/"+tslug) {
			t.Error("the search on /ru/ didn't find the translation")
		}
		var slug string
		_ = db.DB.QueryRow(db.Dialect.Rebind(`SELECT slug FROM nodes WHERE id = ?`), nid).Scan(&slug)
		if _, found := guest.get("/search?q=" + q); !strings.Contains(found, `"/`+slug+`"`) || strings.Contains(found, "/ru/"+tslug) {
			t.Error("the search on / didn't find the original only")
		}
		s.checkLog()
	})
}
