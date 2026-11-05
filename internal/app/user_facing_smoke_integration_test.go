//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestUserFacing - what users hit:
//   - a profile whose alias has "messages" in it is a profile, not the
//     messages tab (that's only /users/<alias>/messages);
//   - an archived thread still opens (titled by its subject) and takes a
//     reply from its member, a stranger's reply is refused (403, not 500);
//   - a comment only on a node its author may see, a reply only to a
//     comment of the same node;
//   - a profile edit checks the name and the email as registration does:
//     another user's name is refused (not a 500 from the unique index),
//     the email is stored lowercase.
func TestUserFacing(t *testing.T) {
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
		role := fmt.Sprint(id(`SELECT id FROM roles WHERE name = ?`, entity.RoleUser))
		for _, name := range []string{"messages-fan", "member", "stranger"} {
			if _, ok := s.submit("/admin/people/create", "/admin/people/create", url.Values{"password_mode": {"set"}, "username": {name}, "email": {name + "@example.com"},
				"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {role}}); !ok {
				t.Fatalf("create %s", name)
			}
		}

		// A profile whose alias has "messages" in it.
		if code, page := s.visitor().get("/" + entity.UserPathPrefix + "/messages-fan"); code != http.StatusOK || !strings.Contains(page, "messages-fan") {
			t.Errorf("a guest on /%s/messages-fan: %d, want the profile", entity.UserPathPrefix, code)
		}

		// A thread from the administrator to the member, archived by the
		// administrator.
		member := id(`SELECT id FROM users WHERE username = 'member'`)
		if resp := s.post("/me/messages/new", url.Values{"csrf_token": {s.csrf("/me/messages/new")}, "subject": {"Hello"},
			"body": {"<p>Hi there</p>"}, "recipients[]": {fmt.Sprint(member)}}); resp.StatusCode >= 400 {
			t.Fatalf("start a thread: %d\n%s", resp.StatusCode, s.log.String())
		}
		thread := fmt.Sprintf("/me/messages/thread/%d", id(`SELECT MAX(id) FROM pm_threads`))
		if resp := s.post(thread+"/archive", url.Values{"csrf_token": {s.csrf(thread)}}); resp.StatusCode >= 400 {
			t.Fatalf("archive: %d", resp.StatusCode)
		}
		if code, page := s.get(thread); code != http.StatusOK || !strings.Contains(page, "<title>Hello") {
			t.Errorf("the archived thread for its member: %d, its title %v - want 200, the subject", code, strings.Contains(page, "<title>Hello"))
		}
		if resp := s.post(thread+"/reply", url.Values{"csrf_token": {s.csrf(thread)}, "body": {"<p>Still here</p>"}}); resp.StatusCode >= 400 {
			t.Errorf("a reply in the archived thread: %d, want it sent", resp.StatusCode)
		}
		stranger := s.visitor()
		stranger.signInAs("stranger", memberPassword)
		if resp := stranger.post(thread+"/reply", url.Values{"csrf_token": {stranger.csrf("/me/messages")}, "body": {"<p>Let me in</p>"}}); resp.StatusCode != http.StatusForbidden {
			t.Errorf("a stranger's reply: %d, want 403", resp.StatusCode)
		}

		// Comments: an unpublished article and a published one.
		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		for title, status := range map[string]string{"Draft": "0", "Public": "1"} {
			if _, ok := s.submit("/create/article", "/create/article", url.Values{"title": {title}, body[1]: {"<p>Text</p>"}, "status": {status}}); !ok {
				t.Fatalf("create %s", title)
			}
		}
		draft, public := id(`SELECT id FROM nodes WHERE title = 'Draft'`), id(`SELECT id FROM nodes WHERE title = 'Public'`)
		commenter := s.visitor()
		commenter.signInAs("member", memberPassword)
		comment := func(nid int, parent string) int {
			t.Helper()
			v := url.Values{"csrf_token": {commenter.csrf(fmt.Sprintf("/node/%d", public))}, "nid": {fmt.Sprint(nid)}, "content": {"A comment."}}
			if parent != "" {
				v.Set("parent_id", parent)
			}
			return commenter.post("/comment/create", v).StatusCode
		}
		if code := comment(draft, ""); code != http.StatusNotFound {
			t.Errorf("a comment on an unpublished node: %d, want 404", code)
		}
		if code := comment(public, ""); code >= 400 {
			t.Fatalf("a comment on a published node: %d", code)
		}
		if _, ok := s.submit("/create/article", "/create/article", url.Values{"title": {"Other"}, body[1]: {"<p>Text</p>"}, "status": {"1"}}); !ok {
			t.Fatal("create Other")
		}
		other := id(`SELECT id FROM nodes WHERE title = 'Other'`)
		if code := comment(other, fmt.Sprint(id(`SELECT MAX(id) FROM comments`))); code != http.StatusBadRequest {
			t.Errorf("a reply to another node's comment: %d, want 400", code)
		}

		// The member's profile edit: another user's name; then an email in
		// capitals; then without the current password.
		edit := func(set url.Values) {
			t.Helper()
			v := url.Values{"csrf_token": {commenter.csrf("/me/edit")}, "username": {"member"}, "email": {"member@example.com"}, "current_password": {memberPassword}}
			for k, vs := range set {
				v[k] = vs
			}
			if resp := commenter.postMultipart("/me/edit", v); resp.StatusCode >= 500 {
				t.Fatalf("POST /me/edit %v: %d", set, resp.StatusCode)
			}
		}
		edit(url.Values{"username": {"stranger"}})
		if n := id(`SELECT COUNT(*) FROM users WHERE username = 'member'`); n != 1 {
			t.Error("the member took another user's name")
		}
		edit(url.Values{"email": {"Member.New@Example.com"}})
		if n := id(`SELECT COUNT(*) FROM users WHERE email = 'member.new@example.com'`); n != 1 {
			t.Error("the edited email isn't stored lowercase")
		}

		// The current password guards only the e-mail and the password: the
		// signature saves without it, a new e-mail without it is refused.
		edit(url.Values{"email": {"member.new@example.com"}, "current_password": {""}, "signature": {"<p>Ahoy</p>"}})
		if n := id(`SELECT COUNT(*) FROM users WHERE username = 'member' AND signature LIKE '%Ahoy%'`); n != 1 {
			t.Error("the signature wasn't saved without the current password")
		}
		edit(url.Values{"email": {"member.other@example.com"}, "current_password": {""}})
		if n := id(`SELECT COUNT(*) FROM users WHERE email = 'member.other@example.com'`); n != 0 {
			t.Error("the e-mail changed without the current password")
		}
	})
}
