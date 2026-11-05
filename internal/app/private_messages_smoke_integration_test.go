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

// TestPrivateMessagesModule - the private messages module end to end:
//   - the profile's Messages tab (a module's profile tab): the recipient's
//     own tab lists the thread with the unread badge, an administrator
//     opens another user's tab, another user gets a 403, a guest too;
//   - switched off on the Modules page, the tab and the module's pages
//     answer 404, the profile has no tab;
//   - uninstalled there, its tables (pm_*) are gone; enabled again, it
//     is installed anew and works (with no conversations).
func TestPrivateMessagesModule(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		count := func(q string, args ...any) int {
			t.Helper()
			var n int
			if err := db.DB.QueryRow(db.Dialect.Rebind(q), args...).Scan(&n); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
			return n
		}
		role := fmt.Sprint(count(`SELECT id FROM roles WHERE name = ?`, entity.RoleUser))
		for _, name := range []string{"recipient", "outsider"} {
			if _, ok := s.submit("/admin/people/create", "/admin/people/create", url.Values{"password_mode": {"set"}, "username": {name}, "email": {name + "@example.com"},
				"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {role}}); !ok {
				t.Fatalf("create %s", name)
			}
		}
		recipientID := count(`SELECT id FROM users WHERE username = 'recipient'`)
		if resp := s.post("/me/messages/new", url.Values{"csrf_token": {s.csrf("/me/messages/new")}, "subject": {"Tea on Friday"},
			"body": {"<p>Shall we?</p>"}, "recipients[]": {fmt.Sprint(recipientID)}}); resp.StatusCode >= 400 {
			t.Fatalf("start a thread: %d\n%s", resp.StatusCode, s.log.String())
		}

		// A message isn't a mailing list: more recipients than allowed
		// are refused.
		many := url.Values{"csrf_token": {s.csrf("/me/messages/new")}, "subject": {"To everyone"}, "body": {"<p>Hello</p>"}}
		for id := 1; id <= 21; id++ {
			many.Add("recipients[]", fmt.Sprint(id))
		}
		if resp := s.post("/me/messages/new", many); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("21 recipients: %d", resp.StatusCode)
		}

		recipient := s.visitor()
		recipient.signInAs("recipient", memberPassword)
		code, tab := recipient.get("/me/messages")
		if code != http.StatusOK || !strings.Contains(tab, "Tea on Friday") || !strings.Contains(tab, `href="/me/messages" class="tab-item active"`) ||
			!strings.Contains(tab, `<span class="badge">1</span>`) || !strings.Contains(tab, "/static/modules/private_messages/") {
			t.Errorf("the recipient's messages tab: %d, the thread %v, the active tab %v, the badge %v, the module's CSS %v", code,
				strings.Contains(tab, "Tea on Friday"), strings.Contains(tab, `href="/me/messages" class="tab-item active"`),
				strings.Contains(tab, `<span class="badge">1</span>`), strings.Contains(tab, "/static/modules/private_messages/"))
		}
		profile := "/" + entity.UserPathPrefix + "/recipient"
		if code, page := s.get(profile + "/messages"); code != http.StatusOK || !strings.Contains(page, "Tea on Friday") {
			t.Errorf("an administrator on the recipient's tab: %d", code)
		}
		outsider := s.visitor()
		outsider.signInAs("outsider", memberPassword)
		if code, page := outsider.get(profile); code != http.StatusOK || strings.Contains(page, profile+"/messages") {
			t.Errorf("another user's view of the profile: %d, the messages tab listed %v", code, strings.Contains(page, profile+"/messages"))
		}
		if code, _ := outsider.get(profile + "/messages"); code != http.StatusForbidden {
			t.Errorf("another user on the tab: %d, want 403", code)
		}
		if code, page := s.visitor().get(profile + "/messages"); code != http.StatusForbidden || !strings.Contains(page, `href="/login?destination=`) {
			t.Errorf("a guest on the tab: %d, want 403 with a login link", code)
		}

		// Switched off: every module on but this one.
		modules := url.Values{"csrf_token": {s.csrf("/admin/config/modules")}}
		for _, m := range entity.Modules {
			if m.Name != "private_messages" {
				modules.Set("module_"+m.Name, "1")
			}
		}
		if resp := s.post("/admin/config/modules", modules); resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "saved=1") {
			t.Fatalf("switching the module off: %d %s", resp.StatusCode, resp.Header.Get("Location"))
		}
		for _, path := range []string{"/me/messages", "/me/messages/new", profile + "/messages"} {
			if code, _ := s.get(path); code != http.StatusNotFound {
				t.Errorf("the module off, %s: %d, want 404", path, code)
			}
		}
		if _, me := recipient.get("/me"); strings.Contains(me, `href="/me/messages"`) {
			t.Error("the module off, the profile links the messages")
		}

		// Uninstalled: its tables go.
		if resp := s.post("/admin/config/modules/private_messages/uninstall", url.Values{"csrf_token": {s.csrf("/admin/config/modules/private_messages/uninstall")},
			"confirm": {"private_messages"}}); resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "uninstalled=private_messages") {
			t.Fatalf("uninstall: %d %s\n%s", resp.StatusCode, resp.Header.Get("Location"), s.log.String())
		}
		for _, table := range []string{"pm_threads", "pm_thread_members", "pm_messages"} {
			if _, err := db.DB.Exec(`SELECT 1 FROM ` + table); err == nil {
				t.Errorf("uninstalled, %s is still there", table)
			}
		}

		// Enabled again.
		modules.Set("csrf_token", s.csrf("/admin/config/modules"))
		modules.Set("module_private_messages", "1")
		if resp := s.post("/admin/config/modules", modules); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("enabling the module again: %d", resp.StatusCode)
		}
		if code, page := recipient.get("/me/messages"); code != http.StatusOK || strings.Contains(page, "Tea on Friday") {
			t.Errorf("enabled again: %d, the old thread %v", code, strings.Contains(page, "Tea on Friday"))
		}
		s.checkLog()
	})
}
