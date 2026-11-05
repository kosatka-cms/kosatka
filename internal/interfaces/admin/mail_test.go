package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

type stubMailRepo struct{ s entity.MailSettings }

func (r *stubMailRepo) Get(context.Context) (*entity.MailSettings, error) {
	s := r.s
	return &s, nil
}

func (r *stubMailRepo) Update(_ context.Context, s *entity.MailSettings) error {
	r.s = *s
	return nil
}

func postMail(repo *stubMailRepo, full bool, path string, form url.Values) *httptest.ResponseRecorder {
	ac := testController(interfaces.BaseDeps{Logger: testLogger(), Auth: captchaAuth{}, Users: captchaUsers{full: full}, Templates: service.NewTemplateService(false), Store: sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))}, nil)
	ac.mailService = service.NewMailService(repo, entity.MailSettings{}, testLogger(), time.Second)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	if path == "/admin/config/mail/test" {
		ac.MailTest(rec, req)
	} else {
		ac.MailPage(rec, req)
	}
	return rec
}

// TestMailPageSave - full access only; empty secret fields keep the saved
// secrets; "delete" goes back to the server config.
func TestMailPageSave(t *testing.T) {
	repo := &stubMailRepo{s: entity.MailSettings{Enabled: true, Transport: entity.MailTransportResend, FromAddress: "old@example.com", ResendAPIKey: "re_old", SMTPPassword: "pw"}}

	for _, path := range []string{"/admin/config/mail", "/admin/config/mail/test"} {
		if rec := postMail(repo, false, path, url.Values{"transport": {"smtp"}}); rec.Code != http.StatusForbidden || repo.s.Transport != entity.MailTransportResend {
			t.Fatalf("%s without full access: %d, %+v", path, rec.Code, repo.s)
		}
	}

	rec := postMail(repo, true, "/admin/config/mail", url.Values{"enabled": {"1"}, "transport": {"resend"}, "from_address": {"Site <new@example.com>"}})
	if !strings.Contains(rec.Header().Get("Location"), "saved=1") || repo.s.FromAddress != "Site <new@example.com>" || repo.s.ResendAPIKey != "re_old" || repo.s.SMTPPassword != "pw" {
		t.Fatalf("keep the secrets: %s %+v", rec.Header().Get("Location"), repo.s)
	}

	postMail(repo, true, "/admin/config/mail", url.Values{"transport": {"resend"}, "from_address": {"new@example.com"}})
	if repo.s.Enabled {
		t.Error("the switch unchecked keeps email on")
	}

	postMail(repo, true, "/admin/config/mail", url.Values{"enabled": {"1"}, "clear": {"1"}, "log_keep_days": {"90"}})
	if repo.s != (entity.MailSettings{Enabled: true, LogKeepDays: 90}) {
		t.Errorf("clear (the log's days kept): %+v", repo.s)
	}
}

// TestMailSettingsFromForm - validation of the submitted settings; with
// the transport in the server config only the switch and the log change.
func TestMailSettingsFromForm(t *testing.T) {
	saved := entity.MailSettings{Enabled: true, Transport: entity.MailTransportResend, FromAddress: "a@example.com", ResendAPIKey: "re_saved"}
	cases := []struct {
		name string
		form url.Values
		code string
	}{
		{"no transport", url.Values{"from_address": {"a@example.com"}}, "transport"},
		{"bad sender", url.Values{"transport": {"resend"}, "from_address": {"not an address"}}, "from"},
		{"no sender", url.Values{"transport": {"resend"}}, "from"},
		{"bad port", url.Values{"transport": {"smtp"}, "from_address": {"a@example.com"}, "smtp_host": {"h"}, "smtp_port": {"70000"}}, "port"},
		{"smtp without host", url.Values{"transport": {"smtp"}, "from_address": {"a@example.com"}, "smtp_port": {"587"}}, "smtp"},
		{"smtp", url.Values{"transport": {"smtp"}, "from_address": {"a@example.com"}, "smtp_host": {"h"}, "smtp_port": {"587"}}, ""},
		{"resend with the saved key", url.Values{"transport": {"resend"}, "from_address": {"a@example.com"}}, ""},
		{"a log's days not offered", url.Values{"transport": {"resend"}, "from_address": {"a@example.com"}, "log_keep_days": {"5"}}, "log"},
		{"no log", url.Values{"transport": {"resend"}, "from_address": {"a@example.com"}, "log_keep_days": {"0"}}, ""},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(c.form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if _, code := mailSettingsFromForm(r, saved, false); code != c.code {
			t.Errorf("%s: %q, want %q", c.name, code, c.code)
		}
	}

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{"transport": {"resend"}, "from_address": {"a@example.com"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, code := mailSettingsFromForm(r, entity.MailSettings{}, false); code != "resend" {
		t.Errorf("resend without a key: %q", code)
	}

	form := url.Values{"transport": {"smtp"}, "from_address": {"other@example.com"}, "resend_api_key": {"re_new"}, "log_keep_days": {"30"}}
	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	next, code := mailSettingsFromForm(r, saved, true)
	if code != "" || next.Enabled || next.LogKeepDays != 30 || next.Transport != saved.Transport || next.FromAddress != saved.FromAddress || next.ResendAPIKey != saved.ResendAPIKey {
		t.Errorf("with the config's transport: %+v %q, want the switch and the log only", next, code)
	}
}

// mailTemplateRows - the mail_templates rows in memory.
type mailTemplateRows map[string]entity.StoredMail

func (r mailTemplateRows) All(context.Context) (map[string]entity.StoredMail, error) { return r, nil }

func (r mailTemplateRows) SaveText(_ context.Context, key string, t entity.MailTemplate) error {
	st := r[key]
	st.MailTemplate = t
	r[key] = st
	return nil
}

func (r mailTemplateRows) SetEnabled(_ context.Context, key string, on bool) error {
	st := r[key]
	st.Enabled = on
	r[key] = st
	return nil
}

// TestMailTemplateHandlers - an email's text saved (trimmed; "Default
// text" - blank), an optional one switched off and on, a required one
// can't be; a test email needs a valid address; an unknown email is 404,
// and all of it needs full access.
func TestMailTemplateHandlers(t *testing.T) {
	service.ResetMailDefs()
	t.Cleanup(service.ResetMailDefs)
	service.RegisterMail(entity.MailDef{Key: "mod.optional", Optional: true, Default: entity.MailTemplate{Subject: "S", Body: "B"}})
	service.RegisterMail(entity.MailDef{Key: "mod.required", Default: entity.MailTemplate{Subject: "S", Body: "B"}})
	rows := mailTemplateRows{}
	post := func(full bool, key, action string, form url.Values) *httptest.ResponseRecorder {
		ac := testController(interfaces.BaseDeps{Logger: testLogger(), Auth: captchaAuth{}, Users: captchaUsers{full: full}, Templates: service.NewTemplateService(false), Store: sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))}, nil)
		ac.mailService = service.NewMailService(&stubMailRepo{}, entity.MailSettings{}, testLogger(), time.Second)
		ac.mailService.SetTemplates(rows)
		req := httptest.NewRequest(http.MethodPost, "/admin/config/mail/templates/"+key+action, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetPathValue("key", key)
		rec := httptest.NewRecorder()
		switch action {
		case "/enabled":
			ac.MailTemplateSwitch(rec, req)
		case "/test":
			ac.MailTemplateTest(rec, req)
		default:
			ac.MailTemplateSave(rec, req)
		}
		return rec
	}

	if rec := post(false, "mod.optional", "", url.Values{"subject": {"x"}}); rec.Code != http.StatusForbidden || len(rows) != 0 {
		t.Fatalf("without full access: %d %v", rec.Code, rows)
	}
	if rec := post(true, "none", "", url.Values{"subject": {"x"}}); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown email: %d", rec.Code)
	}
	if rec := post(true, "mod.optional", "", url.Values{"subject": {" Hi "}, "body": {"Text"}}); !strings.Contains(rec.Header().Get("Location"), "saved=1") ||
		rows["mod.optional"].Subject != "Hi" || rows["mod.optional"].Body != "Text" {
		t.Errorf("saved: %s %+v", rec.Header().Get("Location"), rows["mod.optional"])
	}
	post(true, "mod.optional", "", url.Values{"subject": {"Hi"}, "reset": {"1"}})
	if rows["mod.optional"].MailTemplate != (entity.MailTemplate{}) {
		t.Errorf("the default text: %+v", rows["mod.optional"])
	}
	post(true, "mod.optional", "/enabled", url.Values{})
	if st, ok := rows["mod.optional"]; !ok || st.Enabled {
		t.Errorf("switched off: %+v", st)
	}
	post(true, "mod.optional", "/enabled", url.Values{"on": {"1"}})
	if !rows["mod.optional"].Enabled {
		t.Error("not switched on")
	}
	if rec := post(true, "mod.required", "/enabled", url.Values{}); rec.Code != http.StatusBadRequest {
		t.Errorf("a required email switched: %d", rec.Code)
	}
	if rec := post(true, "mod.required", "/test", url.Values{"to": {"not an address"}}); !strings.Contains(rec.Body.String(), "Enter a valid email address.") {
		t.Errorf("a test to a bad address: %d", rec.Code)
	}
	if rec := post(true, "mod.required", "/test", url.Values{"to": {"admin@example.com"}}); !strings.Contains(rec.Body.String(), "Email sending is off or not configured.") {
		t.Errorf("a test with email off: %d", rec.Code)
	}
}
