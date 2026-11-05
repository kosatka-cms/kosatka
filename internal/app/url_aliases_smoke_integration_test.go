//go:build dbintegration

package app

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestURLAliasesPages - the addresses page finds a node by a part of its
// address and shows that it follows its type's pattern; the patterns page
// saves the types' patterns together, and with one invalid saves none and
// shows the error at that type.
func TestURLAliasesPages(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()

		_, create := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(create)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		if resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {"Alias probe"},
			body[1]: {"<p>Body</p>"}, "status": {"1"}, "path_auto_shown": {"1"}, "path_auto": {"1"}}); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d\n%s", resp.StatusCode, s.log.String())
		}
		code, page := s.get("/admin/config/url-aliases?q=alias-pro")
		if code != http.StatusOK || !strings.Contains(page, "/article/alias-probe") || !strings.Contains(page, "by the pattern") {
			t.Errorf("the addresses page: %d, finds the article: %v", code, strings.Contains(page, "/article/alias-probe"))
		}
		if _, page := s.get("/admin/config/url-aliases?q=no-such-address"); strings.Contains(page, "/article/alias-probe") {
			t.Error("a filter matching nothing lists the article")
		}

		var article, pageType int
		if err := db.DB.QueryRow("SELECT id FROM content_types WHERE machine_name = 'article'").Scan(&article); err != nil {
			t.Fatal(err)
		}
		if err := db.DB.QueryRow("SELECT id FROM content_types WHERE machine_name = 'page'").Scan(&pageType); err != nil {
			t.Fatal(err)
		}
		pattern := func(id int) string {
			t.Helper()
			var p string
			if err := db.DB.QueryRow(db.Dialect.Rebind("SELECT path_pattern FROM content_types WHERE id = ?"), id).Scan(&p); err != nil {
				t.Fatal(err)
			}
			return p
		}
		const patterns = "/admin/config/url-aliases/patterns"
		_, form := s.get(patterns)
		f, ok := formPostingTo(form, patterns, patterns)
		if !ok {
			t.Fatal("no patterns form")
		}
		f.values.Set("csrf_token", s.csrf(patterns))
		f.values.Set("pattern_"+strconv.Itoa(article), "news/[node:title]")
		f.values.Set("pattern_"+strconv.Itoa(pageType), "[node:nope]")
		resp := s.post(f.action, f.values)
		if resp.StatusCode != http.StatusOK || pattern(article) != "" || pattern(pageType) != "" {
			t.Errorf("an invalid pattern: %d, saved %q %q", resp.StatusCode, pattern(article), pattern(pageType))
		}
		f.values.Set("pattern_"+strconv.Itoa(pageType), "about/[node:title]")
		if resp := s.post(f.action, f.values); resp.StatusCode != http.StatusSeeOther || pattern(article) != "news/[node:title]" || pattern(pageType) != "about/[node:title]" {
			t.Errorf("valid patterns: %d → %q, saved %q %q", resp.StatusCode, resp.Header.Get("Location"), pattern(article), pattern(pageType))
		}
		s.checkLog()
	})
}

// TestURLRebuild - a type's pattern changed, the rebuild tab shows what
// changes, then rebuilds the automatic addresses (the old ones redirect)
// and leaves the one set by hand; asked to, rebuilds that one too.
func TestURLRebuild(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()

		_, create := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(create)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		createArticle := func(values url.Values) {
			t.Helper()
			values.Set("csrf_token", s.csrf("/create/article"))
			values.Set(body[1], "<p>Body</p>")
			values.Set("status", "1")
			values.Set("path_auto_shown", "1")
			if resp := s.postMultipart("/create/article", values); resp.StatusCode != http.StatusFound {
				t.Fatalf("create %q: %d\n%s", values.Get("title"), resp.StatusCode, s.log.String())
			}
		}
		createArticle(url.Values{"title": {"Rebuild probe"}, "path_auto": {"1"}})
		createArticle(url.Values{"title": {"Manual probe"}, "slug": {"my/manual-probe"}})
		slug := func(title string) string {
			t.Helper()
			var v string
			if err := db.DB.QueryRow(db.Dialect.Rebind("SELECT slug FROM nodes WHERE title = ?"), title).Scan(&v); err != nil {
				t.Fatal(err)
			}
			return v
		}
		if _, err := db.DB.Exec("UPDATE content_types SET path_pattern = 'posts/[node:title]' WHERE machine_name = 'article'"); err != nil {
			t.Fatal(err)
		}

		const rebuild = "/admin/config/url-aliases/rebuild"
		if code, page := s.get(rebuild + "?type=article&preview=1"); code != http.StatusOK || !strings.Contains(page, "/article/rebuild-probe") || !strings.Contains(page, "/posts/rebuild-probe") || strings.Contains(page, "manual-probe") {
			t.Errorf("the preview: %d", code)
		}
		if slug("Rebuild probe") != "article/rebuild-probe" {
			t.Error("the preview changed an address")
		}
		resp := s.post(rebuild, url.Values{"csrf_token": {s.csrf(rebuild)}, "type": {"article"}})
		if resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "done=1") {
			t.Fatalf("rebuild: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
		}
		if got := slug("Rebuild probe"); got != "posts/rebuild-probe" {
			t.Errorf("rebuilt: %q", got)
		}
		if got := slug("Manual probe"); got != "my/manual-probe" {
			t.Errorf("an address by hand moved: %q", got)
		}
		old, err := s.client.Get(s.base + "/article/rebuild-probe")
		if err != nil {
			t.Fatal(err)
		}
		_ = old.Body.Close()
		if old.StatusCode != http.StatusMovedPermanently || old.Header.Get("Location") != "/posts/rebuild-probe" {
			t.Errorf("the old address: %d → %q", old.StatusCode, old.Header.Get("Location"))
		}

		s.post(rebuild, url.Values{"csrf_token": {s.csrf(rebuild)}, "type": {"article"}, "manual": {"1"}})
		if got := slug("Manual probe"); got != "posts/manual-probe" {
			t.Errorf("with the addresses by hand: %q", got)
		}
		s.checkLog()
	})
}
