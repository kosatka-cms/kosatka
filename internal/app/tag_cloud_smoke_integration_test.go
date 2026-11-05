//go:build dbintegration

package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestTagCloudBlock - an administrator places the tag cloud, its
// placement page offers its settings, a value it doesn't accept isn't
// saved, a valid one is, and the front page's cloud follows them: the
// terms of the chosen vocabulary on the most published materials.
func TestTagCloudBlock(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		for _, title := range []string{"First", "Second"} {
			v := url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {title}, body[1]: {"<p>Body</p>"}, "status": {"1"}}
			if resp := s.postMultipart("/create/article", v); resp.StatusCode != http.StatusFound {
				t.Fatalf("create %q: %d", title, resp.StatusCode)
			}
		}
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := db.DB.Exec(db.Dialect.Rebind(q), args...); err != nil {
				t.Fatal(err)
			}
		}
		exec(`INSERT INTO taxonomy_vocabulary (id, name, slug, public_term_pages) VALUES (50, 'Cloud tags', 'cloud-tags', ?), (51, 'Kinds', 'kinds', ?)`, true, true)
		exec(`INSERT INTO taxonomy_term_data (id, vid, name, slug, weight) VALUES (500, 50, 'Popular tag', 'cloud-tags/popular', 0), (501, 50, 'Rare tag', 'cloud-tags/rare', 0), (510, 51, 'A kind', 'kinds/a-kind', 0)`)
		exec(`INSERT INTO taxonomy_index (nid, tid) SELECT id, 500 FROM nodes WHERE title IN ('First', 'Second')`)
		exec(`INSERT INTO taxonomy_index (nid, tid) SELECT id, 501 FROM nodes WHERE title = 'First'`)
		exec(`INSERT INTO taxonomy_index (nid, tid) SELECT id, 510 FROM nodes WHERE title IN ('First', 'Second')`)

		if resp := s.post("/admin/config/blocks/plugin", url.Values{"csrf_token": {s.csrf("/admin/config/blocks")}, "plugin_id": {"tag_cloud"}, "region": {"sidebar"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("place the tag cloud: %d", resp.StatusCode)
		}
		var id int
		if err := db.DB.QueryRow(`SELECT id FROM block_placements WHERE plugin_id = 'tag_cloud'`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		edit := fmt.Sprintf("/admin/config/blocks/plugin/%d/edit", id)
		_, page := s.get(edit)
		for _, want := range []string{`name="setting_vocabulary"`, `<option value="50" >Cloud tags</option>`, `name="setting_count"`, `name="setting_levels"`} {
			if !strings.Contains(page, want) {
				t.Errorf("the placement page has no %s", want)
			}
		}
		save := func(vocabulary, count string) string {
			t.Helper()
			resp := s.post(edit, url.Values{"csrf_token": {s.csrf(edit)}, "region": {"sidebar"}, "title": {"Cloud"}, "show_title": {"1"}, "visibility_scope": {"all"},
				"setting_vocabulary": {vocabulary}, "setting_count": {count}, "setting_levels": {"6"}})
			return resp.Header.Get("Location")
		}
		if where := save("50", "0"); !strings.Contains(where, "error=settings") {
			t.Errorf("count 0 saved: %s", where)
		}
		if where := save("50", "1"); strings.Contains(where, "error") {
			t.Fatalf("valid settings refused: %s", where)
		}
		var raw string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT plugin_config FROM block_placements WHERE id = ?`), id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var config map[string]string
		if err := json.Unmarshal([]byte(raw), &config); err != nil || config["count"] != "1" || config["vocabulary"] != "50" || config["levels"] != "6" {
			t.Errorf("stored %s %v", raw, err)
		}

		// The save rebuilds the blocks' cache in the background.
		var home string
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if _, home = s.get("/"); strings.Contains(home, "tag-cloud-item") {
				break
			}
		}
		if !strings.Contains(home, `<a href="/cloud-tags/popular" title="2 materials">Popular tag</a>`) {
			t.Errorf("the front page's cloud has no popular tag:\n%s", home)
		}
		if strings.Contains(home, "Rare tag") || strings.Contains(home, "A kind") {
			t.Error("the cloud shows a term beyond its count or of another vocabulary")
		}
	})
}
