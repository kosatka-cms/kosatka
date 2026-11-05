//go:build dbintegration

package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestNodeSeveralImages - an image field with several images, end to end:
// added on the Fields page, an article gets two at once, its page shows
// them in a lightbox grid; the edit form reorders them, describes one,
// refuses more than the field's limit and deletes one.
func TestNodeSeveralImages(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSite(t, db, nil)
		s.signIn()
		q := func(query string, args ...any) *sqlRow {
			return &sqlRow{t: t, row: db.DB.QueryRow(db.Dialect.Rebind(query), args...)}
		}

		var typeID int
		q("SELECT id FROM content_types WHERE machine_name = ?", "article").scan(&typeID)
		fields := fmt.Sprintf("/admin/config/content-types/%d/fields", typeID)
		if resp := s.post(fields, url.Values{"csrf_token": {s.csrf(fields)}, "label": {"Photos"}, "machine_name": {"photos"},
			"field_type": {"image"}, "multiple": {"1"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("add the field: %d", resp.StatusCode)
		}
		var fieldID int
		q("SELECT fi.id FROM field_instances fi JOIN field_storages fs ON fs.id = fi.field_storage_id WHERE fs.machine_name = ?", "field_photos").scan(&fieldID)
		if _, err := db.DB.Exec(db.Dialect.Rebind("UPDATE field_instances SET config = ?, detail_image_style = ? WHERE id = ?"),
			`{"max_values": 3}`, "article_wide", fieldID); err != nil {
			t.Fatal(err)
		}
		field := "field_" + strconv.Itoa(fieldID)

		picture := func(r uint8) string {
			var b bytes.Buffer
			img := image.NewRGBA(image.Rect(0, 0, 40, 30))
			img.Set(0, 0, color.RGBA{R: r, A: 255})
			if err := png.Encode(&b, img); err != nil {
				t.Fatal(err)
			}
			return b.String()
		}
		pictures := func(n int) []upload {
			var out []upload
			for i := range n {
				out = append(out, upload{field, fmt.Sprintf("p%d.png", i), picture(uint8(10 * (i + 1)))})
			}
			return out
		}
		bodyField := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`)
		form := func(path string) url.Values {
			_, page := s.get(path)
			body := bodyField.FindStringSubmatch(page)
			if body == nil {
				t.Fatalf("no body field on %s", path)
			}
			return url.Values{"csrf_token": {s.csrf(path)}, "title": {"Photo story"}, body[1]: {"<p>Body</p>"}, "status": {"1"}}
		}
		images := func(nodeID int) []int {
			rows, err := db.DB.Query(db.Dialect.Rebind("SELECT file_id FROM field_value_file_multi WHERE node_id = ? ORDER BY delta"), nodeID)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rows.Close() }()
			var ids []int
			for rows.Next() {
				var id int
				if err := rows.Scan(&id); err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			return ids
		}

		if resp := s.postUploads("/create/article", form("/create/article"), pictures(2)); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		var nodeID int
		var slug string
		q("SELECT id, slug FROM nodes WHERE title = ?", "Photo story").scan(&nodeID, &slug)
		first := images(nodeID)
		if len(first) != 2 {
			t.Fatalf("saved images: %v", first)
		}
		if _, page := s.get("/" + slug); !strings.Contains(page, "dynamic-field-images-grid") || strings.Count(page, `class="content-image-link"`) < 2 {
			t.Errorf("the page doesn't show both images in a grid")
		}

		edit := fmt.Sprintf("/node/%d/edit", nodeID)
		save := func(extra url.Values, files []upload) *http.Response {
			t.Helper()
			v := form(edit)
			for k, vs := range extra {
				v[k] = vs
			}
			return s.postUploads(edit, v, files)
		}
		// Reversed, the second described.
		reversed := []string{strconv.Itoa(first[1]), strconv.Itoa(first[0])}
		save(url.Values{field + "_keep": reversed, fmt.Sprintf("%s_alt_%d", field, first[1]): {"A red square"}}, nil)
		if got := images(nodeID); !slices.Equal(got, []int{first[1], first[0]}) {
			t.Errorf("reordered: %v, want %v", got, []int{first[1], first[0]})
		}
		var alt string
		q("SELECT alt FROM files WHERE id = ?", first[1]).scan(&alt)
		if alt != "A red square" {
			t.Errorf("alt: %q", alt)
		}
		// Two more would be four of at most three: refused, nothing changes.
		save(url.Values{field + "_keep": reversed}, pictures(2))
		if got := images(nodeID); len(got) != 2 {
			t.Errorf("over the limit: %v", got)
		}
		// One deleted, one added.
		save(url.Values{field + "_keep": reversed, field + "_delete": {strconv.Itoa(first[0])}}, pictures(1))
		if got := images(nodeID); len(got) != 2 || got[0] != first[1] || got[1] == first[0] {
			t.Errorf("one deleted, one added: %v", got)
		}
		s.checkLog()
	})
}

// sqlRow - a row the test scans, failing on an error.
type sqlRow struct {
	t   *testing.T
	row interface{ Scan(...any) error }
}

func (r *sqlRow) scan(dest ...any) {
	r.t.Helper()
	if err := r.row.Scan(dest...); err != nil {
		r.t.Fatal(err)
	}
}
