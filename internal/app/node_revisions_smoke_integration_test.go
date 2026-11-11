//go:build dbintegration

package app

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestNodeRevisions - the history of a type that keeps one (the
// starter's article): creating, editing, publishing from the node's page
// and from the content list each make a version, a save changing nothing
// but the white space of a text doesn't; beyond the type's keep the
// oldest go; the history and the comparison show them, and bringing a
// version back restores its title and text and leaves the publishing.
func TestNodeRevisions(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		var on bool
		if err := db.DB.QueryRow(`SELECT revisions FROM content_types WHERE machine_name = 'article'`).Scan(&on); err != nil || !on {
			t.Fatalf("the starter's article keeps no history (%v)", err)
		}
		if _, err := db.DB.Exec(`UPDATE content_types SET revisions_keep = 5 WHERE machine_name = 'article'`); err != nil {
			t.Fatal(err)
		}
		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		save := func(path, title, text string) {
			t.Helper()
			v := url.Values{"csrf_token": {s.csrf(path)}, "title": {title}, body[1]: {text}, "status": {"1"}}
			if resp := s.postMultipart(path, v); resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("POST %s %q: %d\n%s", path, title, resp.StatusCode, s.log.String())
			}
		}
		save("/create/article", "History", "<p>One two three</p>")
		var id int
		if err := db.DB.QueryRow(`SELECT id FROM nodes WHERE title = 'History'`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		revisions := func() []string {
			t.Helper()
			rows, err := db.DB.Query(db.Dialect.Rebind(`SELECT kind, changed FROM node_revisions WHERE node_id = ? ORDER BY id`), id)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rows.Close() }()
			var out []string
			for rows.Next() {
				var kind string
				var changed []byte
				if err := rows.Scan(&kind, &changed); err != nil {
					t.Fatal(err)
				}
				out = append(out, kind+strings.TrimPrefix(strings.ReplaceAll(string(changed), " ", ""), "null"))
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			return out
		}
		edit := fmt.Sprintf("/node/%d/edit", id)
		save(edit, "History edited", "<p>One two three</p>")
		// Only the text's white space: nothing to keep.
		save(edit, "History edited", "<p>One two three</p>\n")
		save(edit, "History edited", "<p>One two four</p>")
		if resp := s.post(fmt.Sprintf("/node/%d/unpublish", id), url.Values{"csrf_token": {s.csrf("/admin/content")}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("unpublish: %d", resp.StatusCode)
		}
		// Published again from the content list (the bulk action).
		req, _ := http.NewRequest(http.MethodPost, s.base+"/api/admin/bulk/content", bytes.NewBufferString(fmt.Sprintf(`{"action":"status_publish","ids":[%d]}`, id)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", s.csrf("/admin/content"))
		resp, err := s.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("bulk publish: %v %v", err, resp)
		}
		_ = resp.Body.Close()
		// The title's edit leaves the address.
		want := []string{"create", `edit["title"]`, `edit["field:field_body"]`, `edit["status"]`, `edit["status"]`}
		if got := revisions(); strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("revisions %v, want %v", got, want)
		}
		// One more beyond the keep of 5: the first one goes.
		save(edit, "History final", "<p>One two four</p>")
		got := revisions()
		if len(got) != 5 || got[0] != `edit["title"]` {
			t.Errorf("after the keep: %v", got)
		}

		// The history lists them; two compare by words.
		code, page := s.get(fmt.Sprintf("/node/%d/revisions", id))
		if code != http.StatusOK || !strings.Contains(page, "History final") || strings.Count(page, `name="to"`) != 5 {
			t.Fatalf("the history: %d\n%s", code, page)
		}
		var first, current int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT MIN(id), MAX(id) FROM node_revisions WHERE node_id = ?`), id).Scan(&first, &current); err != nil {
			t.Fatal(err)
		}
		code, page = s.get(fmt.Sprintf("/node/%d/revisions/compare?from=%d&to=%d", id, first, current))
		if code != http.StatusOK || !strings.Contains(page, "<del>three</del>") || !strings.Contains(page, "<ins>four</ins>") {
			t.Errorf("the comparison: %d\n%s", code, page)
		}

		// Bringing back the first one kept: its title and text, the
		// publishing as it is now (unpublished behind the history's back,
		// so the revert's own version records it too).
		if _, err := db.DB.Exec(db.Dialect.Rebind(`UPDATE nodes SET status = 0 WHERE id = ?`), id); err != nil {
			t.Fatal(err)
		}
		if resp := s.post(fmt.Sprintf("/node/%d/revisions/%d/revert", id, first), url.Values{"csrf_token": {s.csrf("/admin/content")}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("revert: %d", resp.StatusCode)
		}
		var title string
		var status int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT title, status FROM nodes WHERE id = ?`), id).Scan(&title, &status); err != nil {
			t.Fatal(err)
		}
		var text string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT v.value FROM field_value_text v JOIN field_storages f ON f.id = v.field_id WHERE v.node_id = ? AND f.machine_name = 'field_body'`), id).Scan(&text); err != nil {
			t.Fatal(err)
		}
		if title != "History edited" || text != "<p>One two three</p>" || status != 0 {
			t.Errorf("after the revert: %q, %q, status %d", title, text, status)
		}
		if got := revisions(); got[len(got)-1] != `revert["title","status","field:field_body"]` {
			t.Errorf("the revert's own version: %v", got)
		}
		s.checkLog()
	})
}
