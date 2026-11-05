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

// TestMediaPicker - an article's image field picks from the media
// library: the picker offers the site's images, not a document; a new
// article takes a cover another article has (the same file row, now
// used in two places); a document picked into the image field is refused,
// the form shown again; a guest gets nothing.
func TestMediaPicker(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, true)
		s.signIn()
		var field int
		if err := db.DB.QueryRow(`SELECT fi.id FROM field_instances fi JOIN content_types ct ON ct.id = fi.content_type_id
JOIN field_storages fs ON fs.id = fi.field_storage_id WHERE ct.machine_name = 'article' AND fs.field_type = 'image'`).Scan(&field); err != nil {
			t.Fatal(err)
		}
		var cover, doc int
		if err := db.DB.QueryRow(`SELECT f.id FROM files f JOIN field_value_file v ON v.file_id = f.id WHERE f.uri LIKE '%demo-1.jpg'`).Scan(&cover); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.Exec(`INSERT INTO files (uri, mime_type, size_bytes, created_at, updated_at) VALUES ('public://content/files/rules.pdf', 'application/pdf', 10, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
			t.Fatal(err)
		}
		if err := db.DB.QueryRow(`SELECT id FROM files WHERE uri = 'public://content/files/rules.pdf'`).Scan(&doc); err != nil {
			t.Fatal(err)
		}

		pick := fmt.Sprintf("/api/media/pick?field=%d&q=demo-1", field)
		code, body := s.get(pick)
		var got struct {
			Items []struct {
				ID    int    `json:"id"`
				Thumb string `json:"thumb"`
			} `json:"items"`
		}
		if code != http.StatusOK || json.Unmarshal([]byte(body), &got) != nil {
			t.Fatalf("GET %s: %d %s", pick, code, body)
		}
		offered := false
		for _, it := range got.Items {
			offered = offered || (it.ID == cover && it.Thumb != "")
		}
		if !offered {
			t.Errorf("the picker doesn't offer the cover: %s", body)
		}
		if _, body := s.get(fmt.Sprintf("/api/media/pick?field=%d&q=rules", field)); strings.Contains(body, "rules.pdf") {
			t.Error("the image field's picker offers a document")
		}
		if code, _ := s.visitor().get(pick); code == http.StatusOK {
			t.Errorf("a guest's picker: %d", code)
		}

		_, form := s.get("/create/article")
		if !strings.Contains(form, fmt.Sprintf(`data-media-picker="%d"`, field)) {
			t.Fatal("the article form has no media library picker")
		}
		create := func(title string, picked int) int {
			t.Helper()
			resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {title}, "status": {"1"},
				fmt.Sprintf("field_%d_library", field): {fmt.Sprint(picked)}})
			if resp.StatusCode != http.StatusFound {
				t.Fatalf("create %q: %d", title, resp.StatusCode)
			}
			var value int
			err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT COALESCE(MAX(v.file_id), 0) FROM nodes n LEFT JOIN field_value_file v ON v.node_id = n.id WHERE n.title = ?`), title).Scan(&value)
			if err != nil {
				t.Fatal(err)
			}
			return value
		}
		if got := create("The same harbour", cover); got != cover {
			t.Errorf("the picked cover: file %d, want %d", got, cover)
		}
		// A document into the image field: the form again, nothing saved.
		resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {"Rules as a picture"},
			"status": {"1"}, fmt.Sprintf("field_%d_library", field): {fmt.Sprint(doc)}})
		var n int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM nodes WHERE title = 'Rules as a picture'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK || n != 0 {
			t.Errorf("a document picked into an image field: %d, %d saved", resp.StatusCode, n)
		}
		if _, page := s.get(fmt.Sprintf("/admin/content/media?file=%d", cover)); !regexp.MustCompile(`Used in 2 places`).MatchString(page) {
			t.Error("the media library doesn't count the cover's second place")
		}
		s.checkLog()
	})
}
