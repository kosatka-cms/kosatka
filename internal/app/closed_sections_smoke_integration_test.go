//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestClosedSectionCachedBlocks - a type closed to guests: a cached View
// block in the sidebar leaves its content out for a guest, and shows it
// to a signed-in member the type is for and to the administrator.
func TestClosedSectionCachedBlocks(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := db.DB.Exec(db.Dialect.Rebind(q), args...); err != nil {
				t.Fatal(err)
			}
		}
		exec(`UPDATE content_types SET default_publishing = 'published' WHERE machine_name = 'article'`)
		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		v := url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {"Members news"}, body[1]: {"<p>Body</p>"}, "status": {"1"}}
		if resp := s.postMultipart("/create/article", v); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d\n%s", resp.StatusCode, s.log.String())
		}

		// The front page feed (a block display) in the sidebar.
		var display int
		if err := db.DB.QueryRow(`SELECT id FROM view_displays WHERE machine_name = 'frontpage'`).Scan(&display); err != nil {
			t.Fatal(err)
		}
		if resp := s.post("/admin/config/blocks/view", url.Values{"csrf_token": {s.csrf("/admin/config/blocks")}, "view_display_id": {fmt.Sprint(display)},
			"region": {"sidebar"}, "visibility_scope": {"all"}}); resp.StatusCode != http.StatusSeeOther || strings.Contains(resp.Header.Get("Location"), "error") {
			t.Fatalf("place the View block: %d %s", resp.StatusCode, resp.Header.Get("Location"))
		}

		// Not the front page: its feed is no cached block.
		const page = "/comments/recent"
		sidebar := func(as *site) bool {
			t.Helper()
			_, html := as.get(page)
			return strings.Contains(html, "Members news")
		}
		guest := s.visitor()
		// The placement rebuilds the blocks' cache in the background.
		for deadline := time.Now().Add(10 * time.Second); !sidebar(guest) && time.Now().Before(deadline); {
			time.Sleep(200 * time.Millisecond)
		}
		if !sidebar(guest) {
			_, html := guest.get(page)
			t.Fatalf("the open article isn't in the block:\n%s", html)
		}

		// Articles for signed-in users only.
		var typeID int
		if err := db.DB.QueryRow(`SELECT id FROM content_types WHERE machine_name = 'article'`).Scan(&typeID); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit(fmt.Sprintf("/admin/config/content-types/%d", typeID), "", url.Values{"view_roles": {entity.RoleUser}}); !ok {
			t.Fatal("close the type")
		}
		var roles string
		if err := db.DB.QueryRow(`SELECT view_roles FROM content_types WHERE machine_name = 'article'`).Scan(&roles); err != nil || roles != entity.RoleUser {
			t.Fatalf("view roles %q %v", roles, err)
		}

		var roleID int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id FROM roles WHERE name = ?`), entity.RoleUser).Scan(&roleID); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit("/admin/people/create", "/admin/people/create", url.Values{"password_mode": {"set"}, "username": {"member"}, "email": {"member@example.com"},
			"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {fmt.Sprint(roleID)}}); !ok {
			t.Fatal("create member")
		}
		member := s.visitor()
		member.signInAs("member", memberPassword)

		// The type's save rebuilds the blocks' cache in the background.
		for deadline := time.Now().Add(10 * time.Second); sidebar(guest) && time.Now().Before(deadline); {
			time.Sleep(200 * time.Millisecond)
		}
		if sidebar(guest) {
			t.Error("a guest sees the closed article in the cached block")
		}
		if !sidebar(member) {
			t.Error("a member doesn't see the article in the cached block")
		}
		if !sidebar(s) {
			t.Error("the administrator doesn't see the article in the cached block")
		}
		if sidebar(guest) {
			t.Error("a guest sees the closed article after a member's visit")
		}
	})
}

// TestItemViewRoles - an article given to signed-in users in its form:
// a guest finds neither its page nor it in the lists, search and sitemap;
// a member opens it; its form shows the roles checked, and a save without
// the section (an editor who may not administer content) keeps them.
func TestItemViewRoles(t *testing.T) {
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
		if !strings.Contains(form, `name="view_roles" value="`+entity.RoleUser+`"`) {
			t.Fatal("the form has no view roles")
		}
		create := func(title string, extra url.Values) {
			t.Helper()
			v := url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {title}, body[1]: {"<p>Body</p>"}, "status": {"1"}}
			for k, vs := range extra {
				v[k] = vs
			}
			if resp := s.postMultipart("/create/article", v); resp.StatusCode != http.StatusFound {
				t.Fatalf("create %q: %d\n%s", title, resp.StatusCode, s.log.String())
			}
		}
		create("Open note", nil)
		create("Members note", url.Values{"view_roles_shown": {"1"}, "view_roles": {entity.RoleUser, "Bad Role"}})
		var id int
		var slug, roles string
		if err := db.DB.QueryRow(`SELECT id, slug, view_roles FROM nodes WHERE title = 'Members note'`).Scan(&id, &slug, &roles); err != nil || roles != entity.RoleUser {
			t.Fatalf("stored view roles %q %v", roles, err)
		}

		var roleID int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id FROM roles WHERE name = ?`), entity.RoleUser).Scan(&roleID); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit("/admin/people/create", "/admin/people/create", url.Values{"password_mode": {"set"}, "username": {"member"}, "email": {"member@example.com"},
			"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {fmt.Sprint(roleID)}}); !ok {
			t.Fatal("create member")
		}
		member := s.visitor()
		member.signInAs("member", memberPassword)
		guest := s.visitor()

		// Who views what is read again within a few seconds of a change.
		var code int
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			if code, _ = guest.get("/" + slug); code == http.StatusNotFound {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if code != http.StatusNotFound {
			t.Errorf("a guest opens the members' note: %d", code)
		}
		if code, _ := member.get("/" + slug); code != http.StatusOK {
			t.Errorf("a member opens the members' note: %d", code)
		}
		for _, page := range []string{"/", "/api/search?q=note", "/sitemap.xml", "/rss.xml"} {
			if _, html := guest.get(page); strings.Contains(html, "Members note") || strings.Contains(html, slug) {
				t.Errorf("a guest finds the members' note on %s", page)
			}
		}
		if _, html := guest.get("/"); !strings.Contains(html, "Open note") {
			t.Error("a guest doesn't find the open note on the front page")
		}
		if _, html := member.get("/"); !strings.Contains(html, "Members note") {
			t.Error("a member doesn't find the members' note on the front page")
		}

		edit := fmt.Sprintf("/node/%d/edit", id)
		if _, html := s.get(edit); !strings.Contains(html, `value="`+entity.RoleUser+`" id="view-role-`+entity.RoleUser+`" checked`) {
			t.Error("the edit form doesn't show the role checked")
		}
		// A save without the section keeps the roles.
		if resp := s.postMultipart(edit, url.Values{"csrf_token": {s.csrf(edit)}, "title": {"Members note"}, body[1]: {"<p>Body 2</p>"}, "status": {"1"}}); resp.StatusCode >= 400 {
			t.Fatalf("save: %d", resp.StatusCode)
		}
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT view_roles FROM nodes WHERE id = ?`), id).Scan(&roles); err != nil || roles != entity.RoleUser {
			t.Errorf("after a save without the section: %q %v", roles, err)
		}
		// Unchecked: open again.
		if resp := s.postMultipart(edit, url.Values{"csrf_token": {s.csrf(edit)}, "title": {"Members note"}, body[1]: {"<p>Body 3</p>"}, "status": {"1"}, "view_roles_shown": {"1"}}); resp.StatusCode >= 400 {
			t.Fatalf("save: %d", resp.StatusCode)
		}
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT view_roles FROM nodes WHERE id = ?`), id).Scan(&roles); err != nil || roles != "" {
			t.Errorf("after unchecking: %q %v", roles, err)
		}
	})
}

// TestClosedSectionThemePreview - a previewed theme's blocks are made for
// the previewer: the administrator sees a closed type's article in the
// previewed theme's cached View block.
func TestClosedSectionThemePreview(t *testing.T) {
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
		if resp := s.postMultipart("/create/article", url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {"Members news"}, body[1]: {"<p>Body</p>"}, "status": {"1"}}); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		var display, typeID int
		if err := db.DB.QueryRow(`SELECT id FROM view_displays WHERE machine_name = 'frontpage'`).Scan(&display); err != nil {
			t.Fatal(err)
		}
		if resp := s.post("/admin/config/blocks/view", url.Values{"csrf_token": {s.csrf("/admin/config/blocks")}, "view_display_id": {fmt.Sprint(display)},
			"region": {"sidebar"}, "visibility_scope": {"all"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("place the View block: %d", resp.StatusCode)
		}
		if err := db.DB.QueryRow(`SELECT id FROM content_types WHERE machine_name = 'article'`).Scan(&typeID); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit(fmt.Sprintf("/admin/config/content-types/%d", typeID), "", url.Values{"view_roles": {entity.RoleUser}}); !ok {
			t.Fatal("close the type")
		}

		// The base theme active, kosatka (where the block is) previewed.
		token := s.csrf("/admin/config/appearance")
		if resp := s.post("/admin/config/appearance", url.Values{"theme": {""}, "csrf_token": {token}}); !strings.Contains(resp.Header.Get("Location"), "saved=1") {
			t.Fatalf("choosing the base theme: %s", resp.Header.Get("Location"))
		}
		if resp := s.post("/admin/config/appearance/preview", url.Values{"theme": {"kosatka"}, "csrf_token": {token}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("start the preview: %d", resp.StatusCode)
		}
		_, page := s.get("/comments/recent")
		if !strings.Contains(page, "/static/themes/kosatka/") || !strings.Contains(page, "admin-top-bar-preview") {
			t.Fatal("not the previewed theme")
		}
		if !strings.Contains(page, "Members news") {
			t.Error("the administrator doesn't see the closed article in the previewed theme's block")
		}
	})
}
