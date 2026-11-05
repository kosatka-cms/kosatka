//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestLanguageMenus - an English site with Russian: a main menu link
// named in Russian shows so on /ru/ with its address under /ru/; a link
// for English pages only isn't on /ru/; a link to an article leads on
// /ru/ to its published Russian translation; the logo leads to /ru/.
func TestLanguageMenus(t *testing.T) {
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
			t.Fatalf("settings: %s", resp.Header.Get("Location"))
		}

		var menu int
		if err := db.DB.QueryRow(`SELECT id FROM menus WHERE machine_name = 'main'`).Scan(&menu); err != nil {
			t.Fatal(err)
		}
		links := fmt.Sprintf("/admin/config/menus/%d/links", menu)
		addLink := func(title string, fields url.Values) int {
			t.Helper()
			fields.Set("csrf_token", s.csrf(links))
			fields.Set("title", title)
			if resp := s.post(links, fields); !strings.Contains(resp.Header.Get("Location"), "success=1") {
				t.Fatalf("adding %q: %s", title, resp.Header.Get("Location"))
			}
			var id int
			if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id FROM menu_links WHERE title = ?`), title).Scan(&id); err != nil {
				t.Fatal(err)
			}
			return id
		}
		editLink := func(id int, fields url.Values) {
			t.Helper()
			path := fmt.Sprintf("%s/%d", links, id)
			if _, page := s.get(path + "/edit"); !strings.Contains(page, `name="text_ru"`) {
				t.Fatalf("the link's form has no Russian name")
			}
			fields.Set("csrf_token", s.csrf(path+"/edit"))
			fields.Set("enabled", "1")
			if resp := s.post(path, fields); !strings.Contains(resp.Header.Get("Location"), "success=1") {
				t.Fatalf("saving link %d: %s\n%s", id, resp.Header.Get("Location"), s.log.String())
			}
		}
		calendar := addLink("Calendar", url.Values{"link_type": {"url"}, "link_url": {"/calendar"}})
		editLink(calendar, url.Values{"title": {"Calendar"}, "link_type": {"url"}, "link_url": {"/calendar"}, "text_ru": {"Календарь"}})
		englishOnly := addLink("Shop", url.Values{"link_type": {"url"}, "link_url": {"/shop"}})
		editLink(englishOnly, url.Values{"title": {"Shop"}, "link_type": {"url"}, "link_url": {"/shop"}, "languages_only": {"1"}, "languages": {"en"}})

		// An article translated into Russian, published, in the menu.
		var nid int
		if err := db.DB.QueryRow(`SELECT id FROM nodes WHERE type = 'article' AND status = 1 ORDER BY id LIMIT 1`).Scan(&nid); err != nil {
			t.Fatal(err)
		}
		tab := fmt.Sprintf("/node/%d/translations", nid)
		resp := s.post(tab+"/ru", url.Values{"csrf_token": {s.csrf(tab)}})
		var tid int
		if _, err := fmt.Sscanf(resp.Header.Get("Location"), "/node/%d/edit", &tid); err != nil {
			t.Fatalf("translating: %s", resp.Header.Get("Location"))
		}
		var tslug string
		_ = db.DB.QueryRow(db.Dialect.Rebind(`SELECT slug FROM nodes WHERE id = ?`), tid).Scan(&tslug)
		if resp := s.post(fmt.Sprintf("/node/%d/publish", tid), url.Values{"csrf_token": {s.csrf(tab)}}); resp.StatusCode >= 400 {
			t.Fatalf("publishing: %d", resp.StatusCode)
		}
		addLink("Featured", url.Values{"link_type": {"node"}, "link_node_id": {fmt.Sprint(nid)}})

		guest := s.visitor()
		var ru string
		// The menus are cached: rebuilt after the saves in the background.
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if _, ru = guest.get("/ru/"); strings.Contains(ru, "Календарь") && strings.Contains(ru, "Featured") {
				break
			}
		}
		for _, want := range []string{`href="/ru/calendar"`, "Календарь", `href="/ru/` + tslug + `"`, `href="/ru/"`} {
			if !strings.Contains(ru, want) {
				t.Errorf("/ru/ has no %s", want)
			}
		}
		if strings.Contains(ru, `href="/ru/shop"`) || strings.Contains(ru, ">Shop<") {
			t.Error("/ru/ has the English-only link")
		}
		_, en := guest.get("/")
		for _, want := range []string{`href="/calendar"`, ">Calendar<", `href="/shop"`} {
			if !strings.Contains(en, want) {
				t.Errorf("/ has no %s", want)
			}
		}
		s.checkLog()
	})
}
