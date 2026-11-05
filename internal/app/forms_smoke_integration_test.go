//go:build dbintegration

package app

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestFormSubmissionsAccess - a form's submissions page, CSV and sent
// files: the administrator sees them, a signed-in user without "view form
// submissions" gets 403, a visitor is sent to the login page.
func TestFormSubmissionsAccess(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		hash, err := bcrypt.GenerateFromPassword([]byte(memberPassword), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		s := installSite(t, db, func() {
			// A plain member (the base role only).
			if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO users (username, slug, email, password_hash, status, email_verified, role_id)
				SELECT 'member', 'users/member', 'member@example.com', ?, 1, true, id FROM roles WHERE name = ?`), string(hash), entity.RoleUser); err != nil {
				t.Fatal(err)
			}
		})
		s.signIn()
		resp := s.post("/create/form", url.Values{
			"csrf_token": {s.csrf("/create/form")}, "title": {"Contact"}, "status": {"1"}, "comment_mode": {"hidden"}, "confirmation_url": {"/thanks"},
			"element_row": {"new1", "new2"}, "element_type_new1": {"text"}, "element_label_new1": {"Name"},
			"element_type_new2": {"file"}, "element_label_new2": {"CV"}, "element_ext_new2": {"txt"},
		})
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("create form: %d", resp.StatusCode)
		}
		var nid, slug string
		if err := db.DB.QueryRow("SELECT id, slug FROM nodes WHERE type = 'form'").Scan(&nid, &slug); err != nil {
			t.Fatal(err)
		}
		var nameID, fileID, sid string
		if err := db.DB.QueryRow("SELECT id FROM webform_elements WHERE type = 'text'").Scan(&nameID); err != nil {
			t.Fatal(err)
		}
		if err := db.DB.QueryRow("SELECT id FROM webform_elements WHERE type = 'file'").Scan(&fileID); err != nil {
			t.Fatal(err)
		}
		resp = s.postFiles("/node/"+nid+"/form", url.Values{"csrf_token": {s.csrf("/" + slug)}, "element_" + nameID: {"Ann"}},
			map[string][2]string{"element_" + fileID: {"my cv.txt", "Hello"}})
		if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/thanks" {
			t.Fatalf("submit with a file: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
		}
		if err := db.DB.QueryRow("SELECT id FROM webform_submissions").Scan(&sid); err != nil {
			t.Fatal(err)
		}
		file := "/node/" + nid + "/form/file?sid=" + sid + "&eid=" + fileID + "&d=0"
		pages := []string{"/node/" + nid + "/form/submissions", "/node/" + nid + "/form/submissions.csv", file, "/node/" + nid + "/form/analysis", "/node/" + nid + "/form/submissions/" + sid}
		for _, p := range pages {
			if code, _ := s.get(p); code != http.StatusOK {
				t.Errorf("administrator %s: %d", p, code)
			}
		}
		if _, body := s.get(file); body != "Hello" {
			t.Errorf("file: %q", body)
		}
		if _, body := s.get("/node/" + nid + "/form/submissions"); !strings.Contains(body, "my cv.txt") {
			t.Error("the submissions page has no file link")
		}

		as := func(who string) *site {
			jar, _ := cookiejar.New(nil)
			other := *s
			other.client = &http.Client{Jar: jar, Timeout: 30 * time.Second,
				CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			if who != "" {
				resp := other.post("/login", url.Values{"login": {who}, "pass": {memberPassword}, "csrf_token": {other.csrf("/login")}})
				if resp.StatusCode != http.StatusFound {
					t.Fatalf("sign-in %s: %d", who, resp.StatusCode)
				}
			}
			return &other
		}
		member, visitor := as("member"), as("")
		for _, p := range pages {
			if code, _ := member.get(p); code != http.StatusForbidden {
				t.Errorf("member %s: %d, want 403", p, code)
			}
			resp, err := visitor.client.Get(visitor.base + p)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusFound || !strings.HasPrefix(resp.Header.Get("Location"), "/login") {
				t.Errorf("visitor %s: %d → %q", p, resp.StatusCode, resp.Header.Get("Location"))
			}
		}

		// Submissions older than the given days go, with their files.
		if _, err := db.DB.Exec("UPDATE webform_submissions SET created_at = created_at - INTERVAL '40' DAY"); err != nil {
			t.Fatal(err)
		}
		resp = s.post("/node/"+nid+"/form/submissions/purge", url.Values{"csrf_token": {s.csrf("/node/" + nid + "/form/submissions")}, "days": {"30"}})
		if resp.StatusCode != http.StatusSeeOther || !strings.HasSuffix(resp.Header.Get("Location"), "deleted=1") {
			t.Errorf("purge: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
		}
		if code, _ := s.get(file); code != http.StatusNotFound {
			t.Errorf("file after purge: %d", code)
		}
		s.checkLog()
	})
}
