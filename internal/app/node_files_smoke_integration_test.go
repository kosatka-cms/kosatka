//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestNodeSeveralFiles - a file field with several files, end to end:
// added on the Fields page with several values, an article gets two
// documents at once, its page lists both with their names; the edit form
// reorders them, refuses more than the field's limit, and deletes one
// while adding another.
func TestNodeSeveralFiles(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSite(t, db, nil)
		s.signIn()
		q := func(query string, args ...any) *sqlRow {
			return &sqlRow{t: t, row: db.DB.QueryRow(db.Dialect.Rebind(query), args...)}
		}

		var typeID int
		q("SELECT id FROM content_types WHERE machine_name = ?", "article").scan(&typeID)
		fields := fmt.Sprintf("/admin/config/content-types/%d/fields", typeID)
		if resp := s.post(fields, url.Values{"csrf_token": {s.csrf(fields)}, "label": {"Documents"}, "machine_name": {"documents"},
			"field_type": {"file"}, "multiple": {"1"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("add the field: %d", resp.StatusCode)
		}
		var fieldID int
		q("SELECT fi.id FROM field_instances fi JOIN field_storages fs ON fs.id = fi.field_storage_id WHERE fs.machine_name = ?", "field_documents").scan(&fieldID)
		if _, err := db.DB.Exec(db.Dialect.Rebind("UPDATE field_instances SET config = ? WHERE id = ?"), `{"max_values": 3}`, fieldID); err != nil {
			t.Fatal(err)
		}
		field := "field_" + strconv.Itoa(fieldID)

		documents := func(names ...string) []upload {
			var out []upload
			for _, name := range names {
				out = append(out, upload{field, name, "%PDF-1.4\n% " + name + "\n"})
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
			return url.Values{"csrf_token": {s.csrf(path)}, "title": {"Price list"}, body[1]: {"<p>Body</p>"}, "status": {"1"}}
		}
		files := func(nodeID int) []int {
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

		if resp := s.postUploads("/create/article", form("/create/article"), documents("prices.pdf", "terms.pdf")); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		var nodeID int
		var slug string
		q("SELECT id, slug FROM nodes WHERE title = ?", "Price list").scan(&nodeID, &slug)
		first := files(nodeID)
		if len(first) != 2 {
			t.Fatalf("saved files: %v", first)
		}
		if _, page := s.get("/" + slug); !strings.Contains(page, `class="field-files"`) || !strings.Contains(page, ">prices.pdf</a>") || !strings.Contains(page, ">terms.pdf</a>") {
			t.Errorf("the page doesn't list both files")
		}

		edit := fmt.Sprintf("/node/%d/edit", nodeID)
		if _, page := s.get(edit); !strings.Contains(page, fmt.Sprintf(`name="%s_keep" value="%d"`, field, first[1])) {
			t.Errorf("the edit form doesn't keep the files in rows")
		}
		save := func(extra url.Values, docs []upload) {
			t.Helper()
			v := form(edit)
			for k, vs := range extra {
				v[k] = vs
			}
			s.postUploads(edit, v, docs)
		}
		reversed := []string{strconv.Itoa(first[1]), strconv.Itoa(first[0])}
		save(url.Values{field + "_keep": reversed}, nil)
		if got := files(nodeID); !slices.Equal(got, []int{first[1], first[0]}) {
			t.Errorf("reordered: %v, want %v", got, []int{first[1], first[0]})
		}
		// Two more would be four of at most three: refused, nothing changes.
		save(url.Values{field + "_keep": reversed}, documents("a.pdf", "b.pdf"))
		if got := files(nodeID); len(got) != 2 {
			t.Errorf("over the limit: %v", got)
		}
		// One deleted, one added.
		save(url.Values{field + "_keep": reversed, field + "_delete": {strconv.Itoa(first[0])}}, documents("annex.pdf"))
		got := files(nodeID)
		if len(got) != 2 || got[0] != first[1] || got[1] == first[0] {
			t.Errorf("one deleted, one added: %v", got)
		}
		var name string
		q("SELECT original_filename FROM files WHERE id = ?", got[len(got)-1]).scan(&name)
		if name != "annex.pdf" {
			t.Errorf("the added file's name: %q", name)
		}
		s.checkLog()
	})
}
