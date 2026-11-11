//go:build dbintegration

package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestViewsPublishedEverywhere - a new site's Views come published, and
// visitors see the published versions wherever a View shows: a change made
// since (here written straight to the live rows, as the editor saves it)
// reaches the front page's feed and a sidebar block only once the display
// is published again.
func TestViewsPublishedEverywhere(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, true)
		ctx := context.Background()
		var unpublished int
		if err := db.DB.QueryRowContext(ctx, db.Dialect.Rebind("SELECT COUNT(*) FROM view_displays WHERE is_default = ? AND published_at IS NULL"), false).Scan(&unpublished); err != nil {
			t.Fatal(err)
		}
		if unpublished != 0 {
			t.Fatalf("%d View displays not published after the install", unpublished)
		}

		s.signIn()
		for _, d := range []struct{ name, draft string }{
			{"frontpage", "A draft of the front page"},
			{demoPopularView, "A draft of the popular block"},
		} {
			var id int
			if err := db.DB.QueryRowContext(ctx, db.Dialect.Rebind("SELECT id FROM view_displays WHERE machine_name = ?"), d.name).Scan(&id); err != nil {
				t.Fatal(d.name, err)
			}
			if _, err := db.DB.ExecContext(ctx, db.Dialect.Rebind("UPDATE view_displays SET header_text = ? WHERE id = ?"), "<p>"+d.draft+"</p>", id); err != nil {
				t.Fatal(err)
			}
			if _, home := s.get("/"); strings.Contains(home, d.draft) {
				t.Errorf("%s: the front page shows the unpublished change", d.name)
			}
			edit := fmt.Sprintf("/admin/config/views/%d", id)
			if resp := s.post(edit+"/publish", url.Values{"csrf_token": {s.csrf(edit)}}); resp.StatusCode >= http.StatusBadRequest {
				t.Fatalf("%s: publishing: %d", d.name, resp.StatusCode)
			}
			// A block is rebuilt in the background after publishing.
			shown := false
			for deadline := time.Now().Add(10 * time.Second); !shown && time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
				_, home := s.get("/")
				shown = strings.Contains(home, d.draft)
			}
			if !shown {
				t.Errorf("%s: the front page doesn't show the published change", d.name)
			}
		}
	})
}
