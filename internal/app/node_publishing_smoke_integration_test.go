//go:build dbintegration

package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestNodePublishing - a node's publishing, author and date: an editor who
// may not administer content can't set them, whatever the request says -
// a new node gets its type's publishing (on moderation by default) and
// its author, an edit keeps the node's own (a published node stays
// published); an administrator sets them in the form, a new node's form
// starting published (not on moderation) unless the type's are
// "unpublished".
func TestNodePublishing(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		// The type's publishing is "on moderation": the administrator's form
		// starts published, not on moderation (they're the moderator).
		if !regexp.MustCompile(`name="status" value="1" checked`).MatchString(form) || regexp.MustCompile(`name="on_moderation" value="1" checked`).MatchString(form) {
			t.Error("the administrator's new article form: want \"Published\" checked, \"In moderation\" not")
		}
		node := func(title string) (id, status, onModeration, author int, year int) {
			t.Helper()
			var created string
			if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id, status, on_moderation, created_by, created_at FROM nodes WHERE title = ?`), title).
				Scan(&id, &status, &onModeration, &author, &created); err != nil {
				t.Fatalf("%q: %v", title, err)
			}
			_, _ = fmt.Sscanf(created, "%d", &year)
			return id, status, onModeration, author, year
		}
		var adminID int
		if err := db.DB.QueryRow(`SELECT MIN(id) FROM users`).Scan(&adminID); err != nil {
			t.Fatal(err)
		}

		// A member who may create articles and edit their own, not
		// administer content.
		var roleID int
		var perms string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id, permissions FROM roles WHERE name = ?`), entity.RoleUser).Scan(&roleID, &perms); err != nil {
			t.Fatal(err)
		}
		granted := map[string]any{}
		_ = json.Unmarshal([]byte(perms), &granted)
		granted[service.PermCreate("article")] = true
		granted[service.PermEditOwn("article")] = true
		b, _ := json.Marshal(granted)
		if _, err := db.DB.Exec(db.Dialect.Rebind(`UPDATE roles SET permissions = ? WHERE id = ?`), string(b), roleID); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit("/admin/users/create", "/admin/users/create", url.Values{"password_mode": {"set"}, "username": {"member"}, "email": {"member@example.com"},
			"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {fmt.Sprint(roleID)}}); !ok {
			t.Fatal("create member")
		}
		member := s.visitor()
		member.signInAs("member", memberPassword)

		// The member's request asks to publish past moderation, as the
		// administrator, back in 2001.
		forged := url.Values{"status": {"1"}, "on_moderation": {"0"}, "created_by": {fmt.Sprint(adminID)}, "created_at": {"2001-01-01T00:00"}}
		post := func(as *site, path, title string, extra url.Values) {
			t.Helper()
			v := url.Values{"csrf_token": {as.csrf(path)}, "title": {title}, body[1]: {"<p>Body</p>"}}
			for k, vs := range extra {
				v[k] = vs
			}
			if resp := as.postMultipart(path, v); resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("POST %s %q: %d\n%s", path, title, resp.StatusCode, s.log.String())
			}
		}
		post(member, "/create/article", "By a member", forged)
		id, status, onModeration, author, year := node("By a member")
		if status != 1 || onModeration != 1 || author == adminID || year == 2001 {
			t.Errorf("a member's new node: status %d, on moderation %d, author %d (admin %d), year %d - want on moderation, the member's, now",
				status, onModeration, author, adminID, year)
		}

		// Approved, then edited by its author with a form that has no
		// publishing (or a request asking to unpublish): still published.
		if _, err := db.DB.Exec(db.Dialect.Rebind(`UPDATE nodes SET on_moderation = 0 WHERE id = ?`), id); err != nil {
			t.Fatal(err)
		}
		edit := fmt.Sprintf("/node/%d/edit", id)
		post(member, edit, "By a member", nil)
		post(member, edit, "By a member", url.Values{"status": {"0"}, "created_by": {fmt.Sprint(adminID)}})
		if _, status, onModeration, author, _ := node("By a member"); status != 1 || onModeration != 0 || author == adminID {
			t.Errorf("after the author's edits: status %d, on moderation %d, author %d - want published, the member's", status, onModeration, author)
		}

		// The type's publishing: unpublished.
		if _, err := db.DB.Exec(`UPDATE content_types SET default_publishing = 'unpublished' WHERE machine_name = 'article'`); err != nil {
			t.Fatal(err)
		}
		post(member, "/create/article", "Unpublished by its type", forged)
		if _, status, onModeration, _, _ := node("Unpublished by its type"); status != 0 || onModeration != 0 {
			t.Errorf("unpublished by its type: status %d, on moderation %d", status, onModeration)
		}

		// The administrator sets them; the new node's form starts from the
		// type's publishing (published here).
		if _, err := db.DB.Exec(`UPDATE content_types SET default_publishing = 'published' WHERE machine_name = 'article'`); err != nil {
			t.Fatal(err)
		}
		if _, page := s.get("/create/article"); !regexp.MustCompile(`name="status" value="1" checked`).MatchString(page) {
			t.Error("the administrator's new article form: \"Published\" not checked by the type's publishing")
		}
		post(s, "/create/article", "By the administrator", url.Values{"on_moderation": {"1"}})
		if _, status, onModeration, _, _ := node("By the administrator"); status != 0 || onModeration != 1 {
			t.Errorf("set by the administrator: status %d, on moderation %d, want 0, 1 (as the form says)", status, onModeration)
		}
		s.checkLog()
	})
}
