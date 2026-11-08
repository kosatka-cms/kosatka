//go:build dbintegration

package app

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
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

// TestFormSentByScript - a form sent by forms.js (X-Requested-With) is
// answered with its part, not a page: the form again with the errors and
// what was typed (422), the confirmation, or - with a page after sending -
// its address in X-Webform-Redirect.
func TestFormSentByScript(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSite(t, db, nil)
		s.signIn()
		resp := s.post("/create/form", url.Values{
			"csrf_token": {s.csrf("/create/form")}, "title": {"Request"}, "status": {"1"}, "comment_mode": {"hidden"},
			"confirmation": {"We'll call you back."},
			"element_row":  {"new1", "new2"}, "element_type_new1": {"text"}, "element_label_new1": {"Name"}, "element_required_new1": {"1"},
			"element_type_new2": {"text"}, "element_label_new2": {"Company"},
		})
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("create form: %d", resp.StatusCode)
		}
		var nid, slug, nameID, companyID string
		if err := db.DB.QueryRow("SELECT id, slug FROM nodes WHERE type = 'form'").Scan(&nid, &slug); err != nil {
			t.Fatal(err)
		}
		if err := db.DB.QueryRow("SELECT id FROM webform_elements WHERE label = 'Name'").Scan(&nameID); err != nil {
			t.Fatal(err)
		}
		if err := db.DB.QueryRow("SELECT id FROM webform_elements WHERE label = 'Company'").Scan(&companyID); err != nil {
			t.Fatal(err)
		}
		token := s.csrf("/" + slug)
		// send - the form as forms.js sends it: urlencoded, or multipart
		// (a FormData) when asMultipart.
		asMultipart := false
		send := func(form url.Values) (int, http.Header, string) {
			t.Helper()
			form.Set("csrf_token", token)
			body, contentType := io.Reader(strings.NewReader(form.Encode())), "application/x-www-form-urlencoded"
			if asMultipart {
				var buf bytes.Buffer
				mw := multipart.NewWriter(&buf)
				for k, vs := range form {
					for _, v := range vs {
						_ = mw.WriteField(k, v)
					}
				}
				_ = mw.Close()
				body, contentType = &buf, mw.FormDataContentType()
			}
			req, err := http.NewRequest(http.MethodPost, s.base+"/node/"+nid+"/form", body)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", contentType)
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			// Each from its own address: the sending's rate limit isn't
			// what is tested.
			authAddr++
			req.Header.Set("X-Forwarded-For", "198.51.100."+strconv.Itoa(authAddr%250+1))
			resp, err := s.client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			b, _ := io.ReadAll(resp.Body)
			return resp.StatusCode, resp.Header, string(b)
		}

		code, _, body := send(url.Values{"element_" + companyID: {"Acme"}})
		if code != http.StatusUnprocessableEntity || !strings.Contains(body, "data-webform-body") || !strings.Contains(body, `value="Acme"`) ||
			!strings.Contains(body, "has-error") || strings.Contains(body, "<html") {
			t.Errorf("a missing answer: %d, %.300s", code, body)
		}
		code, _, body = send(url.Values{"element_" + nameID: {"Ann"}})
		if code != http.StatusOK || !strings.Contains(body, "webform-confirmation") || !strings.Contains(body, "We&#39;ll call you back.") || strings.Contains(body, "<html") {
			t.Errorf("sent: %d, %.300s", code, body)
		}
		// A FormData body for a form without a file is read too.
		asMultipart = true
		if code, _, body = send(url.Values{"element_" + nameID: {"Cid"}}); code != http.StatusOK || !strings.Contains(body, "webform-confirmation") {
			t.Errorf("sent as multipart: %d, %.300s", code, body)
		}
		asMultipart = false
		var sent int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM webform_submissions").Scan(&sent); err != nil || sent != 2 {
			t.Errorf("submissions: %d, %v", sent, err)
		}

		if _, err := db.DB.Exec(db.Dialect.Rebind("UPDATE webforms SET confirmation_url = ?"), "/thanks"); err != nil {
			t.Fatal(err)
		}
		if code, header, _ := send(url.Values{"element_" + nameID: {"Bob"}}); code != http.StatusOK || header.Get("X-Webform-Redirect") != "/thanks" {
			t.Errorf("with a page after sending: %d, %q", code, header.Get("X-Webform-Redirect"))
		}
	})
}

// TestFormPreviewOfForms - a form previewed from its editor: a new form's
// questions and an edited one's (a new question with a condition on a
// saved one) on the form's page, sending off, what saving would refuse in
// the strip; nothing saved.
func TestFormPreviewOfForms(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSite(t, db, nil)
		s.signIn()
		if _, page := s.get("/create/form"); !strings.Contains(page, `formaction="/create/form/preview"`) {
			t.Fatal("no Preview button on /create/form")
		}
		code, _, page := s.postRead("/create/form/preview", url.Values{
			"csrf_token": {s.csrf("/create/form")}, "title": {"Survey"}, "status": {"1"},
			"element_row": {"new1"}, "element_type_new1": {"text"}, "element_label_new1": {"Your city"},
		})
		if code != http.StatusOK || !strings.Contains(page, `class="unsaved-preview"`) || !strings.Contains(page, "Your city") ||
			!strings.Contains(page, "Sending is off in the preview.") || !strings.Contains(page, ` disabled>Send</button>`) {
			t.Errorf("a new form's preview: %d", code)
		}
		var forms int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE type = 'form'").Scan(&forms); err != nil || forms != 0 {
			t.Fatalf("forms after a preview: %d, %v", forms, err)
		}
		if _, _, page := s.postRead("/create/form/preview", url.Values{"csrf_token": {s.csrf("/create/form")}, "title": {""}}); !strings.Contains(page, "Saving would be refused:") {
			t.Error("a form saving would refuse: no errors in the strip")
		}

		if resp := s.post("/create/form", url.Values{
			"csrf_token": {s.csrf("/create/form")}, "title": {"Request"}, "status": {"1"}, "comment_mode": {"hidden"},
			"element_row": {"new1"}, "element_type_new1": {"radios"}, "element_label_new1": {"Topic"}, "element_options_new1": {"Ads\nErrors"},
		}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("create form: %d", resp.StatusCode)
		}
		var nid, topicID string
		if err := db.DB.QueryRow("SELECT id FROM nodes WHERE type = 'form'").Scan(&nid); err != nil {
			t.Fatal(err)
		}
		if err := db.DB.QueryRow("SELECT id FROM webform_elements WHERE label = 'Topic'").Scan(&topicID); err != nil {
			t.Fatal(err)
		}
		edit := "/node/" + nid + "/edit"
		if _, page := s.get(edit); !strings.Contains(page, `formaction="/node/`+nid+`/edit/preview"`) {
			t.Fatal("no Preview button on the form's editor")
		}
		_, _, page = s.postRead(edit+"/preview", url.Values{
			"csrf_token": {s.csrf(edit)}, "title": {"Request"}, "status": {"1"},
			"element_row": {topicID, "new1"}, "element_type_" + topicID: {"radios"}, "element_label_" + topicID: {"Topic"}, "element_options_" + topicID: {"Ads\nErrors"},
			"element_type_new1": {"text"}, "element_label_new1": {"Budget"}, "element_if_new1": {topicID}, "element_ifvalue_new1": {"Ads"},
		})
		if !strings.Contains(page, "Budget") || !strings.Contains(page, `data-visible-if="element_`+topicID+`" data-visible-value="Ads"`) {
			t.Error("the new question with its condition isn't on the preview")
		}
		var questions int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM webform_elements").Scan(&questions); err != nil || questions != 1 {
			t.Errorf("questions after a preview: %d, %v", questions, err)
		}
	})
}
