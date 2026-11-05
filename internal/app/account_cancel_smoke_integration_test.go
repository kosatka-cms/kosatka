//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestAccountCancel - a role with "cancel own account": its member sees
// "Delete account" on their profile form; a wrong password or no tick
// sends them back; then the account is gone and they're signed out, their
// article stays without an author. With the account settings' "delete
// their content too" another member's articles go with them, the one in
// the trash too. An
// administrator has no such card and can't post it.
func TestAccountCancel(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		one := func(q string, args ...any) int {
			t.Helper()
			var n int
			if err := db.DB.QueryRow(db.Dialect.Rebind(q), args...).Scan(&n); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
			return n
		}
		if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO roles (name, permissions, created_at, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
			"members", `{"cancel own account": true, "create article content": true}`); err != nil {
			t.Fatal(err)
		}
		members := one(`SELECT id FROM roles WHERE name = 'members'`)
		member := func(name string) (*site, int) {
			t.Helper()
			if _, ok := s.submit("/admin/people/create", "/admin/people/create", url.Values{"password_mode": {"set"}, "username": {name}, "email": {name + "@example.com"},
				"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {fmt.Sprint(members)}}); !ok {
				t.Fatalf("create %s", name)
			}
			uid := one(`SELECT id FROM users WHERE username = ?`, name)
			if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO nodes (type, title, slug, status, created_by, created_at, updated_at) VALUES ('article', ?, ?, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
				"Notes of "+name, "notes-of-"+name, uid); err != nil {
				t.Fatal(err)
			}
			v := s.visitor()
			v.signInAs(name, memberPassword)
			return v, uid
		}
		cancel := func(v *site, form url.Values) string {
			t.Helper()
			form.Set("csrf_token", v.csrf("/me/edit"))
			return v.post("/me/cancel", form).Header.Get("Location")
		}

		ann, annID := member("ann")
		if _, page := ann.get("/me/edit"); !strings.Contains(page, `action="/me/cancel"`) || !strings.Contains(page, "stay on the site without an author") {
			t.Fatal("ann's profile form has no Delete account")
		}
		if where := cancel(ann, url.Values{"password": {"wrong"}, "confirm": {"1"}}); !strings.Contains(where, "cancel_error=password") {
			t.Errorf("a wrong password: %s", where)
		}
		if where := cancel(ann, url.Values{"password": {memberPassword}}); !strings.Contains(where, "cancel_error=confirm") {
			t.Errorf("no tick: %s", where)
		}
		if where := cancel(ann, url.Values{"password": {memberPassword}, "confirm": {"1"}}); where != "/login?msg=account_deleted" {
			t.Errorf("deleting: %s", where)
		}
		if n := one(`SELECT COUNT(*) FROM users WHERE id = ?`, annID); n != 0 {
			t.Error("ann's account is still there")
		}
		if n := one(`SELECT COUNT(*) FROM nodes WHERE slug = 'notes-of-ann' AND created_by IS NULL`); n != 1 {
			t.Error("ann's article isn't kept without an author")
		}
		if code, _ := ann.get("/me/edit"); code == http.StatusOK {
			t.Error("ann is still signed in")
		}

		if _, err := db.DB.Exec(`UPDATE account_settings SET cancel_content = 'delete'`); err != nil {
			t.Fatal(err)
		}
		bob, bobID := member("bob")
		// One more of his in the trash: it goes too.
		if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO nodes (type, title, slug, status, created_by, created_at, updated_at, deleted_at, deleted_by) VALUES ('article', 'Old', 'old-of-bob', 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)`), bobID, bobID); err != nil {
			t.Fatal(err)
		}
		if _, page := bob.get("/me/edit"); !strings.Contains(page, "content, comments, private messages and files are deleted for good") {
			t.Error("bob's card doesn't say his content goes too")
		}
		if where := cancel(bob, url.Values{"password": {memberPassword}, "confirm": {"1"}}); where != "/login?msg=account_deleted" {
			t.Errorf("deleting bob: %s", where)
		}
		// His content goes in the background.
		deadline := time.Now().Add(10 * time.Second)
		for one(`SELECT COUNT(*) FROM users WHERE id = ?`, bobID)+one(`SELECT COUNT(*) FROM nodes WHERE slug IN ('notes-of-bob', 'old-of-bob')`) > 0 {
			if time.Now().After(deadline) {
				t.Fatal("bob or his articles are still there")
			}
			time.Sleep(100 * time.Millisecond)
		}

		if _, page := s.get("/me/edit"); strings.Contains(page, `action="/me/cancel"`) {
			t.Error("the administrator is offered Delete account")
		}
		if code := s.post("/me/cancel", url.Values{"csrf_token": {s.csrf("/me/edit")}, "password": {"x"}, "confirm": {"1"}}).StatusCode; code != http.StatusForbidden {
			t.Errorf("the administrator's cancel: %d", code)
		}
		s.checkLog()
	})
}
