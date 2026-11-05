//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestViewMachineNames - a display's machine name is changed in its
// settings: the front page feed choosing it follows, an invalid name or
// another display's is refused.
func TestViewMachineNames(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		var id int
		if err := db.DB.QueryRow(`SELECT id FROM view_displays WHERE machine_name = 'frontpage'`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		page := fmt.Sprintf("/admin/config/views/%d", id)
		if _, html := s.get(page); !strings.Contains(html, `name="machine_name" class="form-control admin-max-md" value="frontpage"`) {
			t.Fatal("the settings have no machine name")
		}
		if _, ok := s.submit(page, page, url.Values{"machine_name": {"home_feed"}}); !ok {
			t.Fatal("rename")
		}
		var name, feed string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT machine_name FROM view_displays WHERE id = ?`), id).Scan(&name); err != nil || name != "home_feed" {
			t.Errorf("renamed to %q %v", name, err)
		}
		if err := db.DB.QueryRow(`SELECT front_feed_view FROM site_information`).Scan(&feed); err != nil || feed != "home_feed" {
			t.Errorf("the front page feed: %q %v", feed, err)
		}
		if code, _ := s.visitor().get("/"); code != http.StatusOK {
			t.Errorf("the front page: %d", code)
		}

		var other string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT machine_name FROM view_displays WHERE id <> ? AND is_default = ? ORDER BY id`), id, false).Scan(&other); err != nil {
			t.Fatal(err)
		}
		for name, code := range map[string]string{other: "duplicate", "Bad Name": "machine-name"} {
			resp := s.post(page, url.Values{"csrf_token": {s.csrf(page)}, "label": {"Front page"}, "display_type": {"block"}, "status": {"1"}, "machine_name": {name}})
			if where := resp.Header.Get("Location"); !strings.Contains(where, "error="+code) {
				t.Errorf("name %q: %d → %q, want error=%s", name, resp.StatusCode, where, code)
			}
		}
	})
}
