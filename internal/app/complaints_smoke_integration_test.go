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

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestComplaintsModule - the complaints module end to end (off by
// default, turned on on the Modules page):
//   - a signed-in reader sees "Report this content" under an article of a
//     type the settings take and the complaint's window; a guest neither,
//     and their complaint is refused;
//   - the reader's complaint is taken (again - no change) and mailed to
//     the site; the queue shows it to the administrator, not to the
//     reader;
//   - the administrator's "Unpublish" unpublishes the article and takes
//     it out of the queue.
func TestComplaintsModule(t *testing.T) {
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
		_, modules := s.get("/admin/config/modules")
		form := settingsFormValues(modules)
		form.Set("module_complaints", "1")
		form.Set("csrf_token", s.csrf("/admin/config/modules"))
		if resp := s.post("/admin/config/modules", form); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("modules: %d", resp.StatusCode)
		}
		const settings = "/admin/config/complaints"
		if resp := s.post(settings, url.Values{"csrf_token": {s.csrf(settings)}, "comments": {"1"}, "node_types": {"article"}, "hide_after": {"3"}}); !strings.Contains(resp.Header.Get("Location"), "saved=1") {
			t.Fatalf("saving the settings: %s\n%s", resp.Header.Get("Location"), s.log.String())
		}

		role := fmt.Sprint(count(`SELECT id FROM roles WHERE name = ?`, entity.RoleUser))
		if _, ok := s.submit("/admin/users/create", "/admin/users/create", url.Values{"password_mode": {"set"}, "username": {"reader"}, "email": {"reader@example.com"},
			"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {role}}); !ok {
			t.Fatal("create reader")
		}
		_, create := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(create)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		if _, ok := s.submit("/create/article", "/create/article", url.Values{"title": {"Harbour news"}, body[1]: {"<p>Text</p>"}, "status": {"1"}}); !ok {
			t.Fatal("create the article")
		}
		article := count(`SELECT id FROM nodes WHERE title = 'Harbour news'`)
		var slug string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT slug FROM nodes WHERE id = ?`), article).Scan(&slug); err != nil {
			t.Fatal(err)
		}

		complain := func(who *site, reason string) (int, string) {
			t.Helper()
			req, err := http.NewRequest(http.MethodPost, who.base+"/api/complaints",
				strings.NewReader(fmt.Sprintf(`{"kind":"node","id":%d,"reason":%q,"note":"Copied"}`, article, reason)))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Token", who.csrf("/"+slug))
			resp, err := who.client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			b, _ := io.ReadAll(resp.Body)
			return resp.StatusCode, string(b)
		}

		guest := s.visitor()
		if _, page := guest.get("/" + slug); strings.Contains(page, "data-complain") || strings.Contains(page, "complaint-dialog") {
			t.Error("a guest is offered to complain")
		}
		if code, _ := complain(guest, "Insults"); code != http.StatusUnauthorized {
			t.Errorf("a guest's complaint: %d", code)
		}

		r := s.visitor()
		r.signInAs("reader", memberPassword)
		if _, page := r.get("/" + slug); !strings.Contains(page, fmt.Sprintf(`data-complain="node" data-complain-id="%d"`, article)) || !strings.Contains(page, `id="complaint-dialog"`) {
			t.Fatal("the reader has no Report link or window")
		}
		if code, resp := complain(r, "Boring"); code != http.StatusBadRequest {
			t.Errorf("a reason not offered: %d %s", code, resp)
		}
		for range 2 {
			if code, resp := complain(r, "Insults"); code != http.StatusOK || !strings.Contains(resp, `"ok":true`) {
				t.Fatalf("the reader's complaint: %d %s\n%s", code, resp, s.log.String())
			}
		}
		if n := count(`SELECT COUNT(*) FROM complaints`); n != 1 {
			t.Errorf("complaints: %d, want 1", n)
		}
		if n := count(`SELECT COUNT(*) FROM mail_log WHERE mail_key = 'complaints.new_complaint'`); n != 1 {
			t.Errorf("the site's e-mails of the complaint: %d, want 1", n)
		}

		const queue = "/admin/content/complaints"
		if code, _ := r.get(queue); code != http.StatusForbidden {
			t.Errorf("the reader's queue: %d", code)
		}
		code, page := s.get(queue)
		if code != http.StatusOK || !strings.Contains(page, "Harbour news") || !strings.Contains(page, "Insults · 1") {
			t.Fatalf("the queue: %d\n%s", code, page)
		}
		target := count(`SELECT id FROM complaint_targets WHERE object_id = ?`, article)
		decide := fmt.Sprintf("%s/%d/unpublished", queue, target)
		if resp := s.post(decide, url.Values{"csrf_token": {s.csrf(queue)}}); !strings.Contains(resp.Header.Get("Location"), "done=unpublished") {
			t.Fatalf("unpublish: %d %s\n%s", resp.StatusCode, resp.Header.Get("Location"), s.log.String())
		}
		if st := count(`SELECT status FROM nodes WHERE id = ?`, article); st != 0 {
			t.Errorf("the unpublished article's status: %d", st)
		}
		if _, page := s.get(queue); !strings.Contains(page, "No new complaints") {
			t.Error("the decided complaint is still in the queue")
		}
		s.checkLog()
	})
}
