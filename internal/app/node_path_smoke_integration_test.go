//go:build dbintegration

package app

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestNodePathPatterns - a type's URL pattern names its new content: an
// article created with "generate automatically" gets the pattern's slug
// and keeps it when renamed, published or a draft; an address typed
// instead takes over, the old one redirecting there; one created with a
// slug typed by hand keeps it, and so does a renamed one with
// "automatically" off. An invalid pattern isn't saved.
func TestNodePathPatterns(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()

		var typeID int
		if err := db.DB.QueryRow("SELECT id FROM content_types WHERE machine_name = 'article'").Scan(&typeID); err != nil {
			t.Fatal(err)
		}
		typePage := "/admin/config/content-types/" + strconv.Itoa(typeID)
		_, page := s.get(typePage)
		f, ok := formPostingTo(page, typePage, typePage)
		if !ok {
			t.Fatal("no content type form")
		}
		f.values.Set("path_pattern", "[node:author]/[node:title]")
		f.values.Set("csrf_token", s.csrf(typePage))
		if resp := s.post(f.action, f.values); !strings.Contains(resp.Header.Get("Location"), "error=path-pattern") {
			t.Errorf("an invalid pattern: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
		}
		if _, ok := s.submit(typePage, typePage, url.Values{"path_pattern": {"news/[node:created:Y]/[node:title]"}}); !ok {
			t.Fatal("saving the pattern")
		}

		_, create := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(create)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		slugOf := func(title string) (int, string) {
			t.Helper()
			var id int
			var slug string
			if err := db.DB.QueryRow(db.Dialect.Rebind("SELECT id, slug FROM nodes WHERE title = ?"), title).Scan(&id, &slug); err != nil {
				t.Fatalf("%s: %v", title, err)
			}
			return id, slug
		}
		createArticle := func(values url.Values) {
			t.Helper()
			values.Set("csrf_token", s.csrf("/create/article"))
			values.Set(body[1], "<p>Body</p>")
			if !values.Has("status") {
				values.Set("status", "1")
			}
			values.Set("path_auto_shown", "1")
			if resp := s.postMultipart("/create/article", values); resp.StatusCode != http.StatusFound {
				t.Fatalf("create %q: %d\n%s", values.Get("title"), resp.StatusCode, s.log.String())
			}
		}
		year := strconv.Itoa(time.Now().Year())

		createArticle(url.Values{"title": {"Привет, мир"}, "path_auto": {"1"}})
		id, slug := slugOf("Привет, мир")
		if want := "news/" + year + "/privet-mir"; slug != want {
			t.Errorf("by the pattern: %q, want %q", slug, want)
		}
		// Renamed, it keeps its address.
		edit := "/node/" + strconv.Itoa(id) + "/edit"
		if _, ok := s.submit(edit, "", url.Values{"title": {"Пока, мир"}, "path_auto": {"1"}}); !ok {
			t.Fatal("renaming the article")
		}
		if _, renamed := slugOf("Пока, мир"); renamed != slug {
			t.Errorf("renamed: %q, want %q", renamed, slug)
		}
		// An address typed instead: the old one redirects there.
		typed := "news/" + year + "/poka-mir"
		if _, ok := s.submit(edit, "", url.Values{"slug": {typed}, "path_auto": nil}); !ok {
			t.Fatal("typing the article's address")
		}
		if _, changed := slugOf("Пока, мир"); changed != typed {
			t.Errorf("typed: %q, want %q", changed, typed)
		}
		resp, err := s.client.Get(s.base + "/" + slug)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != "/"+typed {
			t.Errorf("the old address: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
		}

		// So does a draft.
		createArticle(url.Values{"title": {"Черновик"}, "path_auto": {"1"}, "status": {"0"}})
		id, draft := slugOf("Черновик")
		if _, ok := s.submit("/node/"+strconv.Itoa(id)+"/edit", "", url.Values{"title": {"Готовая статья"}, "path_auto": {"1"}}); !ok {
			t.Fatal("renaming the draft")
		}
		if _, renamed := slugOf("Готовая статья"); renamed != draft {
			t.Errorf("renamed draft: %q, want %q", renamed, draft)
		}

		createArticle(url.Values{"title": {"По-своему"}, "slug": {"my/own-page"}})
		id, slug = slugOf("По-своему")
		if slug != "my/own-page" {
			t.Errorf("typed by hand: %q", slug)
		}
		edit = "/node/" + strconv.Itoa(id) + "/edit"
		if _, ok := s.submit(edit, "", url.Values{"title": {"По-своему, но иначе"}}); !ok {
			t.Fatal("renaming the article by hand")
		}
		if _, slug = slugOf("По-своему, но иначе"); slug != "my/own-page" {
			t.Errorf("renamed, by hand: %q", slug)
		}
		s.checkLog()
	})
}
