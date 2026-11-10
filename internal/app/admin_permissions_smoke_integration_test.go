//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestAdminPermissions - a role with the admin and the users but not the
// permissions ("administer permissions") can't raise itself: the roles
// and the permission matrix are closed to it, it can't give itself or
// anyone another role, and a full-access account is not its to change.
func TestAdminPermissions(t *testing.T) {
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
		if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO roles (name, permissions, created_at, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
			"clerks", `{"access administration pages": true, "administer users": true}`); err != nil {
			t.Fatal(err)
		}
		clerks := id(`SELECT id FROM roles WHERE name = 'clerks'`)
		admins := id(`SELECT id FROM roles WHERE name = ?`, entity.RoleAdmin)
		if _, ok := s.submit("/admin/users/create", "/admin/users/create", url.Values{"password_mode": {"set"}, "username": {"clerk"}, "email": {"clerk@example.com"},
			"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {fmt.Sprint(clerks)}}); !ok {
			t.Fatal("create the clerk")
		}
		clerkID := id(`SELECT id FROM users WHERE username = 'clerk'`)
		adminID := id(`SELECT MIN(id) FROM users WHERE role_id = ?`, admins)

		clerk := s.visitor()
		clerk.signInAs("clerk", memberPassword)
		for path, want := range map[string]int{
			"/admin/users":             http.StatusOK,
			"/admin/users/permissions": http.StatusForbidden,
			"/admin/users/roles":       http.StatusForbidden,
		} {
			if code, _ := clerk.get(path); code != want {
				t.Errorf("the clerk on %s: %d, want %d", path, code, want)
			}
		}

		// Their own profile, as an administrator of users: no roles to
		// choose, a role sent anyway is ignored.
		edit := fmt.Sprintf("/user/%d/edit", clerkID)
		if _, page := clerk.get(edit); strings.Contains(page, `name="role_id"`) {
			t.Error("the clerk's profile form offers roles")
		}
		clerk.submit(edit, "", url.Values{"role_id": {fmt.Sprint(admins)}})
		if got := id(`SELECT role_id FROM users WHERE id = ?`, clerkID); got != clerks {
			t.Errorf("the clerk's role after the save: %d, want %d", got, clerks)
		}
		// Creating an account: only the registered users' role.
		if _, page := clerk.get("/admin/users/create"); strings.Contains(page, fmt.Sprintf(`<option value="%d"`, admins)) {
			t.Error("the clerk may create an administrator")
		}

		// A full-access account: not its profile form, not its blocking.
		if code, _ := clerk.get(fmt.Sprintf("/user/%d/edit", adminID)); code != http.StatusForbidden {
			t.Errorf("the clerk on the administrator's profile form: %d, want 403", code)
		}
		if code, _ := clerk.get("/user/999999/edit"); code != http.StatusNotFound {
			t.Errorf("the clerk on a profile form of no user: %d, want 404", code)
		}
		req, _ := http.NewRequest(http.MethodPost, clerk.base+"/api/admin/bulk/users", strings.NewReader(fmt.Sprintf(`{"action":"block","ids":[%d,%d]}`, adminID, clerkID)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", clerk.csrf("/admin/users"))
		resp, err := clerk.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if got := id(`SELECT COUNT(*) FROM users WHERE id IN (?, ?) AND status = ?`, adminID, clerkID, entity.UserStatusActive); got != 2 {
			t.Errorf("after the clerk's block of the administrator and itself: %d of 2 active (%d)", got, resp.StatusCode)
		}
		// Last: on a site without the check the save would take the
		// clerk's own permissions away.
		if resp := clerk.post("/admin/users/permissions", url.Values{"csrf_token": {clerk.csrf("/admin/users")}, "grant": {fmt.Sprintf("%d|full_access", clerks)}}); resp.StatusCode != http.StatusForbidden {
			t.Errorf("the clerk saving the permissions: %d, want 403", resp.StatusCode)
		}
		s.checkLog()
	})
}
