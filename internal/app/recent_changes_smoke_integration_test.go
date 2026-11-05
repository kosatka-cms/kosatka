//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestRecentChanges - Content → Changes lists the saves of the content
// of the types keeping their history, newest first, a later one compared
// with the one before; the types not keeping it are named; the content
// list offers the history of the content of a type keeping it.
func TestRecentChanges(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
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
		save("/create/article", "Recent one", "<p>One</p>")
		var id, first, last int
		if err := db.DB.QueryRow(`SELECT id FROM nodes WHERE title = 'Recent one'`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		save(fmt.Sprintf("/node/%d/edit", id), "Recent one", "<p>Two</p>")
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT MIN(id), MAX(id) FROM node_revisions WHERE node_id = ?`), id).Scan(&first, &last); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.Exec(`UPDATE content_types SET revisions = false WHERE machine_name = 'page'`); err != nil {
			t.Fatal(err)
		}

		code, page := s.get("/admin/content/changes")
		if code != http.StatusOK {
			t.Fatalf("the recent changes: %d\n%s", code, page)
		}
		for _, want := range []string{
			">Recent one</a>", fmt.Sprintf(`/node/%d/revisions/compare?from=%d&amp;to=%d`, id, first, last), "Content created",
			"The history of changes is off for 1 type:",
		} {
			if !strings.Contains(page, want) {
				t.Errorf("no %s on the recent changes", want)
			}
		}
		if _, page := s.get("/admin/content/changes?title=nothing+like+it"); strings.Contains(page, ">Recent one</a>") {
			t.Error("the title filter shows another title")
		}
		if _, page := s.get("/admin/content"); !strings.Contains(page, fmt.Sprintf(`href="/node/%d/revisions" class="admin-icon-btn"`, id)) {
			t.Error("no History button in the content list")
		}
	})
}
