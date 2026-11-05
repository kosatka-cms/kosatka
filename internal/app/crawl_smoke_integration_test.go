//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestCrawlSite - a site with content of every kind (made through its own
// forms: articles, a page, a term, a poll, a comment, a required profile
// field, a member) is crawled by following its links,
// from the home page and every GET route, as a guest, as a member and as
// the administrator: no page answers with a server error, and the server
// logs no error.
func TestCrawlSite(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		routes := routerGETPaths(t)
		s.signIn()
		seedSite(t, s, db)

		// Every published node's page is crawled from (a page may be
		// linked from nowhere yet).
		rows, err := db.DB.Query("SELECT slug FROM nodes WHERE status = 1")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		start := append([]string{"/"}, routes...)
		for rows.Next() {
			var slug string
			if err := rows.Scan(&slug); err != nil {
				t.Fatal(err)
			}
			start = append(start, "/"+slug)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if n := len(start) - 1 - len(routes); n < 4 { // two articles, a page, a poll
			t.Errorf("only %d published nodes seeded", n)
		}

		s.visitor().crawl("a guest", start)
		member := s.visitor()
		member.signInAs("member", memberPassword)
		member.crawl("a member", append([]string{"/me"}, start...))
		s.crawl("the administrator", append([]string{"/admin"}, start...))
		s.checkLog()
	})
}

// seedSite makes content of every kind through the site's forms, as the
// signed-in administrator.
func seedSite(t *testing.T, s *site, db testdb.DB) {
	t.Helper()
	id := func(q string) int {
		t.Helper()
		var n int
		if err := db.DB.QueryRow(q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	bodyField := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`)
	for _, c := range []struct{ typ, title string }{{"article", "First article"}, {"article", "Second article"}, {"page", "About"}} {
		path := "/create/" + c.typ
		_, page := s.get(path)
		body := bodyField.FindStringSubmatch(page)
		if body == nil {
			t.Fatalf("no body field on %s", path)
		}
		if _, ok := s.submit(path, path, url.Values{"title": {c.title}, body[1]: {"<p>Some text.</p>"}, "status": {"1"}}); !ok {
			t.Fatalf("create %s", c.typ)
		}
	}
	article := id("SELECT id FROM nodes WHERE title = 'First article'")

	if _, ok := s.submit("/admin/config/taxonomy", "/admin/config/taxonomy",
		url.Values{"name": {"Tags"}, "slug": {"tags"}, "term_path_prefix": {"/tags/"}, "public_term_pages": {"1"}}); !ok {
		t.Fatal("create vocabulary")
	}
	tags := fmt.Sprintf("/admin/config/taxonomy/%d/terms", id("SELECT id FROM taxonomy_vocabulary WHERE slug = 'tags'"))
	if _, ok := s.submit(tags, tags, url.Values{"name": {"Cooking"}, "slug": {"cooking"}}); !ok {
		t.Fatal("create term")
	}

	// A required profile field: the administrator's "add user" and the
	// sign-up ask it.
	if _, ok := s.submit("/admin/people/fields", "/admin/people/fields",
		url.Values{"label": {"City"}, "machine_name": {"city"}, "required": {"1"}}); !ok {
		t.Fatal("create profile field")
	}
	city := id("SELECT id FROM user_fields WHERE machine_name = 'city'")
	cityInput := fmt.Sprintf("user_field_%d", city)
	if _, ok := s.submit("/admin/people/create", "/admin/people/create", url.Values{"password_mode": {"set"}, "username": {"member"}, "email": {"member@example.com"},
		"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, cityInput: {"Paris"},
		"role_id": {fmt.Sprint(id("SELECT id FROM roles WHERE name = '" + entity.RoleUser + "'"))}}); !ok {
		t.Fatal("create member")
	}
	// A visitor signs up with it (the account may wait for approval or a
	// confirmed address: it exists either way).
	guest := s.visitor()
	if resp := guest.post("/user/register", url.Values{"csrf_token": {guest.csrf("/user/register")}, "username": {"newbie"},
		"email": {"newbie@example.com"}, "password": {memberPassword}, "password_confirm": {memberPassword}, cityInput: {"Lyon"}}); resp.StatusCode >= 400 {
		t.Fatalf("sign up: %d", resp.StatusCode)
	}
	for user, want := range map[string]string{"member": "Paris", "newbie": "Lyon"} {
		var got string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT v.value FROM user_field_values v JOIN users u ON u.id = v.user_id
			WHERE u.username = ? AND v.field_id = ?`), user, city).Scan(&got); err != nil || got != want {
			t.Errorf("%s's city: %q %v, want %q", user, got, err, want)
		}
	}

	// A module's content: a poll.
	if _, ok := s.submit("/create/poll", "/create/poll", url.Values{"title": {"Tea or coffee?"},
		"option_id": {"", ""}, "option_label": {"Tea", "Coffee"}, "status": {"1"}}); !ok {
		t.Fatal("create poll")
	}

	if resp := s.post("/comment/create", url.Values{"csrf_token": {s.csrf(fmt.Sprintf("/node/%d", article))},
		"nid": {fmt.Sprint(article)}, "content": {"A first comment."}}); resp.StatusCode >= 400 {
		t.Fatalf("create comment: %d", resp.StatusCode)
	}
}

const memberPassword = "member-password-1234"

// signInAs signs a user in with their password.
func (s *site) signInAs(login, password string) {
	s.t.Helper()
	resp := s.post("/login", url.Values{"login": {login}, "pass": {password}, "csrf_token": {s.csrf("/login")}})
	if resp.StatusCode != http.StatusFound || strings.Contains(resp.Header.Get("Location"), "login") {
		s.t.Fatalf("sign-in %s: %d → %q", login, resp.StatusCode, resp.Header.Get("Location"))
	}
}

// visitor - another client of the same site, signed out.
func (s *site) visitor() *site {
	v := *s
	jar, _ := cookiejar.New(nil)
	v.client = &http.Client{Jar: jar, Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &v
}

var (
	crawlLink = regexp.MustCompile(`href="(/[^"#]*)"`)
	// Not followed: leaving (logout), files, and links of other hosts.
	crawlSkip = regexp.MustCompile(`^(//|/logout|/static/|/files/|/sites/|/private/)|\.(jpe?g|png|gif|webp|svg|ico|css|js|pdf|zip)(\?|$)`)
)

// crawl GETs the start pages and every page they link to on the site,
// breadth first, and fails on a server error. A path is visited with at
// most three query strings (pagination and filters don't multiply).
// It returns the links it reached.
func (s *site) crawl(who string, start []string) map[string]bool {
	s.t.Helper()
	const limit = 1500
	seen := map[string]bool{}
	perPath := map[string]int{}
	queue := []string{}
	add := func(link string) {
		link = strings.ReplaceAll(link, "&amp;", "&")
		if seen[link] || crawlSkip.MatchString(link) {
			return
		}
		path, _, hasQuery := strings.Cut(link, "?")
		if hasQuery {
			if perPath[path] >= 3 {
				return
			}
			perPath[path]++
		}
		seen[link] = true
		queue = append(queue, link)
	}
	for _, p := range start {
		add(p)
	}
	visited := 0
	for len(queue) > 0 && visited < limit {
		link := queue[0]
		queue = queue[1:]
		visited++
		code, body := s.get(link)
		if code >= 500 {
			s.t.Errorf("GET %s as %s: %d", link, who, code)
			continue
		}
		for _, m := range crawlLink.FindAllStringSubmatch(body, -1) {
			add(m[1])
		}
	}
	if visited >= limit {
		s.t.Errorf("crawl as %s: more than %d pages (a link loop?)", who, limit)
	}
	s.t.Logf("crawled %d pages as %s", visited, who)
	return seen
}
