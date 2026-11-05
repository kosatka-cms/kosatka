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

// TestLanguageBlocks - an English site with Russian: the sidebar's
// Popular block titled in Russian shows so on /ru/, as is on /; shown
// on the English pages only, it leaves /ru/.
func TestLanguageBlocks(t *testing.T) {
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

		var placement int
		var region string
		if err := db.DB.QueryRow(`SELECT bp.id, bp.region FROM block_placements bp JOIN view_displays v ON v.id = bp.view_display_id WHERE v.machine_name = 'popular' ORDER BY bp.id LIMIT 1`).Scan(&placement, &region); err != nil {
			t.Fatal(err)
		}
		edit := fmt.Sprintf("/admin/config/blocks/view/%d/edit", placement)
		if _, page := s.get(edit); !strings.Contains(page, `name="placement_title_ru"`) {
			t.Fatal("the placement's form has no Russian title")
		}
		save := func(fields url.Values) {
			t.Helper()
			fields.Set("csrf_token", s.csrf(edit))
			fields.Set("region", region)
			fields.Set("show_title", "1")
			fields.Set("visibility_scope", string(entity.BlockVisibilityAll))
			if resp := s.post(edit, fields); resp.StatusCode != http.StatusSeeOther || strings.Contains(resp.Header.Get("Location"), "error") {
				t.Fatalf("saving the placement: %d %s\n%s", resp.StatusCode, resp.Header.Get("Location"), s.log.String())
			}
		}
		save(url.Values{"title": {"Popular"}, "placement_title_ru": {"Популярное"}})
		guest := s.visitor()
		var ru string
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if _, ru = guest.get("/ru/"); strings.Contains(ru, "<h2>Популярное</h2>") {
				break
			}
		}
		if !strings.Contains(ru, "<h2>Популярное</h2>") {
			t.Error("/ru/ has no Russian title of the block")
		}
		if _, en := guest.get("/"); !strings.Contains(en, "<h2>Popular</h2>") {
			t.Error("/ has no English title of the block")
		}

		save(url.Values{"title": {"Popular"}, "placement_title_ru": {"Популярное"}, "languages_only": {"1"}, "languages": {"en"}})
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if _, ru = guest.get("/ru/"); !strings.Contains(ru, "Популярное") {
				break
			}
		}
		if strings.Contains(ru, "Популярное") {
			t.Error("/ru/ shows the English-only block")
		}
		if _, en := guest.get("/"); !strings.Contains(en, "<h2>Popular</h2>") {
			t.Error("/ lost the English-only block")
		}
		s.checkLog()
	})
}
