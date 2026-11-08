//go:build dbintegration

package app

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestFormPreview - a content form previewed (its Preview button): the
// page of what the form describes, a new item's and an edited one's,
// under the strip - with what saving would refuse and the note of a file
// chosen; nothing saved, no file stored, not indexed, kept by no cache.
// The type's setting turns it off.
func TestFormPreview(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		count := func(query string) int {
			t.Helper()
			var n int
			if err := db.DB.QueryRow(query).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		}
		var pictureID int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT fi.id FROM field_instances fi JOIN field_storages fs ON fs.id = fi.field_storage_id
			JOIN content_types ct ON ct.id = fi.content_type_id WHERE fs.machine_name = ? AND ct.machine_name = ?`), "field_image", "article").Scan(&pictureID); err != nil {
			t.Fatal(err)
		}
		var pic bytes.Buffer
		if err := png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 40, 30))); err != nil {
			t.Fatal(err)
		}
		files := []upload{{"field_" + strconv.Itoa(pictureID), "cover.png", pic.String()}}

		_, form := s.get("/create/article")
		if !strings.Contains(form, `formaction="/create/article/preview"`) {
			t.Fatal("no Preview button on /create/article")
		}
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		nodes, stored := count("SELECT COUNT(*) FROM nodes"), count("SELECT COUNT(*) FROM files")
		resp, page := s.postUploadsPage("/create/article/preview", url.Values{"csrf_token": {s.csrf("/create/article")},
			"title": {"An unsaved idea"}, body[1]: {"<p>Words not saved yet.</p>"}}, files)
		if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Cache-Control"), "no-store") || !strings.Contains(resp.Header.Get("X-Robots-Tag"), "noindex") {
			t.Errorf("the preview: %d, Cache-Control %q, X-Robots-Tag %q", resp.StatusCode, resp.Header.Get("Cache-Control"), resp.Header.Get("X-Robots-Tag"))
		}
		for _, want := range []string{`class="unsaved-preview"`, "An unsaved idea", "Words not saved yet.", "The files chosen in the form show once it is saved."} {
			if !strings.Contains(page, want) {
				t.Errorf("the new item's preview: no %q", want)
			}
		}
		if got := count("SELECT COUNT(*) FROM nodes"); got != nodes {
			t.Errorf("nodes: %d, want %d", got, nodes)
		}
		if got := count("SELECT COUNT(*) FROM files"); got != stored {
			t.Errorf("files: %d, want %d - a preview stores none", got, stored)
		}
		if _, page := s.postUploadsPage("/create/article/preview", url.Values{"csrf_token": {s.csrf("/create/article")}, body[1]: {"<p>No title.</p>"}}, nil); !strings.Contains(page, "Saving would be refused:") || !strings.Contains(page, "No title.") {
			t.Error("a form saving would refuse: no preview with its errors")
		}

		if resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {"Saved words"}, body[1]: {"<p>As saved.</p>"}, "status": {"1"}}); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		var id int
		var slug string
		if err := db.DB.QueryRow(`SELECT id, slug FROM nodes WHERE title = 'Saved words'`).Scan(&id, &slug); err != nil {
			t.Fatal(err)
		}
		edit := fmt.Sprintf("/node/%d/edit", id)
		if _, page := s.get(edit); !strings.Contains(page, fmt.Sprintf(`formaction="/node/%d/edit/preview"`, id)) {
			t.Fatal("no Preview button on the edit form")
		}
		_, page = s.postUploadsPage(edit+"/preview", url.Values{"csrf_token": {s.csrf(edit)}, "title": {"Changed words"}, body[1]: {"<p>As changed.</p>"}, "status": {"1"}}, nil)
		if !strings.Contains(page, "Changed words") || !strings.Contains(page, "As changed.") || strings.Contains(page, "As saved.") {
			t.Error("the edited item's preview doesn't show the form's values")
		}
		if _, page := s.get("/" + slug); !strings.Contains(page, "As saved.") || strings.Contains(page, "As changed.") {
			t.Error("the preview changed the saved item")
		}

		// Turned off on the type's settings (on by default): no button,
		// no preview.
		var typeID int
		if err := db.DB.QueryRow(`SELECT id FROM content_types WHERE machine_name = 'article'`).Scan(&typeID); err != nil {
			t.Fatal(err)
		}
		typePage := fmt.Sprintf("/admin/config/content-types/%d", typeID)
		if _, page := s.get(typePage); !strings.Contains(page, `name="preview" value="1" class="form-check-input" checked`) {
			t.Error("the type's preview isn't on by default")
		}
		if _, ok := s.submit(typePage, "", url.Values{"preview": nil}); !ok {
			t.Fatal("turning the preview off")
		}
		if _, page := s.get(edit); strings.Contains(page, "node-edit-preview") {
			t.Error("a Preview button on a type without preview")
		}
		if resp := s.postMultipart(edit+"/preview", url.Values{"csrf_token": {s.csrf(edit)}, "title": {"Changed words"}}); resp.StatusCode != http.StatusNotFound {
			t.Errorf("a type without preview previews: %d", resp.StatusCode)
		}
	})
}
