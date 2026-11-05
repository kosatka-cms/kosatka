//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestNodeTranslations - an English site with Russian, articles
// translated: an article's Translations tab offers Russian; translating
// makes an unpublished Russian copy with the title kept, in the
// article's set, its form naming the original; published, the English
// article's switcher and hreflang lead to it; saving the original marks
// it outdated, saving it clears the mark; a second Russian version is
// refused; the front page in each language lists the content by the "No
// translation" setting; the form sets an item's language, but not one
// its set has; a page type that isn't translated has no tab.
func TestNodeTranslations(t *testing.T) {
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
		if resp := s.post("/admin/config/languages/settings", url.Values{"csrf_token": {s.csrf("/admin/config/languages")}, "translatable": {"article"}}); !strings.Contains(resp.Header.Get("Location"), "done=saved") {
			t.Fatalf("translatable types: %s", resp.Header.Get("Location"))
		}

		var nid int
		var slug, title string
		if err := db.DB.QueryRow(`SELECT id, slug, title FROM nodes WHERE type = 'article' AND status = 1 AND promote = true ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&nid, &slug, &title); err != nil {
			t.Fatal(err)
		}
		tab := fmt.Sprintf("/node/%d/translations", nid)
		if _, page := s.get("/" + slug); !strings.Contains(page, `href="`+tab+`"`) {
			t.Error("the article has no Translations tab")
		}
		if code, page := s.get(tab); code != http.StatusOK || !strings.Contains(page, `action="`+tab+`/ru"`) {
			t.Fatalf("%s: %d", tab, code)
		}
		resp := s.post(tab+"/ru", url.Values{"csrf_token": {s.csrf(tab)}})
		var tid int
		if _, err := fmt.Sscanf(resp.Header.Get("Location"), "/node/%d/edit", &tid); err != nil || resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("translating: %d %s\n%s", resp.StatusCode, resp.Header.Get("Location"), s.log.String())
		}
		var lang, ttitle, tslug string
		var status int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT langcode, title, slug, status FROM nodes WHERE id = ?`), tid).Scan(&lang, &ttitle, &tslug, &status); err != nil || lang != "ru" || ttitle != title || status != 0 {
			t.Fatalf("the translation: %q %q %d %v", lang, ttitle, status, err)
		}
		var tnid int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT tnid FROM node_translations WHERE nid = ?`), tid).Scan(&tnid); err != nil || tnid != nid {
			t.Fatalf("the translation's set: %d %v", tnid, err)
		}
		edit := fmt.Sprintf("/node/%d/edit", tid)
		_, editPage := s.get(edit)
		if !strings.Contains(editPage, "Translation into Русский") {
			t.Error("the translation's form doesn't name it")
		}
		if resp := s.post(fmt.Sprintf("/node/%d/publish", tid), url.Values{"csrf_token": {s.csrf("/" + slug)}}); resp.StatusCode >= 400 {
			t.Fatalf("publishing the translation: %d", resp.StatusCode)
		}
		guest := s.visitor()
		_, page := guest.get("/" + slug)
		if !strings.Contains(page, `<a class="nav-link" href="/ru/`+tslug+`" lang="ru"`) {
			t.Error("the switcher doesn't lead to the translation")
		}
		if !strings.Contains(page, `hreflang="ru" href="`+guest.base+`/ru/`+tslug+`"`) || !strings.Contains(page, `hreflang="x-default" href="`+guest.base+`/`+slug+`"`) {
			t.Error("no alternates of the translated article")
		}

		// The forms: the original saved - the translation outdated; the
		// translation saved - up to date.
		save := func(id int, extra ...string) {
			t.Helper()
			path := fmt.Sprintf("/node/%d/edit", id)
			_, page := s.get(path)
			fields := url.Values{"csrf_token": {s.csrf(path)}, "status": {"1"}}
			for i := 0; i+1 < len(extra); i += 2 {
				fields.Set(extra[i], extra[i+1])
			}
			fields.Set("title", regexp.MustCompile(`name="title"[^>]*value="([^"]*)"`).FindStringSubmatch(page)[1])
			for _, m := range regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindAllStringSubmatch(page, -1) {
				fields.Set(m[1], "<p>Text.</p>")
			}
			if resp := s.postMultipart(path, fields); resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("saving %d: %d\n%s", id, resp.StatusCode, s.log.String())
			}
		}
		outdated := func() bool {
			t.Helper()
			var o bool
			if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT outdated FROM node_translations WHERE nid = ?`), tid).Scan(&o); err != nil {
				t.Fatal(err)
			}
			return o
		}
		save(nid)
		if !outdated() {
			t.Error("the original saved, the translation isn't outdated")
		}
		save(tid)
		if outdated() {
			t.Error("the translation saved, it is still outdated")
		}

		if resp := s.post(tab+"/ru", url.Values{"csrf_token": {s.csrf(tab)}}); !strings.Contains(resp.Header.Get("Location"), "error=exists") {
			t.Errorf("a second Russian version: %s", resp.Header.Get("Location"))
		}

		// The lists ("No translation" - the original, marked): the Russian
		// front page has the translation, not its original, and the
		// untranslated English articles marked; the English one, the
		// original, not the translation.
		_, ruFront := guest.get("/ru/")
		if !strings.Contains(ruFront, `href="/ru/`+tslug+`"`) || strings.Contains(ruFront, `href="/`+slug+`"`) || !strings.Contains(ruFront, `node-teaser-language">English<`) {
			t.Errorf("the Russian front page: translation %v, original %v, mark %v", strings.Contains(ruFront, `href="/ru/`+tslug+`"`), strings.Contains(ruFront, `href="/`+slug+`"`), strings.Contains(ruFront, `node-teaser-language">English<`))
		}
		if _, enFront := guest.get("/"); !strings.Contains(enFront, `href="/`+slug+`"`) || strings.Contains(enFront, `href="/ru/`+tslug+`"`) || strings.Contains(enFront, "node-teaser-language") {
			t.Error("the English front page: not the original, or the translation, or a mark")
		}
		// Only the language's content.
		if resp := s.post("/admin/config/languages/settings", url.Values{"csrf_token": {s.csrf("/admin/config/languages")}, "translatable": {"article"}, "untranslated": {"hide"}}); !strings.Contains(resp.Header.Get("Location"), "done=saved") {
			t.Fatalf("settings: %s", resp.Header.Get("Location"))
		}
		// The front page's list is a cached block: rendered again
		// after the save (CacheService.ForceUpdate) - waited for.
		var ruOnly string
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if _, ruOnly = guest.get("/ru/"); !strings.Contains(ruOnly, "node-teaser-language") {
				break
			}
		}
		if !strings.Contains(ruOnly, `href="/ru/`+tslug+`"`) || strings.Contains(ruOnly, "node-teaser-language") {
			t.Error("the Russian front page, only Russian: an English article")
		}

		// The form's Language: another article goes Russian; the
		// translation can't be English (its original is).
		var other int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id FROM nodes WHERE type = 'article' AND id <> ? AND id <> ? ORDER BY id LIMIT 1`), nid, tid).Scan(&other); err != nil {
			t.Fatal(err)
		}
		save(other, "langcode", "ru")
		save(tid, "langcode", "en")
		var otherLang, tLang string
		_ = db.DB.QueryRow(db.Dialect.Rebind(`SELECT langcode FROM nodes WHERE id = ?`), other).Scan(&otherLang)
		_ = db.DB.QueryRow(db.Dialect.Rebind(`SELECT langcode FROM nodes WHERE id = ?`), tid).Scan(&tLang)
		if otherLang != "ru" || tLang != "ru" {
			t.Errorf("the form's language: %q (want ru), the translation %q (want ru)", otherLang, tLang)
		}
		var page2ID int
		var page2 string
		if err := db.DB.QueryRow(`SELECT id, slug FROM nodes WHERE type = 'page' ORDER BY id LIMIT 1`).Scan(&page2ID, &page2); err == nil {
			if _, body := s.get("/" + page2); strings.Contains(body, fmt.Sprintf(`/node/%d/translations"`, page2ID)) {
				t.Error("a page (not translated) has the Translations tab")
			}
		}
		s.checkLog()
	})
}
