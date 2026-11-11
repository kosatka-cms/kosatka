//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestOwnDrafts - an author who may edit only their own content and may
// not view unpublished content creates an item of a type whose new
// content is unpublished: they open it and edit it (they may edit it);
// a guest and another member don't see it.
func TestOwnDrafts(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := db.DB.Exec(db.Dialect.Rebind(q), args...); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
		exec(`UPDATE content_types SET default_publishing = ? WHERE machine_name = 'article'`, "unpublished")
		exec(`INSERT INTO roles (name, permissions, created_at, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			"writers", `{"create article content": true, "edit own article content": true}`)
		var writers int
		if err := db.DB.QueryRow(`SELECT id FROM roles WHERE name = 'writers'`).Scan(&writers); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"writer", "reader"} {
			role := fmt.Sprint(writers)
			if _, ok := s.submit("/admin/users/create", "/admin/users/create", url.Values{"password_mode": {"set"}, "username": {name}, "email": {name + "@example.com"},
				"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {role}}); !ok {
				t.Fatalf("create %s", name)
			}
		}

		writer := s.visitor()
		writer.signInAs("writer", memberPassword)
		_, form := writer.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		resp := writer.postMultipart("/create/article", url.Values{"csrf_token": {writer.csrf("/create/article")}, "title": {"A draft of mine"}, body[1]: {"<p>Text</p>"}})
		if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		var id, status int
		var slug string
		if err := db.DB.QueryRow(`SELECT id, status, slug FROM nodes WHERE title = 'A draft of mine'`).Scan(&id, &status, &slug); err != nil {
			t.Fatal(err)
		}
		if status != 0 {
			t.Fatalf("the draft is published (%d)", status)
		}
		for _, path := range []string{"/" + slug, fmt.Sprintf("/node/%d/edit", id)} {
			if code, _ := writer.get(path); code != http.StatusOK {
				t.Errorf("the author on %s: %d", path, code)
			}
		}
		if _, ok := writer.submit(fmt.Sprintf("/node/%d/edit", id), "", url.Values{"title": {"A draft of mine, edited"}}); !ok {
			t.Error("the author saving their draft")
		}

		reader := s.visitor()
		reader.signInAs("reader", memberPassword)
		if code, _ := reader.get("/" + slug); code == http.StatusOK {
			t.Error("another member sees the draft")
		}
		if code, _ := s.visitor().get("/" + slug); code == http.StatusOK {
			t.Error("a guest sees the draft")
		}
		s.checkLog()
	})
}
