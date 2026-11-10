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

// TestSubscriptionsModule - the subscriptions module end to end (on by
// default on a new site):
//   - the author of an article follows its comments; a reader follows them
//     with the button of its page, not those of a draft;
//   - a comment tells the followers, a reply its comment's author (as a
//     reply, not a comment too), the commenter not; the reader's
//     Notifications tab shows the comments as one item, the user menu
//     in the header (its toggle and item) and the tab the unread count;
//   - opening a notification reads it (with its group) and goes to the
//     comment; another user's notification is not found, a guest signs in
//     first;
//   - an e-mail's unsubscribe link asks a guest, then unsubscribes: the
//     next comment doesn't tell the reader, nor subscribes them again.
func TestSubscriptionsModule(t *testing.T) {
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
		for _, name := range []string{"reader", "writer"} {
			if _, ok := s.submit("/admin/users/create", "/admin/users/create", url.Values{"password_mode": {"set"}, "username": {name}, "email": {name + "@example.com"},
				"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {role}}); !ok {
				t.Fatalf("create %s", name)
			}
		}
		// The new users' comments published at once (no new authors' check).
		saveCommentSettings(t, s, url.Values{"new_author_checks": {"0"}})
		admin, reader, writer := count(`SELECT id FROM users WHERE username = 'admin'`),
			count(`SELECT id FROM users WHERE username = 'reader'`), count(`SELECT id FROM users WHERE username = 'writer'`)

		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		for title, status := range map[string]string{"Harbour news": "1", "Harbour draft": "0"} {
			if _, ok := s.submit("/create/article", "/create/article", url.Values{"title": {title}, body[1]: {"<p>Text</p>"}, "status": {status}}); !ok {
				t.Fatalf("create %s", title)
			}
		}
		article, draft := count(`SELECT id FROM nodes WHERE title = 'Harbour news'`), count(`SELECT id FROM nodes WHERE title = 'Harbour draft'`)
		var slug string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT slug FROM nodes WHERE id = ?`), article).Scan(&slug); err != nil {
			t.Fatal(err)
		}
		page := "/" + slug
		subscribed := func(user, node int) int {
			t.Helper()
			return count(`SELECT COUNT(*) FROM subscriptions WHERE user_id = ? AND node_id = ? AND muted = ?`, user, node, false)
		}
		if subscribed(admin, article) != 1 {
			t.Error("the author doesn't follow the article's comments")
		}

		r := s.visitor()
		r.signInAs("reader", memberPassword)
		follow := fmt.Sprintf("/subscriptions/follow/node/%d", article)
		if _, html := r.get(page); !strings.Contains(html, `action="`+follow+`"`) {
			t.Fatal("the article has no subscription button")
		}
		resp := r.post(follow, url.Values{"csrf_token": {r.csrf(page)}, "back": {page + "#comments"}})
		if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != page+"#comments" || subscribed(reader, article) != 1 {
			t.Fatalf("following: %d → %q, subscribed %d", resp.StatusCode, resp.Header.Get("Location"), subscribed(reader, article))
		}
		if _, html := r.get(page); !strings.Contains(html, `name="action" value="unsubscribe"`) {
			t.Error("followed, the button doesn't offer to unsubscribe")
		}
		if resp := r.post(fmt.Sprintf("/subscriptions/follow/node/%d", draft), url.Values{"csrf_token": {r.csrf(page)}}); resp.StatusCode != http.StatusNotFound {
			t.Errorf("following a draft: %d, want 404", resp.StatusCode)
		}

		w := s.visitor()
		w.signInAs("writer", memberPassword)
		said := 0
		comment := func(who *site, parent int) int {
			t.Helper()
			said++ // each text its own: the same one again is refused
			v := url.Values{"csrf_token": {who.csrf(page)}, "nid": {fmt.Sprint(article)}, "content": {fmt.Sprintf("Fair winds, %d.", said)}}
			if parent > 0 {
				v.Set("parent_id", fmt.Sprint(parent))
			}
			if resp := who.post("/comment/create", v); resp.StatusCode >= 400 {
				t.Fatalf("comment: %d\n%s", resp.StatusCode, s.log.String())
			}
			return count(`SELECT MAX(id) FROM comments`)
		}
		first := comment(w, 0)
		comment(s, first)
		kinds := func(user int) string {
			t.Helper()
			rows, err := db.DB.Query(db.Dialect.Rebind(`SELECT kind FROM subscription_notifications WHERE user_id = ? ORDER BY id`), user)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rows.Close() }()
			var out []string
			for rows.Next() {
				var k string
				if err := rows.Scan(&k); err != nil {
					t.Fatal(err)
				}
				out = append(out, k)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			return strings.Join(out, ",")
		}
		for user, want := range map[int]string{reader: "comment,comment", writer: "reply", admin: "comment"} {
			if got := kinds(user); got != want {
				t.Errorf("user %d's notifications: %q, want %q", user, got, want)
			}
		}

		code, tab := r.get("/me/notifications")
		if code != http.StatusOK || !strings.Contains(tab, "<strong>2 new comments</strong> on \"Harbour news\"") ||
			!strings.Contains(tab, `<span class="badge">2</span>`) || !strings.Contains(tab, "/static/modules/subscriptions/") {
			t.Errorf("the reader's tab: %d, the group %v, the tab's count %v, the module's CSS %v", code,
				strings.Contains(tab, "2 new comments"), strings.Contains(tab, `<span class="badge">2</span>`),
				strings.Contains(tab, "/static/modules/subscriptions/"))
		}
		// The theme's header: the user menu, its count and the item.
		if _, html := r.get("/"); !strings.Contains(html, `id="user-menu-toggle"`) ||
			!strings.Contains(html, `href="/me/notifications" class="dropdown-item dropdown-item-notifications" data-mobile-user-menu`) ||
			strings.Count(html, `<span class="badge bg-danger ms-1">2</span>`) != 2 {
			t.Errorf("the header's user menu: the toggle %v, the item %v, the counts %d", strings.Contains(html, `id="user-menu-toggle"`),
				strings.Contains(html, `href="/me/notifications"`), strings.Count(html, `<span class="badge bg-danger ms-1">2</span>`))
		}
		if code, _ := s.get("/" + entity.UserPathPrefix + "/reader/notifications"); code == http.StatusOK {
			t.Error("an administrator opened another user's notifications")
		}

		latest := count(`SELECT MAX(id) FROM subscription_notifications WHERE user_id = ?`, reader)
		open := fmt.Sprintf("/notifications/%d", latest)
		if code, _ := w.get(open); code != http.StatusNotFound {
			t.Errorf("another user opens the reader's notification: %d, want 404", code)
		}
		resp, err := s.visitor().client.Get(s.base + open)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusFound || !strings.HasPrefix(resp.Header.Get("Location"), "/login?destination=") {
			t.Errorf("a guest opens a notification: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
		}
		resp, err = r.client.Get(s.base + open)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if want := fmt.Sprintf("%s#comment-%d", page, count(`SELECT MAX(id) FROM comments`)); resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != want {
			t.Errorf("opening: %d → %q, want %q", resp.StatusCode, resp.Header.Get("Location"), want)
		}
		if n := count(`SELECT COUNT(*) FROM subscription_notifications WHERE user_id = ? AND read_at IS NULL`, reader); n != 0 {
			t.Errorf("after opening the group: %d unread", n)
		}

		var token string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT token FROM subscriptions WHERE user_id = ? AND node_id = ?`), reader, article).Scan(&token); err != nil {
			t.Fatal(err)
		}
		guest := s.visitor()
		unsubscribe := "/subscriptions/unsubscribe/" + token
		if code, html := guest.get(unsubscribe); code != http.StatusOK || !strings.Contains(html, "\"Harbour news\"") || !strings.Contains(html, `<form method="post">`) {
			t.Errorf("the unsubscribe link: %d", code)
		}
		if subscribed(reader, article) != 1 {
			t.Error("opening the unsubscribe link unsubscribed")
		}
		if resp := guest.post(unsubscribe, url.Values{"csrf_token": {guest.csrf(unsubscribe)}}); resp.StatusCode != http.StatusOK || subscribed(reader, article) != 0 {
			t.Errorf("unsubscribing by the link: %d, still subscribed %d", resp.StatusCode, subscribed(reader, article))
		}
		if code, _ := guest.get("/subscriptions/unsubscribe/missing"); code != http.StatusNotFound {
			t.Errorf("an unknown token: %d, want 404", code)
		}
		comment(r, 0)
		comment(w, 0)
		if got := kinds(reader); got != "comment,comment" {
			t.Errorf("unsubscribed, the reader was told: %q", got)
		}
		if subscribed(reader, article) != 0 {
			t.Error("commenting subscribed the reader again after they unsubscribed")
		}
		s.checkLog()
	})
}
