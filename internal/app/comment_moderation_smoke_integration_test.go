//go:build dbintegration

package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestCommentModeration - a new author's first comment waits for
// approval: its author sees it marked, a visitor neither in the list nor
// by its address; the admin's list opens on the waiting ones with why;
// approved, it is public and its author's next comment is published at
// once. Another newcomer's comment marked as spam with blocking: hidden,
// the account blocked. "Every comment" holds even a known author's,
// "no check" none.
func TestCommentModeration(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, true)
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
			"readers", `{"access content": true}`); err != nil {
			t.Fatal(err)
		}
		readers := one(`SELECT id FROM roles WHERE name = 'readers'`)
		member := func(name string) (*site, int) {
			t.Helper()
			if _, ok := s.submit("/admin/people/create", "/admin/people/create", url.Values{"password_mode": {"set"}, "username": {name}, "email": {name + "@example.com"},
				"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {fmt.Sprint(readers)}}); !ok {
				t.Fatalf("create %s", name)
			}
			v := s.visitor()
			v.signInAs(name, memberPassword)
			return v, one(`SELECT id FROM users WHERE username = ?`, name)
		}
		nid := one(`SELECT MIN(id) FROM nodes WHERE type = 'article' AND status = 1`)
		var slug string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT slug FROM nodes WHERE id = ?`), nid).Scan(&slug); err != nil {
			t.Fatal(err)
		}
		send := func(who *site, text string) int {
			t.Helper()
			// From an address of its own: the posting rate limit (by
			// address) would refuse the test's many comments.
			code, _, _ := who.postRead("/comment/create", url.Values{"csrf_token": {who.csrf("/" + slug)}, "nid": {fmt.Sprint(nid)}, "parent_id": {"0"}, "content": {"<p>" + text + "</p>"}})
			return code
		}
		post := func(who *site, text string) (id int, status int, reason string) {
			t.Helper()
			if code := send(who, text); code != http.StatusFound {
				t.Fatalf("posting %q: %d\n%s", text, code, s.log.String())
			}
			if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id, status, hold_reason FROM comments WHERE content = ?`), "<p>"+text+"</p>").Scan(&id, &status, &reason); err != nil {
				t.Fatalf("the comment %q: %v", text, err)
			}
			return id, status, reason
		}
		list := func(who *site) string {
			t.Helper()
			_, body := who.get(fmt.Sprintf("/api/comments/%d", nid))
			return body
		}

		// A newcomer's first comment waits.
		newcomer, _ := member("newcomer")
		held, status, reason := post(newcomer, "Will the lecture be recorded?")
		if status != 0 || reason != "new_author" {
			t.Fatalf("the first comment: status %d, %q", status, reason)
		}
		if body := list(newcomer); !strings.Contains(body, "Will the lecture be recorded?") || !strings.Contains(body, "comment-held") {
			t.Error("its author doesn't see it marked")
		}
		if body := list(s.visitor()); strings.Contains(body, "Will the lecture be recorded?") {
			t.Error("a visitor sees the waiting comment")
		}
		if code, _ := s.visitor().get(fmt.Sprintf("/comment/%d", held)); code != http.StatusNotFound {
			t.Errorf("its address for a visitor: %d", code)
		}
		_, admin := s.get("/admin/content/comment")
		if !strings.Contains(admin, "Will the lecture be recorded?") || !strings.Contains(admin, "new author") || !strings.Contains(admin, `aria-current="page">Waiting for approval · 1`) {
			t.Fatal("the admin's list doesn't open on the waiting comment")
		}

		// Approved: public; the author's next one at once.
		approve := fmt.Sprintf("/admin/content/comment/%d/approve", held)
		if resp := s.post(approve, url.Values{"csrf_token": {s.csrf("/admin/content/comment")}, "back": {"/admin/content/comment?status=pending"}}); resp.Header.Get("Location") != "/admin/content/comment?status=pending" {
			t.Fatalf("approve: %d %s", resp.StatusCode, resp.Header.Get("Location"))
		}
		if body := list(s.visitor()); !strings.Contains(body, "Will the lecture be recorded?") {
			t.Error("the approved comment isn't public")
		}
		// Its address (the latest comments block's link) leads a visitor to
		// it on its node's page.
		v := s.visitor()
		if resp, err := v.client.Get(v.base + fmt.Sprintf("/comment/%d", held)); err != nil {
			t.Fatal(err)
		} else if _ = resp.Body.Close(); resp.StatusCode != http.StatusFound || !strings.HasSuffix(resp.Header.Get("Location"), fmt.Sprintf("#comment-%d", held)) {
			t.Errorf("the approved comment's address for a visitor: %d %q", resp.StatusCode, resp.Header.Get("Location"))
		}
		if _, status, _ := post(newcomer, "Thanks, found it."); status != 1 {
			t.Errorf("the known author's next comment: status %d", status)
		}

		// Spam, the author blocked.
		drifter, drifterID := member("drifter")
		spam, _, _ := post(drifter, "Cheap watches here")
		req, err := http.NewRequest(http.MethodPost, s.base+"/api/admin/bulk/comments", strings.NewReader(fmt.Sprintf(`{"action":"spam_block","ids":[%d]}`, spam)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", s.csrf("/admin/content/comment"))
		resp, err := s.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("spam_block: %v %v", resp, err)
		}
		_ = resp.Body.Close()
		if st := one(`SELECT status FROM comments WHERE id = ?`, spam); st != 2 {
			t.Errorf("the spam comment: status %d", st)
		}
		if st := one(`SELECT status FROM users WHERE id = ?`, drifterID); st != 0 {
			t.Errorf("the spammer: status %d", st)
		}
		if _, page := s.get("/admin/content/comment?status=spam"); !strings.Contains(page, "Cheap watches here") || !strings.Contains(page, "Not spam") {
			t.Error("the spam tab")
		}

		// The setting: every comment, then none.
		saveCommentSettings(t, s, url.Values{"new_author_checks": {"-1"}})
		if _, status, _ := post(newcomer, "One more question."); status != 0 {
			t.Errorf("every comment: status %d", status)
		}
		saveCommentSettings(t, s, url.Values{"new_author_checks": {"0"}})
		if _, status, _ := post(newcomer, "And the last one."); status != 1 {
			t.Errorf("no check: status %d", status)
		}

		// Links over the limit wait; a stop word refuses; the same text
		// again is refused.
		if _, status, reason := post(newcomer, "Maps: https://a.example.com https://b.example.com https://c.example.com"); status != 0 || reason != "links:3" {
			t.Errorf("3 links: status %d, %q", status, reason)
		}
		saveCommentSettings(t, s, url.Values{"stop_words": {"cheap watches"}, "stop_words_action": {"reject"}})
		if code := send(newcomer, "Cheap  WATCHES, again"); code != http.StatusBadRequest {
			t.Errorf("a stop word: %d", code)
		}
		if code := send(newcomer, "And the last one."); code != http.StatusBadRequest {
			t.Errorf("the same text again: %d", code)
		}
		if _, admin := s.get("/admin/content/comment?status=pending"); !strings.Contains(admin, "3 links") {
			t.Error("the waiting comment's reason")
		}

		// Spam kept 7 days, then deleted.
		saveCommentSettings(t, s, url.Values{"spam_keep_days": {"7"}})
		if _, err := db.DB.Exec(db.Dialect.Rebind(`UPDATE comments SET updated_at = ? WHERE id = ?`), time.Now().AddDate(0, 0, -8).UTC(), spam); err != nil {
			t.Fatal(err)
		}
		comments := database.NewCommentRepository(database.ClientFor(db.DB))
		job := service.SpamPurgeJob(database.NewCommentSettingsRepository(db.DB), comments, service.NewCommentsService(comments, slog.Default()), time.Now)
		if err := job.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if n := one(`SELECT COUNT(*) FROM comments WHERE id = ?`, spam); n != 0 {
			t.Error("the old spam wasn't deleted")
		}
		s.checkLog()
	})
}

// saveCommentSettings saves the comment settings as the page has them,
// with set's values.
func saveCommentSettings(t *testing.T, s *site, set url.Values) {
	t.Helper()
	const page = "/admin/config/comments"
	_, html := s.get(page)
	form := settingsFormValues(html)
	if m := regexp.MustCompile(`(?s)<textarea[^>]*name="stop_words"[^>]*>(.*?)</textarea>`).FindStringSubmatch(html); m != nil {
		form.Set("stop_words", m[1])
	}
	for k, v := range set {
		form[k] = v
	}
	form.Set("csrf_token", s.csrf(page))
	if resp := s.post(page, form); !strings.Contains(resp.Header.Get("Location"), "saved=1") {
		t.Fatalf("comment settings %v: %s", set, resp.Header.Get("Location"))
	}
}

// TestBotTrap - with the bot trap on, a comment sent without the time its
// form was shown, or sooner than the set seconds, is refused; sent from
// the form a moment later, it is posted. The registration form carries
// the trap too.
func TestBotTrap(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, true)
		s.signIn()
		saveCommentSettings(t, s, url.Values{"bot_trap": {"1"}, "bot_min_seconds": {"2"}, "new_author_checks": {"0"}})
		var nid int
		var slug string
		if err := db.DB.QueryRow(`SELECT id, slug FROM nodes WHERE type = 'article' AND status = 1 ORDER BY id LIMIT 1`).Scan(&nid, &slug); err != nil {
			t.Fatal(err)
		}
		send := func(text, shown string) int {
			t.Helper()
			v := url.Values{"csrf_token": {s.csrf("/" + slug)}, "nid": {fmt.Sprint(nid)}, "parent_id": {"0"}, "content": {"<p>" + text + "</p>"}}
			if shown != "" {
				v.Set("form_shown", shown)
			}
			code, _, _ := s.postRead("/comment/create", v)
			return code
		}
		if code := send("Without the form.", ""); code != http.StatusBadRequest {
			t.Errorf("without the time shown: %d", code)
		}
		_, page := s.get("/" + slug)
		m := regexp.MustCompile(`name="form_shown" value="([^"]+)"`).FindStringSubmatch(page)
		if m == nil || !strings.Contains(page, `name="address_line_2"`) {
			t.Fatal("the comment form has no bot trap")
		}
		if code := send("Too soon.", m[1]); code != http.StatusBadRequest {
			t.Errorf("sent at once: %d", code)
		}
		time.Sleep(2100 * time.Millisecond)
		if code := send("A person's comment.", m[1]); code != http.StatusFound {
			t.Errorf("sent a moment later: %d\n%s", code, s.log.String())
		}
		if _, page := s.visitor().get("/user/register"); strings.Contains(page, `action="/user/register"`) && !strings.Contains(page, `name="form_shown"`) {
			t.Error("the registration form has no bot trap")
		}
		s.checkLog()
	})
}
