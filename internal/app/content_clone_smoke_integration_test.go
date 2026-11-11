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

// TestContentClone - "Make a copy" on an article: a new unpublished one,
// "Copy of …", with the same text, an address of its own, its form opened;
// the original stays as it was. An author who may edit their own article
// copies it only with the "copy content" permission.
func TestContentClone(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		if _, err := db.DB.Exec(`UPDATE content_types SET default_publishing = 'published' WHERE machine_name = 'article'`); err != nil {
			t.Fatal(err)
		}
		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		if resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {`Spring "sale" <now>`}, body[1]: {"<p>Everything half price.</p>"}, "status": {"1"}}); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		var id int
		var slug string
		if err := db.DB.QueryRow(`SELECT id, slug FROM nodes WHERE title LIKE 'Spring "sale"%'`).Scan(&id, &slug); err != nil {
			t.Fatal(err)
		}
		// The copy is asked first, the title escaped once in the question.
		if _, page := s.get("/" + slug); !strings.Contains(page, fmt.Sprintf(`action="/node/%d/clone"`, id)) ||
			!strings.Contains(page, `data-confirm="Make a copy of &#34;Spring &#34;sale&#34; &lt;now&gt;&#34;? It will be an unpublished draft."`) {
			t.Fatal("the page has no copy button")
		}
		clone := fmt.Sprintf("/node/%d/clone", id)
		resp := s.post(clone, url.Values{"csrf_token": {s.csrf("/" + slug)}})
		where := resp.Header.Get("Location")
		if resp.StatusCode != http.StatusSeeOther || !strings.HasSuffix(where, "/edit") {
			t.Fatalf("copy: %d %s", resp.StatusCode, where)
		}
		var copyID, status int
		var copySlug string
		if err := db.DB.QueryRow(`SELECT id, slug, status FROM nodes WHERE title LIKE 'Copy of Spring "sale"%'`).Scan(&copyID, &copySlug, &status); err != nil {
			t.Fatal(err)
		}
		if where != fmt.Sprintf("/node/%d/edit", copyID) || status != 0 || copySlug == slug {
			t.Errorf("the copy: %s, status %d, slug %q (the original's %q)", where, status, copySlug, slug)
		}
		if _, page := s.get(where); !strings.Contains(page, "Everything half price.") || !strings.Contains(page, `value="Copy of Spring &#34;sale&#34; &lt;now&gt;"`) {
			t.Error("the copy's form lacks the text or the title")
		}
		if _, page := s.get("/" + slug); !strings.Contains(page, "Everything half price.") {
			t.Error("the original lost its text")
		}
		if code, _ := s.visitor().get("/" + copySlug); code == http.StatusOK {
			t.Errorf("a guest opens the unpublished copy: %d", code)
		}

		// An author of their own article: no copy without the permission.
		if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO roles (name, permissions, created_at, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
			"writers", `{"create article content": true, "edit own article content": true}`); err != nil {
			t.Fatal(err)
		}
		var writers int
		if err := db.DB.QueryRow(`SELECT id FROM roles WHERE name = 'writers'`).Scan(&writers); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit("/admin/users/create", "/admin/users/create", url.Values{"password_mode": {"set"}, "username": {"writer"}, "email": {"writer@example.com"},
			"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {fmt.Sprint(writers)}}); !ok {
			t.Fatal("create the writer")
		}
		writer := s.visitor()
		writer.signInAs("writer", memberPassword)
		if resp := writer.postMultipart("/create/article", url.Values{"csrf_token": {writer.csrf("/create/article")}, "title": {"Mine to copy"}, body[1]: {"<p>Text</p>"}}); resp.StatusCode != http.StatusFound {
			t.Fatalf("the writer's article: %d", resp.StatusCode)
		}
		var ownID int
		var ownSlug string
		if err := db.DB.QueryRow(`SELECT id, slug FROM nodes WHERE title = 'Mine to copy'`).Scan(&ownID, &ownSlug); err != nil {
			t.Fatal(err)
		}
		ownClone := fmt.Sprintf("/node/%d/clone", ownID)
		if _, page := writer.get("/" + ownSlug); strings.Contains(page, `action="`+ownClone+`"`) {
			t.Error("the writer has a copy button without the permission")
		}
		if resp := writer.post(ownClone, url.Values{"csrf_token": {writer.csrf("/" + ownSlug)}}); resp.StatusCode != http.StatusForbidden {
			t.Errorf("the writer copying without the permission: %d", resp.StatusCode)
		}
		if _, err := db.DB.Exec(db.Dialect.Rebind(`UPDATE roles SET permissions = ? WHERE id = ?`),
			`{"create article content": true, "edit own article content": true, "copy content": true}`, writers); err != nil {
			t.Fatal(err)
		}
		if _, page := writer.get("/" + ownSlug); !strings.Contains(page, `action="`+ownClone+`"`) {
			t.Error("the writer has no copy button with the permission")
		}
		if resp := writer.post(ownClone, url.Values{"csrf_token": {writer.csrf("/" + ownSlug)}}); resp.StatusCode != http.StatusSeeOther {
			t.Errorf("the writer copying with the permission: %d", resp.StatusCode)
		}
	})
}
