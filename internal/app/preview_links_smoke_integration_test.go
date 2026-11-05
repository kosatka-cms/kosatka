//go:build dbintegration

package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestPreviewLinks - an unpublished article: a guest doesn't see it, but
// its preview link shows its page to them, not indexed and kept by no
// cache; a new link replaces the old one; revoked, it shows nothing.
func TestPreviewLinks(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		if resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {"Secret draft"}, body[1]: {"<p>Not yet.</p>"}, "status": {"0"}}); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		var id int
		var slug string
		if err := db.DB.QueryRow(`SELECT id, slug FROM nodes WHERE title = 'Secret draft'`).Scan(&id, &slug); err != nil {
			t.Fatal(err)
		}
		edit := fmt.Sprintf("/node/%d/edit", id)
		if _, page := s.get(edit); !strings.Contains(page, `data-preview-link="/node/`) {
			t.Fatal("the form has no preview link card")
		}
		link := func(action string) map[string]string {
			t.Helper()
			resp, err := s.client.PostForm(s.base+fmt.Sprintf("/node/%d/preview-link", id), url.Values{"csrf_token": {s.csrf(edit)}, "action": {action}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			out := map[string]string{}
			if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&out) != nil {
				t.Fatalf("%s: %d", action, resp.StatusCode)
			}
			return out
		}
		first := link("create")
		path := first["url"][strings.Index(first["url"], "/preview/"):]

		guest := s.visitor()
		if code, _ := guest.get("/" + slug); code == http.StatusOK {
			t.Error("a guest sees the draft")
		}
		resp, err := guest.client.Get(guest.base + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Cache-Control"), "no-store") || !strings.Contains(resp.Header.Get("X-Robots-Tag"), "noindex") {
			t.Errorf("the preview: %d, Cache-Control %q, X-Robots-Tag %q", resp.StatusCode, resp.Header.Get("Cache-Control"), resp.Header.Get("X-Robots-Tag"))
		}
		if _, page := guest.get(path); !strings.Contains(page, "Not yet.") {
			t.Error("the preview doesn't show the draft")
		}

		second := link("create")
		if code, _ := guest.get(path); code != http.StatusNotFound {
			t.Errorf("a replaced link: %d", code)
		}
		path = second["url"][strings.Index(second["url"], "/preview/"):]
		link("revoke")
		if code, _ := guest.get(path); code != http.StatusNotFound {
			t.Errorf("a revoked link: %d", code)
		}
		if code, _ := guest.get("/preview/nope"); code != http.StatusNotFound {
			t.Errorf("an unknown link: %d", code)
		}
	})
}
