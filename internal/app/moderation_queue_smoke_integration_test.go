//go:build dbintegration

package app

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestModerationQueueBulk - a role with only the moderation permission
// opens the queue and takes items off moderation in bulk (its confirm
// page and API are open to it), without the publishing buttons it may
// not use; the content bulk API stays the admin's.
func TestModerationQueueBulk(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		id := func(q string, args ...any) int {
			t.Helper()
			var n int
			if err := db.DB.QueryRow(db.Dialect.Rebind(q), args...).Scan(&n); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
			return n
		}
		_, create := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(create)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		for _, title := range []string{"Harbour notes", "Harbour rumours"} {
			if resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")},
				"title": {title}, body[1]: {"<p>Body</p>"}, "status": {"1"}}); resp.StatusCode != http.StatusFound {
				t.Fatalf("create %q: %d\n%s", title, resp.StatusCode, s.log.String())
			}
		}
		if _, err := db.DB.Exec("UPDATE nodes SET on_moderation = 1 WHERE title LIKE 'Harbour%'"); err != nil {
			t.Fatal(err)
		}
		a, b := id(`SELECT id FROM nodes WHERE title = 'Harbour notes'`), id(`SELECT id FROM nodes WHERE title = 'Harbour rumours'`)

		if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO roles (name, permissions, created_at, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
			"moderators", `{"moderate content": true}`); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit("/admin/people/create", "/admin/people/create", url.Values{"password_mode": {"set"}, "username": {"mod"}, "email": {"mod@example.com"},
			"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {fmt.Sprint(id(`SELECT id FROM roles WHERE name = 'moderators'`))}}); !ok {
			t.Fatal("create the moderator")
		}
		mod := s.visitor()
		mod.signInAs("mod", memberPassword)

		code, queue := mod.get("/admin/moderation")
		if code != http.StatusOK || !strings.Contains(queue, `value="moderation_off"`) || strings.Contains(queue, `value="status_publish"`) || !strings.Contains(queue, `name="ids" value="`) {
			t.Fatalf("the queue for the moderator: %d, bulk %v, publish %v", code, strings.Contains(queue, `value="moderation_off"`), strings.Contains(queue, `value="status_publish"`))
		}
		token := mod.csrf("/admin/moderation")
		resp, err := mod.client.PostForm(mod.base+"/admin/moderation/bulk-confirm", url.Values{"csrf_token": {token}, "action": {"moderation_off"}, "ids": {fmt.Sprint(a), fmt.Sprint(b)}})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		page := string(raw)
		if resp.StatusCode != http.StatusOK || !strings.Contains(page, "/api/admin/bulk/moderation") {
			t.Fatalf("the confirm page: %d\n%s", resp.StatusCode, page)
		}
		bulk := func(path string) int {
			t.Helper()
			req, _ := http.NewRequest(http.MethodPost, mod.base+path, strings.NewReader(fmt.Sprintf(`{"action":"moderation_off","ids":[%d,%d]}`, a, b)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Token", token)
			resp, err := mod.client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			return resp.StatusCode
		}
		if code := bulk("/api/admin/bulk/content"); code != http.StatusForbidden {
			t.Errorf("the content bulk API for the moderator: %d, want 403", code)
		}
		if code := bulk("/api/admin/bulk/moderation"); code != http.StatusOK {
			t.Errorf("the moderation bulk API: %d", code)
		}
		if n := id(`SELECT COUNT(*) FROM nodes WHERE on_moderation = 1 AND title LIKE 'Harbour%'`); n != 0 {
			t.Errorf("%d items still on moderation", n)
		}
		s.checkLog()
	})
}
