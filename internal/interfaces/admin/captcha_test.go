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

type captchaAuth struct {
	service.AuthService
}

func (captchaAuth) GetUserID(*http.Request) (int, bool) { return 1, true }

// The rest of what the site's pages (the 403 one) ask: a regular user.
func (captchaAuth) HasPermission(context.Context, *entity.User, string) bool { return false }
func (captchaAuth) CreatableNodeTypes(context.Context, *entity.User) []entity.CreatableMaterialLink {
	return nil
}
func (captchaAuth) CanUseSiteInMaintenanceMode(context.Context, *entity.User) bool { return false }

type captchaUsers struct {
	service.UserService
	full bool
}

func (u captchaUsers) ByID(_ context.Context, id int) (*entity.User, error) {
	role := &entity.Role{Permissions: map[string]any{}}
	if u.full {
		role.Permissions[service.PermFullAccess] = true
	}
	return &entity.User{ID: id, Role: role}, nil
}

type stubCaptchaRepo struct{ s entity.CaptchaSettings }

func (r *stubCaptchaRepo) Get(context.Context) (*entity.CaptchaSettings, error) {
	c := r.s
	return &c, nil
}

func (r *stubCaptchaRepo) Update(_ context.Context, s *entity.CaptchaSettings) error {
	r.s = *s
	return nil
}

// TestCaptchaPageSave - full access only; an empty secret keeps the saved
// one, a lone key is refused, "delete the keys" clears both; the forms
// follow their checkboxes.
func TestCaptchaPageSave(t *testing.T) {
	repo := &stubCaptchaRepo{s: entity.CaptchaSettings{SiteKey: "site-1", SecretKey: "secret-1", Forms: map[string]bool{}}}
	post := func(full bool, form url.Values) *httptest.ResponseRecorder {
		ac := testController(interfaces.BaseDeps{Logger: testLogger(), Auth: captchaAuth{}, Users: captchaUsers{full: full}, Templates: service.NewTemplateService(false), Store: sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))}, nil)
		ac.captchaService = service.NewCaptchaService(repo, "", "", time.Second)
		req := httptest.NewRequest(http.MethodPost, "/admin/config/captcha", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		ac.CaptchaPage(rec, req)
		return rec
	}

	if rec := post(false, url.Values{"site_key": {"x"}, "secret_key": {"y"}}); rec.Code != http.StatusForbidden || repo.s.SiteKey != "site-1" {
		t.Fatalf("without full access: %d, %+v", rec.Code, repo.s)
	}

	rec := post(true, url.Values{"enabled": {"1"}, "site_key": {"site-2"}, "secret_key": {""}, "form_" + entity.CaptchaFormLogin: {"1"}})
	if !strings.Contains(rec.Header().Get("Location"), "saved=1") || !repo.s.Enabled || repo.s.SiteKey != "site-2" || repo.s.SecretKey != "secret-1" {
		t.Fatalf("keep the secret: %s %+v", rec.Header().Get("Location"), repo.s)
	}
	if !repo.s.Forms[entity.CaptchaFormLogin] || repo.s.Forms[entity.CaptchaFormRegister] {
		t.Errorf("forms: %v", repo.s.Forms)
	}

	repo.s.SecretKey = ""
	if rec := post(true, url.Values{"site_key": {"site-3"}}); !strings.Contains(rec.Header().Get("Location"), "error=pair") || repo.s.SiteKey != "site-2" {
		t.Fatalf("a lone site key: %s %+v", rec.Header().Get("Location"), repo.s)
	}

	// Switched off: the keys stay.
	repo.s.SecretKey = "secret-1"
	if post(true, url.Values{"site_key": {"site-2"}}); repo.s.Enabled || repo.s.SiteKey != "site-2" || repo.s.SecretKey != "secret-1" {
		t.Fatalf("switch off: %+v", repo.s)
	}

	if post(true, url.Values{"clear_keys": {"1"}, "site_key": {"ignored"}}); repo.s.SiteKey != "" || repo.s.SecretKey != "" {
		t.Fatalf("clear: %+v", repo.s)
	}
}
