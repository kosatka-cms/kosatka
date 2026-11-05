//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestContentClone - "Make a copy" on an article: a new unpublished one,
// "Copy of …", with the same text, an address of its own, its form opened;
// the original stays as it was.
func TestContentClone(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		if _, err := db.DB.Exec(`UPDATE content_types SET default_publishing = 'published' WHERE machine_name = 'article'`); err != nil {
			t.Fatal(err)
		}
		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		if resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {`Spring "sale" <now>`}, body[1]: {"<p>Everything half price.</p>"}, "status": {"1"}}); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		var id int
		var slug string
		if err := db.DB.QueryRow(`SELECT id, slug FROM nodes WHERE title LIKE 'Spring "sale"%'`).Scan(&id, &slug); err != nil {
			t.Fatal(err)
		}
		// The copy is asked first, the title escaped once in the question.
		if _, page := s.get("/" + slug); !strings.Contains(page, fmt.Sprintf(`action="/node/%d/clone"`, id)) ||
			!strings.Contains(page, `data-confirm="Make a copy of &#34;Spring &#34;sale&#34; &lt;now&gt;&#34;? It will be an unpublished draft."`) {
			t.Fatal("the page has no copy button")
		}
		clone := fmt.Sprintf("/node/%d/clone", id)
		resp := s.post(clone, url.Values{"csrf_token": {s.csrf("/" + slug)}})
		where := resp.Header.Get("Location")
		if resp.StatusCode != http.StatusSeeOther || !strings.HasSuffix(where, "/edit") {
			t.Fatalf("copy: %d %s", resp.StatusCode, where)
		}
		var copyID, status int
		var copySlug string
		if err := db.DB.QueryRow(`SELECT id, slug, status FROM nodes WHERE title LIKE 'Copy of Spring "sale"%'`).Scan(&copyID, &copySlug, &status); err != nil {
			t.Fatal(err)
		}
		if where != fmt.Sprintf("/node/%d/edit", copyID) || status != 0 || copySlug == slug {
			t.Errorf("the copy: %s, status %d, slug %q (the original's %q)", where, status, copySlug, slug)
		}
		if _, page := s.get(where); !strings.Contains(page, "Everything half price.") || !strings.Contains(page, `value="Copy of Spring &#34;sale&#34; &lt;now&gt;"`) {
			t.Error("the copy's form lacks the text or the title")
		}
		if _, page := s.get("/" + slug); !strings.Contains(page, "Everything half price.") {
			t.Error("the original lost its text")
		}
		if code, _ := s.visitor().get("/" + copySlug); code == http.StatusOK {
			t.Errorf("a guest opens the unpublished copy: %d", code)
		}
	})
}
