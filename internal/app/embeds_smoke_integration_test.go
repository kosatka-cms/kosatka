//go:build dbintegration

package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestEmbeds - a block gets a machine name from its title; an article's
// text in a format with "Embeds" on shows the block and a View block's
// list in place of their tags, the feed leaves the tags out; a block's
// name can be changed, not to another block's.
func TestEmbeds(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		if _, err := db.DB.Exec(`UPDATE content_types SET default_publishing = 'published' WHERE machine_name = 'article'`); err != nil {
			t.Fatal(err)
		}

		if resp := s.post("/admin/config/blocks", url.Values{"csrf_token": {s.csrf("/admin/config/blocks")}, "title": {"Footer contacts"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("create the block: %d", resp.StatusCode)
		}
		var blockID int
		var name string
		if err := db.DB.QueryRow(`SELECT id, machine_name FROM blocks WHERE title = 'Footer contacts'`).Scan(&blockID, &name); err != nil || name != "footer_contacts" {
			t.Fatalf("the block's name %q %v", name, err)
		}
		edit := fmt.Sprintf("/admin/config/blocks/%d/edit", blockID)
		if _, ok := s.submit(edit, "", url.Values{"body": {"<p>Call us at noon</p>"}, "machine_name": {"contacts"}}); !ok {
			t.Fatal("save the block")
		}
		if resp := s.post("/admin/config/blocks", url.Values{"csrf_token": {s.csrf("/admin/config/blocks")}, "title": {"Other"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("create another block: %d", resp.StatusCode)
		}
		var otherID int
		if err := db.DB.QueryRow(`SELECT id FROM blocks WHERE title = 'Other'`).Scan(&otherID); err != nil {
			t.Fatal(err)
		}
		otherEdit := fmt.Sprintf("/admin/config/blocks/%d/edit", otherID)
		if resp := s.post(fmt.Sprintf("/admin/config/blocks/%d", otherID), url.Values{"csrf_token": {s.csrf(otherEdit)}, "title": {"Other"}, "machine_name": {"contacts"}}); !strings.Contains(resp.Header.Get("Location"), "error=machine_name") {
			t.Errorf("another block's name taken: %s", resp.Header.Get("Location"))
		}

		// "Embeds" on for the article's format.
		var formatID int
		if err := db.DB.QueryRow(`SELECT id FROM text_formats WHERE machine_name = 'basic_html'`).Scan(&formatID); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit(fmt.Sprintf("/admin/config/text-formats/%d/edit", formatID), "", url.Values{"filters": {"embeds"}}); !ok {
			t.Fatal("turn the filter on")
		}

		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		create := func(title, text string) {
			t.Helper()
			v := url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {title}, body[1]: {text}, "status": {"1"}}
			if resp := s.postMultipart("/create/article", v); resp.StatusCode != http.StatusFound {
				t.Fatalf("create %q: %d\n%s", title, resp.StatusCode, s.log.String())
			}
		}
		create("Listed news", "<p>Plain news.</p>")
		v := url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {"Members news"}, body[1]: {"<p>For members.</p>"}, "status": {"1"},
			"view_roles_shown": {"1"}, "view_roles": {"user"}}
		if resp := s.postMultipart("/create/article", v); resp.StatusCode != http.StatusFound {
			t.Fatalf("create the members' news: %d", resp.StatusCode)
		}
		create("With embeds", "<p>Before.</p><p>[[block:contacts]]</p><p>[[view:frontpage]]</p><p>Missing [[block:nope]] here.</p>")
		var slug string
		if err := db.DB.QueryRow(`SELECT slug FROM nodes WHERE title = 'With embeds'`).Scan(&slug); err != nil {
			t.Fatal(err)
		}

		guest := s.visitor()
		var page string
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if _, page = guest.get("/" + slug); strings.Contains(page, "Call us at noon") {
				break
			}
		}
		for _, want := range []string{`class="block block-embed block-embed-contacts"`, "Call us at noon", "Listed news", "Missing  here."} {
			if !strings.Contains(page, want) {
				t.Errorf("the page has no %q", want)
			}
		}
		// The embedded list as each visitor may see it.
		if strings.Contains(page, "Members news") {
			t.Error("a guest sees the members' news in the embedded list")
		}
		if _, own := s.get("/" + slug); !strings.Contains(own, "Members news") {
			t.Error("the administrator doesn't see the members' news in the embedded list")
		}
		if strings.Contains(page, "[[") || strings.Contains(page, "kosatka-embed") {
			t.Error("a tag or a mark left on the page")
		}
		if _, feed := guest.get("/rss.xml"); strings.Contains(feed, "[[") {
			t.Error("the feed has a tag")
		}
	})
}

// TestModuleEmbeds - the modules' kinds: [[poll:<id>]] and [[form:<id>]]
// put the poll's and the form's place, loaded for each visitor, with the
// module's script; [[gallery:<id>]] shows the gallery to whom its content
// is open, nothing to the others; a missing one - nothing.
func TestModuleEmbeds(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := db.DB.Exec(db.Dialect.Rebind(q), args...); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
		exec(`UPDATE content_types SET default_publishing = 'published' WHERE machine_name = 'article'`)
		var formatID int
		if err := db.DB.QueryRow(`SELECT id FROM text_formats WHERE machine_name = 'basic_html'`).Scan(&formatID); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit(fmt.Sprintf("/admin/config/text-formats/%d/edit", formatID), "", url.Values{"filters": {"embeds"}}); !ok {
			t.Fatal("turn the filter on")
		}

		exec(`INSERT INTO nodes (id, type, title, slug, status, created_by) VALUES (900, 'poll', 'Best sea?', 'poll/best-sea', 1, 1)`)
		exec(`INSERT INTO polls (nid) VALUES (900)`)
		if resp := s.post("/create/form", url.Values{"csrf_token": {s.csrf("/create/form")}, "title": {"Contact"}, "status": {"1"}, "comment_mode": {"hidden"},
			"element_row": {"new1"}, "element_type_new1": {"text"}, "element_label_new1": {"Name"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("create the form: %d", resp.StatusCode)
		}
		var formID int
		if err := db.DB.QueryRow(`SELECT id FROM nodes WHERE type = 'form'`).Scan(&formID); err != nil {
			t.Fatal(err)
		}

		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		create := func(title, text string) (int, string) {
			t.Helper()
			v := url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {title}, body[1]: {text}, "status": {"1"}}
			if resp := s.postMultipart("/create/article", v); resp.StatusCode != http.StatusFound {
				t.Fatalf("create %q: %d\n%s", title, resp.StatusCode, s.log.String())
			}
			var id int
			var slug string
			if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id, slug FROM nodes WHERE title = ?`), title).Scan(&id, &slug); err != nil {
				t.Fatal(err)
			}
			return id, slug
		}
		holder, _ := create("Trip photos", "<p>The trip.</p>")
		exec(`INSERT INTO gallery_nodes (nid) VALUES (?)`, holder)
		exec(`INSERT INTO files (id, uri, mime_type, width, height, size_bytes) VALUES (700, 'public://sea.jpg', 'image/jpeg', 800, 600, 1000)`)
		exec(`INSERT INTO galleries (id, nid, title, status) VALUES (50, ?, 'Sea pictures', ?)`, holder, true)
		exec(`INSERT INTO gallery_items (gallery_id, fid, title) VALUES (50, 700, 'Waves')`)
		_, slug := create("All embeds", fmt.Sprintf("<p>[[poll:900]]</p><p>[[form:%d]]</p><p>[[gallery:50]]</p><p>[[gallery:51]]</p>", formID))

		guest := s.visitor()
		_, page := guest.get("/" + slug)
		for _, want := range []string{`data-poll-embed="/node/900/poll/embed"`, fmt.Sprintf(`data-webform-embed="/node/%d/form/embed"`, formID),
			"Sea pictures", `data-pswp-width="800"`, "polls.min.js", "forms.min.js"} {
			if !strings.Contains(page, want) {
				t.Errorf("the page has no %q", want)
			}
		}
		if strings.Contains(page, "[[") {
			t.Error("a tag left on the page")
		}
		if code, _ := guest.get("/node/900/poll/embed"); code != http.StatusOK {
			t.Errorf("the embedded poll's place: %d", code)
		}

		// The gallery's content closed to guests: nothing for a guest, the
		// gallery for the administrator.
		exec(`UPDATE nodes SET view_roles = 'user' WHERE id = ?`, holder)
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if _, page = guest.get("/" + slug); !strings.Contains(page, "Sea pictures") {
				break
			}
		}
		if strings.Contains(page, "Sea pictures") {
			t.Error("a guest sees the gallery of content closed to them")
		}
		if _, own := s.get("/" + slug); !strings.Contains(own, "Sea pictures") {
			t.Error("the administrator doesn't see the gallery")
		}
	})
}

// TestEmbedsPicker - the editor's "Insert" window: refused while no
// format has "Embeds" on; then the kinds, the things found by their
// title and their descriptions (the preview, the cards), a missing one
// left out; the content form's editor loads the button's script.
func TestEmbedsPicker(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		apiJSON := func(path string, into any) int {
			t.Helper()
			resp, err := s.client.Get(s.base + path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if into != nil && resp.StatusCode == http.StatusOK {
				if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
					t.Fatalf("%s: %v", path, err)
				}
			}
			return resp.StatusCode
		}
		if code := apiJSON("/api/embeds/kinds", nil); code != http.StatusForbidden {
			t.Errorf("kinds while no format embeds: %d", code)
		}

		var formatID int
		if err := db.DB.QueryRow(`SELECT id FROM text_formats WHERE machine_name = 'basic_html'`).Scan(&formatID); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit(fmt.Sprintf("/admin/config/text-formats/%d/edit", formatID), "", url.Values{"filters": {"embeds"}}); !ok {
			t.Fatal("turn the filter on")
		}
		if resp := s.post("/admin/config/blocks", url.Values{"csrf_token": {s.csrf("/admin/config/blocks")}, "title": {"Opening hours"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("create the block: %d", resp.StatusCode)
		}

		var kinds struct {
			Kinds []struct{ Name, Label string }
		}
		if code := apiJSON("/api/embeds/kinds", &kinds); code != http.StatusOK {
			t.Fatalf("kinds: %d", code)
		}
		var names []string
		for _, k := range kinds.Kinds {
			names = append(names, k.Name)
		}
		for _, want := range []string{"block", "view", "gallery", "poll", "form"} {
			if !slices.Contains(names, want) {
				t.Errorf("no kind %q in %v", want, names)
			}
		}

		var found struct {
			Options []struct{ Name, Title string }
		}
		if code := apiJSON("/api/embeds/search?kind=block&q=open", &found); code != http.StatusOK || len(found.Options) != 1 || found.Options[0].Name != "opening_hours" {
			t.Errorf("search: %d %+v", code, found)
		}
		if code := apiJSON("/api/embeds/search?kind=view&q=", &found); code != http.StatusOK || !slices.ContainsFunc(found.Options, func(o struct{ Name, Title string }) bool { return o.Name == "frontpage" }) {
			t.Errorf("search the Views: %d %+v", code, found)
		}
		var described struct {
			Items map[string]struct {
				Title string
				Lines []string
			}
		}
		if code := apiJSON("/api/embeds/describe?tag=block:opening_hours&tag=view:frontpage&tag=block:nope&tag=bad", &described); code != http.StatusOK {
			t.Fatalf("describe: %d", code)
		}
		if described.Items["block:opening_hours"].Title != "Opening hours" {
			t.Errorf("the block: %+v", described.Items)
		}
		if _, ok := described.Items["view:frontpage"]; !ok {
			t.Errorf("the View: %+v", described.Items)
		}
		if _, ok := described.Items["block:nope"]; ok || len(described.Items) != 2 {
			t.Errorf("a missing or invalid tag described: %+v", described.Items)
		}

		if _, form := s.get("/create/article"); !strings.Contains(form, "embeds_editor.min.js") || !strings.Contains(form, "kosatkaembed") {
			t.Error("the editor has no Insert button")
		}
		guest := s.visitor()
		if code, _ := guest.get("/api/embeds/kinds"); code != http.StatusUnauthorized && code != http.StatusForbidden {
			t.Errorf("a guest's kinds: %d", code)
		}
	})
}
