//go:build dbintegration

package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestAdminFormsResave - the administrator opens an edit form and saves it
// as is, twice: every save succeeds (no error redirect, no error page) and
// the server logs no error. Saving unchanged values is what an editor does
// most and what broke on MySQL, whose UPDATE reports only the rows it
// changed (a type's gallery settings saved again hit a duplicate key).
func TestAdminFormsResave(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()

		// Content to edit: an article (the install creates none).
		_, create := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(create)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		if resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")},
			"title": {"Resave test"}, body[1]: {"<p>Body</p>"}, "status": {"1"}}); resp.StatusCode != http.StatusFound {
			t.Fatalf("create article: %d\n%s", resp.StatusCode, s.log.String())
		}
		var nodeID int
		if err := db.DB.QueryRow("SELECT id FROM nodes WHERE title = 'Resave test'").Scan(&nodeID); err != nil {
			t.Fatal(err)
		}

		// A system block and a View block placed too: with the menu the
		// install placed, every kind of placement's edit form is saved.
		for _, action := range []string{"/admin/config/blocks/plugin", "/admin/config/blocks/view"} {
			if _, ok := s.submit("/admin/config/blocks", action, nil); !ok {
				t.Fatalf("place a block through %s", action)
			}
		}

		pages := []string{
			fmt.Sprintf("/node/%d/edit", nodeID),
			"/user/1/edit",
			"/admin/config/system/site-information",
			"/admin/config/account",
			"/admin/config/appearance",
		}
		// The edit forms the admin lists link to.
		for list, link := range map[string]*regexp.Regexp{
			"/admin/config/content-types": regexp.MustCompile(`href="(/admin/config/content-types/\d+)"`),
			"/admin/config/views":         regexp.MustCompile(`href="(/admin/config/views/\d+)"`),
			"/admin/config/blocks":        regexp.MustCompile(`href="(/admin/config/blocks/(?:[a-z]+/)?\d+/edit)"`),
			"/admin/config/menus":         regexp.MustCompile(`href="(/admin/config/menus/\d+/edit)"`),
			"/admin/config/text-formats":  regexp.MustCompile(`href="(/admin/config/text-formats/\d+/edit)"`),
			"/admin/config/date-formats":  regexp.MustCompile(`href="(/admin/config/date-formats/\d+)"`),
			"/admin/config/image-styles":  regexp.MustCompile(`href="(/admin/config/image-styles/\d+/edit)"`),
			"/admin/config/seo":           regexp.MustCompile(`href="(/admin/config/seo/types/\d+/edit)"`),
			"/admin/config/search":        regexp.MustCompile(`href="(/admin/config/search/types/[a-z_]+/edit)"`),
		} {
			_, page := s.get(list)
			found := link.FindAllStringSubmatch(page, -1)
			// A new site has types, Views, blocks and menus; search may
			// list no type yet.
			if len(found) == 0 && list != "/admin/config/search" {
				t.Errorf("%s links to no edit form", list)
			}
			for _, m := range found {
				if !slices.Contains(pages, m[1]) {
					pages = append(pages, m[1])
				}
			}
		}

		for _, page := range pages {
			for round := 1; round <= 2; round++ {
				s.resave(page, round)
			}
		}
		s.checkLog()
	})
}

// TestAdminCreateEditDelete - what an editor sets up in the admin goes
// through the admin's own forms: a content type and a field of a type, a
// menu and a menu link, a vocabulary with a term, a View, a block, an
// image style, a text format, an SEO and a breadcrumb
// rule, a user, a role and a redirect are each created, saved twice
// unchanged (where they have an edit form) and deleted; the server logs
// no error.
func TestAdminCreateEditDelete(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		id := func(q string, args ...any) int {
			t.Helper()
			var n int
			if err := db.DB.QueryRow(db.Dialect.Rebind(q), args...).Scan(&n); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
			return n
		}
		gone := func(what, table string, n int) {
			t.Helper()
			if left := id("SELECT COUNT(*) FROM "+table+" WHERE id = ?", n); left != 0 {
				t.Errorf("the deleted %s is still there", what)
			}
		}
		lifecycle := func(what, list string, set url.Values, idQuery, edit, del, table string) {
			t.Helper()
			if _, ok := s.submit(list, list, set); !ok {
				t.Fatalf("create %s", what)
			}
			n := id(idQuery)
			editPage := fmt.Sprintf(edit, n)
			for round := 1; round <= 2; round++ {
				s.resave(editPage, round)
			}
			delPage := fmt.Sprintf(del, n)
			if _, ok := s.submit(delPage, delPage, nil); !ok {
				t.Errorf("delete %s", what)
			}
			gone(what, table, n)
		}

		// A content type (deleted from the list: it has no confirmation
		// page).
		const types = "/admin/config/content-types"
		if _, ok := s.submit(types, types, url.Values{"label": {"Recipe"}, "machine_name": {"recipe"}}); !ok {
			t.Fatal("create content type")
		}
		typeID := id("SELECT id FROM content_types WHERE machine_name = 'recipe'")
		typePage := fmt.Sprintf("%s/%d", types, typeID)
		for round := 1; round <= 2; round++ {
			s.resave(typePage, round)
		}
		if _, ok := s.submit(types, typePage+"/delete", nil); !ok {
			t.Error("delete content type")
		}
		gone("content type", "content_types", typeID)

		lifecycle("menu", "/admin/config/menus", url.Values{"label": {"Footer"}},
			"SELECT id FROM menus WHERE label = 'Footer'", "/admin/config/menus/%d/edit", "/admin/config/menus/%d/delete", "menus")
		lifecycle("View", "/admin/config/views", url.Values{"label": {"Recipes"}, "url_path": {"recipes-list"}},
			"SELECT id FROM view_displays WHERE label = 'Recipes'", "/admin/config/views/%d", "/admin/config/views/%d/delete", "view_displays")
		lifecycle("block", "/admin/config/blocks", url.Values{"title": {"Notice"}, "body": {"<p>Closed on Monday</p>"}},
			"SELECT id FROM blocks WHERE title = 'Notice'", "/admin/config/blocks/%d/edit", "/admin/config/blocks/%d/delete", "blocks")

		// A vocabulary and its term (the term goes first: a vocabulary
		// with terms isn't deleted).
		const vocabs = "/admin/config/taxonomy"
		if _, ok := s.submit(vocabs, vocabs, url.Values{"name": {"Tags"}, "slug": {"tags"}}); !ok {
			t.Fatal("create vocabulary")
		}
		vid := id("SELECT id FROM taxonomy_vocabulary WHERE slug = 'tags'")
		lifecycle("term", fmt.Sprintf("%s/%d/terms", vocabs, vid), url.Values{"name": {"Cooking"}, "slug": {"cooking"}},
			"SELECT id FROM taxonomy_term_data WHERE slug = 'cooking'",
			fmt.Sprintf("%s/%d/terms/%%d/edit", vocabs, vid), fmt.Sprintf("%s/%d/terms/%%d/delete", vocabs, vid), "taxonomy_term_data")
		vocabPage := fmt.Sprintf("%s/%d/edit", vocabs, vid)
		for round := 1; round <= 2; round++ {
			s.resave(vocabPage, round)
		}
		delVocab := fmt.Sprintf("%s/%d/delete", vocabs, vid)
		if _, ok := s.submit(delVocab, delVocab, nil); !ok {
			t.Error("delete vocabulary")
		}
		gone("vocabulary", "taxonomy_vocabulary", vid)

		article := id("SELECT id FROM content_types WHERE machine_name = 'article'")
		fields := fmt.Sprintf("%s/%d/fields", types, article)
		lifecycle("field", fields, url.Values{"label": {"Subtitle"}, "machine_name": {"subtitle"}},
			"SELECT fi.id FROM field_instances fi JOIN field_storages fs ON fs.id = fi.field_storage_id WHERE fs.machine_name LIKE '%subtitle'",
			fields+"/%d/edit", fields+"/%d/delete", "field_instances")

		// A field of another type through this type's URLs: not changed,
		// not deleted, not reordered.
		page := id("SELECT id FROM content_types WHERE machine_name = 'page'")
		pageBody := id("SELECT id FROM field_instances WHERE content_type_id = ?", page)
		label := func() int {
			return id("SELECT COUNT(*) FROM field_instances WHERE id = ? AND label = 'Hijacked'", pageBody)
		}
		token := s.csrf(fields)
		foreign := fmt.Sprintf("%s/%d", fields, pageBody)
		if resp := s.post(foreign, url.Values{"csrf_token": {token}, "label": {"Hijacked"}}); resp.StatusCode != http.StatusNotFound || label() != 0 {
			t.Errorf("update of another type's field: %d, changed %d", resp.StatusCode, label())
		}
		if resp := s.post(foreign+"/delete", url.Values{"csrf_token": {token}}); resp.StatusCode != http.StatusNotFound {
			t.Errorf("delete of another type's field: %d", resp.StatusCode)
		}
		if id("SELECT COUNT(*) FROM field_instances WHERE id = ?", pageBody) != 1 {
			t.Error("another type's field was deleted")
		}
		for _, form := range []string{"/reorder", "/display-detail"} {
			resp := s.post(fields+form, url.Values{"csrf_token": {token}, "order": {strconv.Itoa(pageBody)}})
			if where := resp.Header.Get("Location"); !strings.Contains(where, "error=invalid") {
				t.Errorf("%s with another type's field: %d %q", form, resp.StatusCode, where)
			}
		}

		// A list field goes to several values and back on its own page.
		if _, ok := s.submit(fields, fields, url.Values{"label": {"Color"}, "machine_name": {"color"}, "field_type": {"list"}}); !ok {
			t.Fatal("create a list field")
		}
		color := id("SELECT fi.id FROM field_instances fi JOIN field_storages fs ON fs.id = fi.field_storage_id WHERE fs.machine_name LIKE '%color'")
		colorPage := fmt.Sprintf("%s/%d", fields, color)
		if _, ok := s.submit(colorPage+"/edit", "", url.Values{"allowed_values": {"red|Red\nblue|Blue"}}); !ok {
			t.Fatal("save the list field's values")
		}
		multiple := func() int {
			t.Helper()
			var raw []byte
			if err := db.DB.QueryRow(db.Dialect.Rebind("SELECT fs.config FROM field_storages fs JOIN field_instances fi ON fi.field_storage_id = fs.id WHERE fi.id = ?"), color).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var config map[string]any
			if err := json.Unmarshal(raw, &config); err != nil {
				t.Fatal(err)
			}
			if config["multiple"] == true {
				return 1
			}
			return 0
		}
		for _, want := range []int{1, 0} {
			if where, ok := s.submit(colorPage+"/cardinality", colorPage+"/cardinality", nil); !ok || !strings.Contains(where, "cardinality=1") {
				t.Errorf("change the number of values (to %d): %q %v", want, where, ok)
			}
			if got := multiple(); got != want {
				t.Errorf("multiple after the change: %d, want %d", got, want)
			}
		}
		mainMenu := fmt.Sprintf("/admin/config/menus/%d/links", id("SELECT MIN(id) FROM menus"))
		lifecycle("menu link", mainMenu, url.Values{"title": {"Docs"}, "link_type": {"url"}, "link_url": {"/docs"}},
			"SELECT id FROM menu_links WHERE title = 'Docs'", mainMenu+"/%d/edit", mainMenu+"/%d/delete", "menu_links")
		lifecycle("image style", "/admin/config/image-styles", url.Values{"name": {"teaser_test"}, "label": {"Teaser test"}, "width": {"320"}},
			"SELECT id FROM image_styles WHERE name = 'teaser_test'", "/admin/config/image-styles/%d/edit", "/admin/config/image-styles/%d/delete", "image_styles")
		lifecycle("text format", "/admin/config/text-formats", url.Values{"label": {"Plain test"}},
			"SELECT id FROM text_formats WHERE label = 'Plain test'", "/admin/config/text-formats/%d/edit", "/admin/config/text-formats/%d/delete", "text_formats")
		const seo = "/admin/config/seo"
		if _, ok := s.submit(seo, seo+"/rules", url.Values{"content_type": {"article"}, "title_template": {"[node:title] | [site:name]"}}); !ok {
			t.Fatal("create SEO rule")
		}
		rule := id("SELECT MAX(id) FROM seo_rules")
		for round := 1; round <= 2; round++ {
			s.resave(fmt.Sprintf("%s/rules/%d/edit", seo, rule), round)
		}
		if del := fmt.Sprintf("%s/rules/%d/delete", seo, rule); !submitted(s, del, del) {
			t.Error("delete SEO rule")
		}
		gone("SEO rule", "seo_rules", rule)
		lifecycle("breadcrumb rule", "/admin/config/breadcrumbs", url.Values{"content_type": {"article"},
			"menu_link_id": {fmt.Sprint(id("SELECT MIN(id) FROM menu_links"))}},
			"SELECT MAX(id) FROM breadcrumb_rules", "/admin/config/breadcrumbs/%d/edit", "/admin/config/breadcrumbs/%d/delete", "breadcrumb_rules")

		// A user: created by the administrator, edited, deleted (the
		// people list's bulk action).
		if _, ok := s.submit("/admin/users/create", "/admin/users/create", url.Values{"password_mode": {"set"}, "username": {"carol"}, "email": {"carol@example.com"},
			"password": {"carol-password-1234"}, "password_confirm": {"carol-password-1234"}, "status": {"1"},
			"role_id": {fmt.Sprint(id("SELECT id FROM roles WHERE name = ?", entity.RoleUser))}}); !ok {
			t.Fatal("create user")
		}
		carol := id("SELECT id FROM users WHERE username = 'carol'")
		for round := 1; round <= 2; round++ {
			s.resave(fmt.Sprintf("/user/%d/edit", carol), round)
		}
		req, err := http.NewRequest(http.MethodPost, s.base+"/api/admin/bulk/users", strings.NewReader(fmt.Sprintf(`{"action":"delete","ids":[%d]}`, carol)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", s.csrf("/admin/users"))
		if resp, err := s.client.Do(req); err != nil || resp.StatusCode >= 400 {
			t.Errorf("delete user: %v %v", resp, err)
		} else {
			_ = resp.Body.Close()
		}
		gone("user", "users", carol)

		// A role and a redirect: their forms are on their lists.
		const roles = "/admin/users/roles"
		if _, ok := s.submit(roles, roles, url.Values{"name": {"editor_test"}, "label": {"Editor test"}}); !ok {
			t.Fatal("create role")
		}
		role := id("SELECT id FROM roles WHERE name = 'editor_test'")
		if !submitted(s, roles, fmt.Sprintf("%s/%d/rename", roles, role)) {
			t.Error("rename role unchanged")
		}
		if !submitted(s, roles, fmt.Sprintf("%s/%d/delete", roles, role)) {
			t.Error("delete role")
		}
		gone("role", "roles", role)
		const redirects = "/admin/config/redirects"
		if _, ok := s.submit(redirects, redirects, url.Values{"from": {"/old-page"}, "to": {"/new-page"}}); !ok {
			t.Fatal("create redirect")
		}
		redirect := id("SELECT id FROM redirects WHERE from_path LIKE '%old-page'")
		if !submitted(s, redirects, fmt.Sprintf("%s/delete/%d", redirects, redirect)) {
			t.Error("delete redirect")
		}
		gone("redirect", "redirects", redirect)

		s.checkLog()
	})
}

// submitted - page's form posting to action, sent unchanged, was accepted.
func submitted(s *site, page, action string) bool {
	s.t.Helper()
	_, ok := s.submit(page, action, nil)
	return ok
}

// resave submits the edit form of page as the browser would, without
// changing anything, and fails unless the save is accepted.
func (s *site) resave(page string, round int) {
	s.t.Helper()
	if _, ok := s.submit(page, "", nil); !ok {
		s.t.Errorf("save %d of %s failed", round, page)
	}
}

// submit sends a form of page as the browser would, with set's values in
// place of the form's: the form posting to action, or (action "") the
// page's edit form (editForm). ok - the server accepted it: a redirect
// that isn't to an error; where is that redirect.
func (s *site) submit(page, action string, set url.Values) (where string, ok bool) {
	s.t.Helper()
	code, body := s.get(page)
	if code != http.StatusOK {
		s.t.Errorf("GET %s: %d", page, code)
		return "", false
	}
	var f form
	if action == "" {
		f, ok = editForm(body, page)
	} else {
		f, ok = formPostingTo(body, page, action)
	}
	if !ok {
		s.t.Errorf("%s has no form posting to %q", page, action)
		return "", false
	}
	for k, v := range set {
		f.values[k] = v
	}
	// The page's script adds the CSRF token to every POST form.
	if f.values.Get("csrf_token") == "" {
		if m := csrfMeta.FindStringSubmatch(body); m != nil {
			f.values.Set("csrf_token", m[1])
		}
	}
	var resp *http.Response
	if f.multipart {
		resp = s.postMultipart(f.action, f.values)
	} else {
		resp = s.post(f.action, f.values)
	}
	where = resp.Header.Get("Location")
	if resp.StatusCode >= 400 || resp.StatusCode == http.StatusOK || strings.Contains(where, "error") {
		s.t.Errorf("POST %s from %s: %d → %q", f.action, page, resp.StatusCode, where)
		return where, false
	}
	return where, true
}

// form - what a browser sends for a form: where, how, and the values.
type form struct {
	action    string
	multipart bool
	values    url.Values
}

// editForm - the page's form that saves it: the first POST form posting
// to the page itself (an empty action or the page's path), else the first
// POST form whose action isn't a delete or a logout.
func editForm(page, path string) (form, bool) {
	if f, ok := pickForm(page, path, func(a string) bool { return a == path }); ok {
		return f, true
	}
	return pickForm(page, path, func(a string) bool {
		return strings.HasPrefix(a, "/") && !strings.Contains(a, "delete") && !strings.Contains(a, "logout")
	})
}

// formPostingTo - the page's first POST form whose action is action.
func formPostingTo(page, path, action string) (form, bool) {
	return pickForm(page, path, func(a string) bool { return a == action })
}

// pickForm - the page's first POST form whose action (an empty one is
// the page's path) matches.
func pickForm(page, path string, match func(action string) bool) (form, bool) {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return form{}, false
	}
	var forms []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "form" && strings.EqualFold(attr(n, "method"), "post") {
			forms = append(forms, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	for _, n := range forms {
		action := attr(n, "action")
		if action == "" {
			action = path
		}
		if match(action) {
			return form{action: action, multipart: attr(n, "enctype") == "multipart/form-data", values: formValues(n)}, true
		}
	}
	return form{}, false
}

// formValues - the values a browser submits for the form: enabled named
// fields, checked boxes only, a select's selected options (else its
// first), and the first named submit button (Enter submits with it).
func formValues(f *html.Node) url.Values {
	v := url.Values{}
	submitted := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			name := attr(n, "name")
			_, disabled := attrOK(n, "disabled")
			if name != "" && !disabled {
				switch n.Data {
				case "input":
					switch strings.ToLower(attr(n, "type")) {
					case "checkbox", "radio":
						if _, checked := attrOK(n, "checked"); checked {
							value, ok := attrOK(n, "value")
							if !ok {
								value = "on"
							}
							v.Add(name, value)
						}
					case "submit", "image":
						if !submitted {
							v.Add(name, attr(n, "value"))
							submitted = true
						}
					case "file", "button", "reset":
					default:
						v.Add(name, attr(n, "value"))
					}
				case "button":
					if t := strings.ToLower(attr(n, "type")); (t == "" || t == "submit") && !submitted {
						v.Add(name, attr(n, "value"))
						submitted = true
					}
				case "textarea":
					v.Add(name, text(n))
				case "select":
					var options []*html.Node
					var collect func(*html.Node)
					collect = func(c *html.Node) {
						if c.Type == html.ElementNode && c.Data == "option" {
							options = append(options, c)
						}
						for d := c.FirstChild; d != nil; d = d.NextSibling {
							collect(d)
						}
					}
					collect(n)
					chosen := 0
					for _, o := range options {
						if _, sel := attrOK(o, "selected"); sel {
							v.Add(name, optionValue(o))
							chosen++
						}
					}
					if _, multiple := attrOK(n, "multiple"); chosen == 0 && !multiple && len(options) > 0 {
						v.Add(name, optionValue(options[0]))
					}
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(f)
	return v
}

func optionValue(o *html.Node) string {
	if v, ok := attrOK(o, "value"); ok {
		return v
	}
	return strings.TrimSpace(text(o))
}

func attr(n *html.Node, key string) string {
	v, _ := attrOK(n, key)
	return v
}

func attrOK(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(c *html.Node) {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
		for d := c.FirstChild; d != nil; d = d.NextSibling {
			walk(d)
		}
	}
	walk(n)
	return b.String()
}

// settingsFormValues - a settings form's fields as the page fills them
// (inputs with a value, the checked boxes and radios, the selected
// options).
func settingsFormValues(page string) url.Values {
	out := url.Values{}
	for _, m := range regexp.MustCompile(`<input([^>]*)>`).FindAllStringSubmatch(page, -1) {
		attrs := m[1]
		name := attrValue(attrs, "name")
		if name == "" || name == "csrf_token" {
			continue
		}
		typ := attrValue(attrs, "type")
		if (typ == "checkbox" || typ == "radio") && !strings.Contains(attrs, " checked") {
			continue
		}
		out.Add(name, attrValue(attrs, "value"))
	}
	for _, m := range regexp.MustCompile(`(?s)<select[^>]*name="([^"]+)"[^>]*>(.*?)</select>`).FindAllStringSubmatch(page, -1) {
		if v := regexp.MustCompile(`<option value="([^"]*)"[^>]*selected`).FindStringSubmatch(m[2]); v != nil {
			out.Set(m[1], v[1])
		}
	}
	return out
}

// attrValue - attribute name's value in attrs ("" - none).
func attrValue(attrs, name string) string {
	m := regexp.MustCompile(`\s` + name + `="([^"]*)"`).FindStringSubmatch(attrs)
	if m == nil {
		return ""
	}
	return m[1]
}
