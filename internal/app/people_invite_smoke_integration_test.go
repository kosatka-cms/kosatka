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
	"github.com/kosatka-cms/kosatka/pkg/randtoken"
)

// TestPeopleInvite - an administrator invites a person (no password, no
// login typed): the account is there without a password, its login made
// of the address, the list shows the invitation; by its link the person
// chooses a login and a password and is signed in; the link works once,
// and the password signs in after.
func TestPeopleInvite(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		var roleID int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id FROM roles WHERE name = ?`), entity.RoleUser).Scan(&roleID); err != nil {
			t.Fatal(err)
		}
		where, ok := s.submit("/admin/people/create", "/admin/people/create", url.Values{"password_mode": {"invite"}, "username": {""},
			"email": {"invitee@example.com"}, "status": {"1"}, "role_id": {fmt.Sprint(roleID)}})
		if !ok || !strings.Contains(where, "/admin/people?invited=") {
			t.Fatalf("the invitation: %q", where)
		}
		var id int
		var name, hash string
		if err := db.DB.QueryRow(`SELECT id, username, password_hash FROM users WHERE email = 'invitee@example.com'`).Scan(&id, &name, &hash); err != nil {
			t.Fatal(err)
		}
		if name != "invitee" || hash != entity.PasswordUnset {
			t.Errorf("the invited account: %q %q", name, hash)
		}
		if _, page := s.get("/admin/people"); !strings.Contains(page, fmt.Sprintf(`formaction="/admin/people/%d/invite"`, id)) {
			t.Error("the list has no invitation to send again")
		}

		// The link the e-mail would carry (its raw token isn't kept).
		raw := "smoke-invite-token"
		if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO user_auth_tokens (user_id, token_hash, kind, expires_at, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP + INTERVAL '1' DAY, CURRENT_TIMESTAMP)`),
			id, randtoken.Hash(raw), entity.UserAuthTokenKindInvite); err != nil {
			t.Fatal(err)
		}
		v := s.visitor()
		link := "/user/invite?token=" + raw
		if code, page := v.get(link); code != http.StatusOK || !strings.Contains(page, `name="username" value="invitee"`) {
			t.Fatalf("the invitation's page: %d", code)
		}
		resp := v.post("/user/invite", url.Values{"token": {raw}, "username": {"Invitee"}, "password": {memberPassword},
			"password_confirm": {memberPassword}, "csrf_token": {v.csrf(link)}})
		if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/me" {
			t.Fatalf("accepting: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
		}
		if _, page := v.get(link); !strings.Contains(page, "This invitation link is invalid") {
			t.Error("the link works twice")
		}
		v.visitor().signInAs("Invitee", memberPassword)
	})
}
