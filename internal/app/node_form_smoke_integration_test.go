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

// TestNodeFormWritesOnSave - what the content form only notes while it is
// read is written with the content: a form sent back with errors creates
// no new tag and changes no alt text, and shows what was typed - the text,
// the new tag, the alt text, the image uploaded, kept for the next submit
// (which needn't upload it again); the valid form creates the tag,
// indexes it and writes the alt text. Only a saved form names its draft
// (the autosave script's key) to be dropped.
func TestNodeFormWritesOnSave(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSite(t, db, nil)
		s.signIn()
		q := func(query string, args ...any) *sqlRow {
			return &sqlRow{t: t, row: db.DB.QueryRow(db.Dialect.Rebind(query), args...)}
		}
		exec := func(query string, args ...any) {
			t.Helper()
			if _, err := db.DB.Exec(db.Dialect.Rebind(query), args...); err != nil {
				t.Fatal(err)
			}
		}

		var typeID int
		q("SELECT id FROM content_types WHERE machine_name = ?", "article").scan(&typeID)
		exec("INSERT INTO taxonomy_vocabulary (id, name, slug) VALUES (60, 'Topics', 'topics')")
		fields := fmt.Sprintf("/admin/config/content-types/%d/fields", typeID)
		if resp := s.post(fields, url.Values{"csrf_token": {s.csrf(fields)}, "label": {"Topics"}, "machine_name": {"topics"},
			"field_type": {"taxonomy"}, "multiple": {"1"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("add the field: %d", resp.StatusCode)
		}
		fieldOf := func(storage string) string {
			var id int
			q("SELECT fi.id FROM field_instances fi JOIN field_storages fs ON fs.id = fi.field_storage_id WHERE fs.machine_name = ? AND fi.content_type_id = ?", storage, typeID).scan(&id)
			return "field_" + strconv.Itoa(id)
		}
		tags, picture := fieldOf("field_topics"), fieldOf("field_image")
		exec("UPDATE field_storages SET config = ? WHERE machine_name = ?", `{"multiple": true, "vocabulary_id": 60}`, "field_topics")
		exec("UPDATE field_instances SET config = ? WHERE id = ?", `{"widget": "autocomplete"}`, strings.TrimPrefix(tags, "field_"))

		var png1 bytes.Buffer
		if err := png.Encode(&png1, image.NewRGBA(image.Rect(0, 0, 40, 30))); err != nil {
			t.Fatal(err)
		}
		bodyField := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`)
		form := func(path, title string, extra url.Values) url.Values {
			t.Helper()
			_, page := s.get(path)
			body := bodyField.FindStringSubmatch(page)
			if body == nil {
				t.Fatalf("no body field on %s", path)
			}
			v := url.Values{"csrf_token": {s.csrf(path)}, "title": {title}, body[1]: {"<p>Typed body " + title + "</p>"}, "status": {"1"}}
			for k, vs := range extra {
				v[k] = vs
			}
			return v
		}
		// send posts the form: the answer, and the draft it names saved.
		send := func(path string, v url.Values, files []upload) (code int, page, saved string) {
			t.Helper()
			resp, page := s.postUploadsPage(path, v, files)
			for _, c := range resp.Cookies() {
				if c.Name == "kosatka_draft_saved" {
					saved = c.Value
				}
			}
			return resp.StatusCode, page, saved
		}
		draftKey := regexp.MustCompile(`data-autosave="([^"]+)"`)
		keyOf := func(path string) string {
			t.Helper()
			_, page := s.get(path)
			m := draftKey.FindStringSubmatch(page)
			if m == nil {
				t.Fatalf("no draft key on %s", path)
			}
			return m[1]
		}
		terms := func(name string) int {
			var n int
			q("SELECT COUNT(*) FROM taxonomy_term_data WHERE vid = 60 AND name = ?", name).scan(&n)
			return n
		}
		lastAlt := func() string {
			var alt string
			q("SELECT COALESCE(alt, '') FROM files ORDER BY id DESC LIMIT 1").scan(&alt)
			return alt
		}
		cover := []upload{{picture, "cover.png", png1.String()}}

		// Sent back (no title): the upload is kept, the rest only shown.
		code, page, saved := send("/create/article", form("/create/article", "", url.Values{tags: {"new:Early tag"}, picture + "_alt": {"Early alt"}}), cover)
		if code != http.StatusOK || !strings.Contains(page, "The title field is required") {
			t.Fatalf("a form without a title: %d", code)
		}
		if saved != "" {
			t.Errorf("a form with errors named its draft saved: %q", saved)
		}
		if terms("Early tag") != 0 || lastAlt() != "" {
			t.Errorf("a form with errors created the tag (%d) or wrote the alt (%q)", terms("Early tag"), lastAlt())
		}
		if !strings.Contains(page, "Early tag") {
			t.Errorf("the form sent back lost the new tag")
		}
		if !strings.Contains(page, "Typed body ") {
			t.Errorf("the form sent back lost the text typed")
		}
		var uploaded int
		q("SELECT id FROM files ORDER BY id DESC LIMIT 1").scan(&uploaded)
		if !strings.Contains(page, fmt.Sprintf(`name="%s_sent" value="%d"`, picture, uploaded)) || !strings.Contains(page, `value="Early alt"`) {
			t.Errorf("the form sent back doesn't keep the image uploaded (file %d) or its alt", uploaded)
		}

		// The image kept: not uploaded again.
		code, _, saved = send("/create/article", form("/create/article", "Tagged story", url.Values{tags: {"new:Fresh tag"}, picture + "_alt": {"Cover alt"}, picture + "_sent": {fmt.Sprint(uploaded)}}), nil)
		if code != http.StatusFound {
			t.Fatalf("create: %d", code)
		}
		if want := keyOf("/create/article"); saved != want {
			t.Errorf("create: the draft saved %q, want %q", saved, want)
		}
		var nodeID, fresh, fileID int
		q("SELECT id FROM nodes WHERE title = ?", "Tagged story").scan(&nodeID)
		q("SELECT id FROM taxonomy_term_data WHERE vid = 60 AND name = ?", "Fresh tag").scan(&fresh)
		q("SELECT file_id FROM field_value_file WHERE node_id = ?", nodeID).scan(&fileID)
		indexed := func(tid int) bool {
			var n int
			q("SELECT COUNT(*) FROM taxonomy_index WHERE nid = ? AND tid = ?", nodeID, tid).scan(&n)
			return n == 1
		}
		alt := func() string {
			var a string
			q("SELECT COALESCE(alt, '') FROM files WHERE id = ?", fileID).scan(&a)
			return a
		}
		if !indexed(fresh) || alt() != "Cover alt" || fileID != uploaded {
			t.Errorf("create: the new tag indexed %v, alt %q, image %d (want the one kept, %d)", indexed(fresh), alt(), fileID, uploaded)
		}

		edit := fmt.Sprintf("/node/%d/edit", nodeID)
		changes := url.Values{tags: {fmt.Sprintf("%d,new:Second tag", fresh)}, picture + "_alt": {"Changed alt"}}
		code, page, saved = send(edit, form(edit, "", changes), nil)
		if code != http.StatusOK || !strings.Contains(page, "The title field is required") || saved != "" {
			t.Fatalf("an edit without a title: %d, the draft saved %q", code, saved)
		}
		if terms("Second tag") != 0 || alt() != "Cover alt" {
			t.Errorf("an edit with errors created the tag (%d) or wrote the alt (%q)", terms("Second tag"), alt())
		}
		if !strings.Contains(page, "Second tag") || !strings.Contains(page, `value="Changed alt"`) {
			t.Errorf("the edit sent back lost the new tag or the alt typed")
		}
		if !strings.Contains(page, "Typed body &lt;/p&gt;") || strings.Contains(page, "Typed body Tagged story") {
			t.Errorf("the edit sent back shows the saved text, not the one typed")
		}

		code, _, saved = send(edit, form(edit, "Tagged story", changes), nil)
		if code != http.StatusSeeOther {
			t.Fatalf("save: %d", code)
		}
		if want := keyOf(edit); saved != want {
			t.Errorf("save: the draft saved %q, want %q", saved, want)
		}
		var second int
		q("SELECT id FROM taxonomy_term_data WHERE vid = 60 AND name = ?", "Second tag").scan(&second)
		if !indexed(fresh) || !indexed(second) || alt() != "Changed alt" {
			t.Errorf("save: tags indexed %v %v, alt %q", indexed(fresh), indexed(second), alt())
		}
		s.checkLog()
	})
}
