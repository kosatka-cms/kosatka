//go:build dbintegration

package app

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestContentTrash - a deleted article goes to the trash: its page is
// gone, the trash lists it (who, the type); restored, it is back;
// deleted again and deleted for good, its row is gone. A bulk delete
// puts the content in the trash too, and the trash is emptied at once.
func TestContentTrash(t *testing.T) {
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
		create := func(title string) (int, string) {
			t.Helper()
			if resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {title}, body[1]: {"<p>Body</p>"}, "status": {"1"}}); resp.StatusCode != http.StatusFound {
				t.Fatalf("create %q: %d", title, resp.StatusCode)
			}
			var id int
			var slug string
			if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id, slug FROM nodes WHERE title = ?`), title).Scan(&id, &slug); err != nil {
				t.Fatal(err)
			}
			return id, slug
		}
		id, slug := create("Old news")

		del := fmt.Sprintf("/node/%d/delete", id)
		if _, page := s.get(del); !strings.Contains(page, "trash") && !strings.Contains(page, "корзин") {
			t.Error("the confirmation doesn't speak of the trash")
		}
		if resp := s.post(del, url.Values{"csrf_token": {s.csrf(del)}}); resp.StatusCode != http.StatusFound {
			t.Fatalf("delete: %d", resp.StatusCode)
		}
		if code, _ := s.get("/" + slug); code != http.StatusNotFound {
			t.Errorf("the page of content in the trash: %d", code)
		}
		if _, page := s.get("/admin/content/trash"); !strings.Contains(page, "Old news") {
			t.Error("the trash doesn't list it")
		}

		trash := "/admin/content/trash"
		if resp := s.post(fmt.Sprintf("%s/%d/restore", trash, id), url.Values{"csrf_token": {s.csrf(trash)}}); !strings.Contains(resp.Header.Get("Location"), "success=restored") {
			t.Fatalf("restore: %d %s", resp.StatusCode, resp.Header.Get("Location"))
		}
		if code, _ := s.get("/" + slug); code != http.StatusOK {
			t.Errorf("the restored page: %d", code)
		}
		if _, page := s.get(trash + "?success=restored"); !strings.Contains(page, "alert-success") {
			t.Error("no notice of the restoring")
		}

		_ = s.post(del, url.Values{"csrf_token": {s.csrf(del)}})
		if resp := s.post(fmt.Sprintf("%s/%d/delete", trash, id), url.Values{"csrf_token": {s.csrf(trash)}}); !strings.Contains(resp.Header.Get("Location"), "success=deleted") {
			t.Fatalf("delete for good: %s", resp.Header.Get("Location"))
		}
		var n int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT COUNT(*) FROM nodes WHERE id = ?`), id).Scan(&n); err != nil || n != 0 {
			t.Errorf("the row after deleting for good: %d %v", n, err)
		}

		// A bulk delete: to the trash; then the trash emptied.
		other, _ := create("Bulk news")
		req, _ := http.NewRequest(http.MethodPost, s.base+"/api/admin/bulk/content", bytes.NewBufferString(fmt.Sprintf(`{"action":"delete","ids":[%d]}`, other)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", s.csrf("/admin/content"))
		resp, err := s.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("bulk delete: %v %v", resp, err)
		}
		_ = resp.Body.Close()
		var deletedAt *string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT CAST(deleted_at AS CHAR(32)) FROM nodes WHERE id = ?`), other).Scan(&deletedAt); err != nil || deletedAt == nil {
			t.Errorf("bulk-deleted: in the trash? %v %v", deletedAt, err)
		}
		if resp := s.post(trash+"/empty", url.Values{"csrf_token": {s.csrf(trash)}}); !strings.Contains(resp.Header.Get("Location"), "success=emptied") {
			t.Fatalf("empty: %s", resp.Header.Get("Location"))
		}
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM nodes WHERE deleted_at IS NOT NULL`).Scan(&n); err != nil || n != 0 {
			t.Errorf("in the trash after emptying: %d %v", n, err)
		}
	})
}
