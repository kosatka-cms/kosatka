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

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestNodeFrontPageFlags - "on the front page" and "sticky": a new
// article takes its type's defaults (an article is promoted) unless an
// administrator sets them; a save without them keeps the node's own; a
// user who may not administer content can't set them, whatever the form
// says; the front page lists the promoted ones, the sticky first.
func TestNodeFrontPageFlags(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		_, form := s.get("/create/article")
		body := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(form)
		if body == nil {
			t.Fatal("no body field on /create/article")
		}
		create := func(as *site, title string, extra url.Values) {
			t.Helper()
			v := url.Values{"csrf_token": {as.csrf("/create/article")}, "title": {title}, body[1]: {"<p>Body</p>"}, "status": {"1"}}
			for k, vs := range extra {
				v[k] = vs
			}
			if resp := as.postMultipart("/create/article", v); resp.StatusCode != http.StatusFound {
				t.Fatalf("create %q: %d\n%s", title, resp.StatusCode, s.log.String())
			}
		}
		flags := func(title string) (id int, promote, sticky bool) {
			t.Helper()
			if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id, promote, sticky FROM nodes WHERE title = ?`), title).Scan(&id, &promote, &sticky); err != nil {
				t.Fatalf("%q: %v", title, err)
			}
			return id, promote, sticky
		}

		create(s, "Defaults", nil)
		if _, promote, sticky := flags("Defaults"); !promote || sticky {
			t.Errorf("an article by default: promote %v, sticky %v, want true, false", promote, sticky)
		}
		create(s, "Set by the admin", url.Values{"node_flags_shown": {"1"}, "sticky": {"1"}})
		if _, promote, sticky := flags("Set by the admin"); promote || !sticky {
			t.Errorf("set by the administrator: promote %v, sticky %v, want false, true", promote, sticky)
		}

		// An edit without the flags in the form keeps the node's own.
		id, _, _ := flags("Defaults")
		edit := fmt.Sprintf("/node/%d/edit", id)
		if resp := s.postMultipart(edit, url.Values{"csrf_token": {s.csrf(edit)}, "title": {"Defaults"}, body[1]: {"<p>Body 2</p>"}, "status": {"1"}}); resp.StatusCode >= 400 {
			t.Fatalf("edit: %d", resp.StatusCode)
		}
		if _, promote, sticky := flags("Defaults"); !promote || sticky {
			t.Errorf("after an edit without them: promote %v, sticky %v, want true, false", promote, sticky)
		}

		// A member who may create articles, not administer content.
		var roleID int
		var perms string
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT id, permissions FROM roles WHERE name = ?`), entity.RoleUser).Scan(&roleID, &perms); err != nil {
			t.Fatal(err)
		}
		granted := map[string]any{}
		_ = json.Unmarshal([]byte(perms), &granted)
		granted[service.PermCreate("article")] = true
		b, _ := json.Marshal(granted)
		if _, err := db.DB.Exec(db.Dialect.Rebind(`UPDATE roles SET permissions = ? WHERE id = ?`), string(b), roleID); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.submit("/admin/users/create", "/admin/users/create", url.Values{"password_mode": {"set"}, "username": {"member"}, "email": {"member@example.com"},
			"password": {memberPassword}, "password_confirm": {memberPassword}, "status": {"1"}, "role_id": {fmt.Sprint(roleID)}}); !ok {
			t.Fatal("create member")
		}
		// Published at once (the type's publishing; on moderation by
		// default, TestNodePublishing), so the front page lists it.
		if _, err := db.DB.Exec(`UPDATE content_types SET default_publishing = 'published' WHERE machine_name = 'article'`); err != nil {
			t.Fatal(err)
		}
		member := s.visitor()
		member.signInAs("member", memberPassword)
		create(member, "By a member", url.Values{"node_flags_shown": {"1"}, "sticky": {"1"},
			"path_auto_shown": {"1"}, "slug": {"articles"}, "meta_title": {"Taken over"}, "comment_mode": {"hidden"}})
		if _, promote, sticky := flags("By a member"); !promote || sticky {
			t.Errorf("a member's flags: promote %v, sticky %v, want the type's defaults (true, false)", promote, sticky)
		}
		// Nor the address, the meta tags or the comments' mode.
		var slug, mode string
		var seo int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT n.slug, n.comment_mode, (SELECT COUNT(*) FROM page_seo p WHERE p.nid = n.id) FROM nodes n WHERE n.title = ?`), "By a member").Scan(&slug, &mode, &seo); err != nil {
			t.Fatal(err)
		}
		if slug == "articles" || mode == "hidden" || seo != 0 {
			t.Errorf("a member's settings taken: slug %q, comment mode %q, SEO rows %d", slug, mode, seo)
		}

		// An administrator's typed address isn't an engine route's (or a
		// View's, a term's page): the node would hide it.
		v := url.Values{"csrf_token": {s.csrf("/create/article")}, "title": {"At the login"}, body[1]: {"<p>Body</p>"}, "status": {"1"},
			"path_auto_shown": {"1"}, "slug": {"login"}}
		resp := s.postMultipart("/create/article", v)
		var made int
		if err := db.DB.QueryRow(db.Dialect.Rebind(`SELECT COUNT(*) FROM nodes WHERE title = ?`), "At the login").Scan(&made); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK || made != 0 {
			t.Errorf("a typed address an engine route answers: %d, nodes made %d", resp.StatusCode, made)
		}

		// The front page: the promoted ones, the sticky first (made sticky
		// now, "Defaults" goes before the newer "By a member").
		if resp := s.postMultipart(edit, url.Values{"csrf_token": {s.csrf(edit)}, "title": {"Defaults"}, body[1]: {"<p>Body 2</p>"}, "status": {"1"},
			"node_flags_shown": {"1"}, "promote": {"1"}, "sticky": {"1"}}); resp.StatusCode >= 400 {
			t.Fatalf("make sticky: %d", resp.StatusCode)
		}
		_, home := s.get("/")
		first, newer := strings.Index(home, ">Defaults<"), strings.Index(home, ">By a member<")
		if first < 0 || newer < 0 || first > newer {
			t.Errorf("the front page: the sticky %d, the newer %d (want both, the sticky first)", first, newer)
		}
		if strings.Contains(home, ">Set by the admin<") {
			t.Error("the front page lists a node not promoted to it")
		}
	})
}
