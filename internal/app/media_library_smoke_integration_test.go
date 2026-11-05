//go:build dbintegration

package app

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/internal/testdb"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// TestMediaLibrary - the demo site's files in the media library: a
// cover's card names its article; its alt text and focal point saved
// (the article's picture cropped at it); a file nothing
// uses found as such and deleted with its stored file; one in use not
// deleted; a gallery's picture's card names the module.
func TestMediaLibrary(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, true)
		s.signIn()
		const page = "/admin/content/media"
		if code, body := s.get(page); code != http.StatusOK || !strings.Contains(body, "demo-1.jpg") || !strings.Contains(body, `class="media-grid"`) {
			t.Fatalf("GET %s: %d", page, code)
		}

		var cover int
		var title string
		if err := db.DB.QueryRow(`SELECT f.id, n.title FROM files f JOIN field_value_file v ON v.file_id = f.id JOIN nodes n ON n.id = v.node_id
WHERE f.uri LIKE '%demo-1.jpg'`).Scan(&cover, &title); err != nil {
			t.Fatal(err)
		}
		card := fmt.Sprintf("%s?file=%d", page, cover)
		if _, body := s.get(card); !strings.Contains(body, title) || !strings.Contains(body, "Used in 1 place") {
			t.Errorf("the cover's card names no %q", title)
		}
		if resp := s.post(fmt.Sprintf("%s/%d", page, cover), url.Values{"csrf_token": {s.csrf(card)}, "alt": {"A harbour at dawn"}}); resp.StatusCode != http.StatusSeeOther {
			t.Errorf("saving the alt text: %d", resp.StatusCode)
		}
		var alt string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT alt FROM files WHERE id = ?`), cover).Scan(&alt); err != nil || alt != "A harbour at dawn" {
			t.Errorf("the alt text: %q %v", alt, err)
		}
		// Its focal point: the article's picture (a style cropping at it)
		// gets an address with it, made on request.
		if resp := s.post(fmt.Sprintf("%s/%d", page, cover), url.Values{"csrf_token": {s.csrf(card)}, "alt": {"A harbour at dawn"}, "focal_x": {"10"}, "focal_y": {"20"}}); !strings.Contains(resp.Header.Get("Location"), "success=saved") {
			t.Errorf("saving the focal point: %s", resp.Header.Get("Location"))
		}
		var slug string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT n.slug FROM nodes n JOIN field_value_file v ON v.node_id = n.id WHERE v.file_id = ?`), cover).Scan(&slug); err != nil {
			t.Fatal(err)
		}
		_, article := s.get("/" + slug)
		focused := regexp.MustCompile(`src="([^"]+/styles/article_wide/[^"]+f=10-20)"`).FindStringSubmatch(article)
		if focused == nil {
			t.Fatal("the article's picture has no address with its focal point")
		}
		if code, _ := s.get(strings.ReplaceAll(focused[1], "&amp;", "&")); code != http.StatusOK {
			t.Errorf("GET %s: %d", focused[1], code)
		}
		if resp := s.post(fmt.Sprintf("%s/%d", page, cover), url.Values{"csrf_token": {s.csrf(card)}, "focal_x": {"150"}, "focal_y": {"20"}}); !strings.Contains(resp.Header.Get("Location"), "error=save") {
			t.Errorf("a point outside the image: %s", resp.Header.Get("Location"))
		}
		if resp := s.post(fmt.Sprintf("%s/%d/delete", page, cover), url.Values{"csrf_token": {s.csrf(card)}}); !strings.Contains(resp.Header.Get("Location"), "error=in_use") {
			t.Errorf("deleting a file in use: %s", resp.Header.Get("Location"))
		}

		// A file nothing uses.
		const key = "content/images/stray.jpg"
		if _, err := paths.PublicStore().Put(context.Background(), key, bytes.NewReader([]byte("x")), filestore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO files (uri, mime_type, size_bytes, created_at, updated_at) VALUES (?, 'image/jpeg', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`), "public://"+key); err != nil {
			t.Fatal(err)
		}
		var stray int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id FROM files WHERE uri = ?`), "public://"+key).Scan(&stray); err != nil {
			t.Fatal(err)
		}
		_, unused := s.get(page + "?unused=1")
		if !strings.Contains(unused, "stray.jpg") || strings.Contains(unused, "demo-1.jpg") {
			t.Error("the unused files: not only the stray one")
		}
		if resp := s.post(fmt.Sprintf("%s/%d/delete", page, stray), url.Values{"csrf_token": {s.csrf(page)}}); !strings.Contains(resp.Header.Get("Location"), "success=deleted") {
			t.Errorf("deleting an unused file: %s", resp.Header.Get("Location"))
		}
		var n int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT COUNT(*) FROM files WHERE id = ?`), stray).Scan(&n); err != nil || n != 0 {
			t.Errorf("the stray file's row: %d %v", n, err)
		}
		if ok, _ := filestore.Exists(context.Background(), paths.PublicStore(), key); ok {
			t.Error("the stray file is still stored")
		}

		var picture int
		if err := db.DB.QueryRow(`SELECT fid FROM gallery_items ORDER BY id LIMIT 1`).Scan(&picture); err != nil {
			t.Fatal(err)
		}
		if _, body := s.get(fmt.Sprintf("%s?file=%d", page, picture)); !regexp.MustCompile(`</a>\s*·\s*Galleries\s*</li>`).MatchString(body) {
			t.Error("a gallery picture's card doesn't name the module")
		}
		s.checkLog()
	})
}
