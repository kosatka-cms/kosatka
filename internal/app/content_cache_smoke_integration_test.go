//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestViewBlockFollowsContent - a cached View block in the sidebar shows
// an article created, renamed and deleted within seconds, with no refresh
// asked for: the content's changes rebuild the blocks' cache themselves.
func TestViewBlockFollowsContent(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		if _, err := db.DB.Exec(`UPDATE content_types SET default_publishing = 'published' WHERE machine_name = 'article'`); err != nil {
			t.Fatal(err)
		}
		var display int
		if err := db.DB.QueryRow(`SELECT id FROM view_displays WHERE machine_name = 'frontpage'`).Scan(&display); err != nil {
			t.Fatal(err)
		}
		if resp := s.post("/admin/config/blocks/view", url.Values{"csrf_token": {s.csrf("/admin/config/blocks")}, "view_display_id": {fmt.Sprint(display)},
			"region": {"sidebar"}, "visibility_scope": {"all"}}); resp.StatusCode != http.StatusSeeOther || strings.Contains(resp.Header.Get("Location"), "error") {
			t.Fatalf("place the View block: %d %s", resp.StatusCode, resp.Header.Get("Location"))
		}

		// Not the front page: its feed is no cached block.
		const page = "/comments/recent"
		guest := s.visitor()
		shows := func(title string) bool {
			t.Helper()
			_, html := guest.get(page)
			return strings.Contains(html, title)
		}
		// waitFor - whether the block comes to show title (or not to).
		waitFor := func(title string, want bool) bool {
			t.Helper()
			for deadline := time.Now().Add(15 * time.Second); shows(title) != want && time.Now().Before(deadline); {
				time.Sleep(200 * time.Millisecond)
			}
			return shows(title) == want
		}

		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		fields := func(path, title string) url.Values {
			return url.Values{"csrf_token": {s.csrf(path)}, "title": {title}, body[1]: {"<p>Body</p>"}, "status": {"1"}}
		}
		if resp := s.postMultipart("/create/article", fields("/create/article", "Fresh story")); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		if !waitFor("Fresh story", true) {
			t.Fatal("the block doesn't show the article created")
		}

		var id int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id FROM nodes WHERE title = ?`), "Fresh story").Scan(&id); err != nil {
			t.Fatal(err)
		}
		edit := fmt.Sprintf("/node/%d/edit", id)
		if resp := s.postMultipart(edit, fields(edit, "Renamed story")); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("save: %d", resp.StatusCode)
		}
		if !waitFor("Renamed story", true) {
			t.Fatal("the block doesn't show the article renamed")
		}

		del := fmt.Sprintf("/node/%d/delete", id)
		if resp := s.post(del, url.Values{"csrf_token": {s.csrf(edit)}}); resp.StatusCode >= 400 {
			t.Fatalf("delete: %d", resp.StatusCode)
		}
		if !waitFor("Renamed story", false) {
			t.Error("the block still shows the article deleted")
		}
		s.checkLog()
	})
}
