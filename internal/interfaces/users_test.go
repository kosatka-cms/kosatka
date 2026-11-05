package interfaces

import (
	"context"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/lib/htmlimages"
)

type fakeProfileTabViews struct {
	views.Service
	tabs entity.ViewDisplayDefs
}

func (f fakeProfileTabViews) UserTabDisplays(context.Context) (entity.ViewDisplayDefs, error) {
	return f.tabs, nil
}

type fakeProfileTabRenderer map[int]int

func (f fakeProfileTabRenderer) RenderProfileTab(context.Context, entity.ViewDisplayDef, int, int, string, url.Values) (template.HTML, int, error) {
	return "", 0, nil
}

func (f fakeProfileTabRenderer) ProfileTabCount(_ context.Context, v entity.ViewDisplayDef, _ int) (int, error) {
	return f[v.ID], nil
}

func TestProfileViewTabs(t *testing.T) {
	c := &UserController{BaseController: &BaseController{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	c.profileViews, c.profileTabs = fakeProfileTabViews{tabs: entity.ViewDisplayDefs{
		{ID: 1, Label: "Content", ProfileTabPath: "content", ProfileTabAccess: entity.ViewProfileTabAccessAll},
		{ID: 2, Label: "In moderation", ProfileTabPath: "moderation", ProfileTabAccess: entity.ViewProfileTabAccessOwner},
	}}, fakeProfileTabRenderer{1: 12, 2: 3}

	visitor := c.profileViewTabs(context.Background(), 7, false, "/users/ishmael")
	if len(visitor) != 1 || visitor[0].URL != "/users/ishmael/content" || visitor[0].Count != 12 || !visitor[0].Public {
		t.Errorf("visitor tabs: %+v", visitor)
	}
	owner := c.profileViewTabs(context.Background(), 7, true, "/me")
	if len(owner) != 2 || owner[1].URL != "/me/moderation" || owner[1].Count != 3 || owner[1].Public {
		t.Errorf("owner tabs: %+v", owner)
	}
}

// TestCaptchaFormError - with the CAPTCHA off for a form (or no keys) its
// submissions pass without a response.
func TestCaptchaFormError(t *testing.T) {
	t.Cleanup(func() { SetCaptcha(nil) })
	uc := UserController{}
	req := httptest.NewRequest(http.MethodPost, "/login", nil)

	SetCaptcha(nil)
	if msg := uc.captchaFormError(context.Background(), req, entity.CaptchaFormLogin); msg != "" {
		t.Errorf("no CAPTCHA service: %q", msg)
	}

	repo := &stubCaptchaRepo{s: entity.CaptchaSettings{Enabled: true, SiteKey: "s", SecretKey: "k", Forms: map[string]bool{entity.CaptchaFormLogin: false}}}
	svc := service.NewCaptchaService(repo, "", "", time.Second)
	if err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	SetCaptcha(svc)
	if msg := uc.captchaFormError(context.Background(), req, entity.CaptchaFormLogin); msg != "" {
		t.Errorf("login switched off: %q", msg)
	}
	if svc.SiteKey(entity.CaptchaFormLogin) != "" {
		t.Error("a site key for a form switched off")
	}
	// A form missing from the settings is on (the default).
	if svc.SiteKey(entity.CaptchaFormRegister) != "s" {
		t.Error("a form missing from the settings is off")
	}
}

// stubCaptchaRepo - CAPTCHA settings held in memory.
type stubCaptchaRepo struct{ s entity.CaptchaSettings }

func (r *stubCaptchaRepo) Get(context.Context) (*entity.CaptchaSettings, error) {
	c := r.s
	return &c, nil
}

func (r *stubCaptchaRepo) Update(_ context.Context, s *entity.CaptchaSettings) error {
	r.s = *s
	return nil
}

// profileTabAuth - the visitor is viewer (nil - a guest); admin - they
// may administer users.
type profileTabAuth struct {
	service.AuthService
	viewer *entity.User
	admin  bool
}

func (a profileTabAuth) GetUserID(*http.Request) (int, bool) {
	if a.viewer == nil {
		return 0, false
	}
	return a.viewer.ID, true
}

func (a profileTabAuth) GetCurrentUser(context.Context, *http.Request) (*entity.User, error) {
	if a.viewer == nil {
		return nil, entity.ErrEntityNotFound
	}
	return a.viewer, nil
}

func (a profileTabAuth) HasPermission(_ context.Context, user *entity.User, perm string) bool {
	return user != nil && a.admin && perm == service.PermAdministerUsers
}

func (a profileTabAuth) CreatableNodeTypes(context.Context, *entity.User) []entity.CreatableMaterialLink {
	return nil
}

// profileTabUsers - the users ann (1) and bob (2).
type profileTabUsers struct{ service.UserService }

var profileTabTestUsers = map[int]*entity.User{
	1: {ID: 1, Username: "ann", Slug: "users/ann", Status: entity.UserStatusActive},
	2: {ID: 2, Username: "bob", Slug: "users/bob", Status: entity.UserStatusActive},
}

func (profileTabUsers) ByID(_ context.Context, id int) (*entity.User, error) {
	if u, ok := profileTabTestUsers[id]; ok {
		return u, nil
	}
	return nil, entity.ErrEntityNotFound
}

func (profileTabUsers) FindBySlug(_ context.Context, slug string) (*entity.User, error) {
	for _, u := range profileTabTestUsers {
		if u.Slug == slug {
			return u, nil
		}
	}
	return nil, entity.ErrEntityNotFound
}

func (profileTabUsers) CountWithStatus(context.Context, int) (int, error)    { return 0, nil }
func (profileTabUsers) BumpSessionVersion(context.Context, int) (int, error) { return 1, nil }

// recordingTemplates - a template service keeping the last page's name
// and data.
type recordingTemplates struct {
	name string
	data map[string]any
}

func (t *recordingTemplates) Render(_ io.Writer, name string, data any, _ *http.Request, _ template.FuncMap) error {
	t.name, t.data = name, data.(map[string]any)
	return nil
}

func (t *recordingTemplates) RenderNamed(io.Writer, string, any, ...string) error { return nil }

func (t *recordingTemplates) SetContentProcessor(*htmlimages.ContentProcessor) {}

// TestProfileModuleTab - a module's profile tab (AddProfileTab): listed
// with its badge where Allowed lets the visitor, its URL serving the tab
// (the module's Load fills the data, the page renders
// "profile_tab_<module>"); not allowed - not listed, the URL a 403 (a
// guest offered to log in); the module off - the URL a 404. A View tab
// with the module tab's path isn't listed.
func TestProfileModuleTab(t *testing.T) {
	ResetModuleHooks()
	t.Cleanup(func() {
		ResetModuleHooks()
		service.SetModules(map[string]bool{"tabtest": true})
	})
	AddProfileTab(ProfileTab{
		Module: "tabtest", Path: "inbox", Label: "Inbox",
		Allowed: func(_ context.Context, viewer, _ *entity.User, isOwner, canAdminister bool) bool {
			return viewer != nil && (isOwner || canAdminister)
		},
		Badge: func(context.Context, *entity.User) int { return 3 },
		Load: func(_ context.Context, _ http.ResponseWriter, _ *http.Request, profile *entity.User, data map[string]any) error {
			data["inboxOf"] = profile.Username
			return nil
		},
	})
	if !entity.ProfileTabPathReserved("inbox") {
		t.Error("the module's tab path isn't reserved for the user_tab displays")
	}

	serve := func(viewer *entity.User, admin bool, path, alias, tab string) (int, *recordingTemplates) {
		t.Helper()
		ts := &recordingTemplates{}
		c := &UserController{
			BaseController: &BaseController{templateService: ts, authService: profileTabAuth{viewer: viewer, admin: admin}, userService: profileTabUsers{},
				store: sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef")), logger: testLogger()},
			profileViews: fakeProfileTabViews{tabs: entity.ViewDisplayDefs{{ID: 9, Label: "Old inbox", ProfileTabPath: "inbox"}}},
			profileTabs:  fakeProfileTabRenderer{},
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.SetPathValue("alias", alias)
		req.SetPathValue("tab", tab)
		rec := httptest.NewRecorder()
		c.Profile(rec, req)
		return rec.Code, ts
	}
	ann, bob := profileTabTestUsers[1], profileTabTestUsers[2]

	code, ts := serve(ann, false, "/me/inbox", "", "inbox")
	if code != http.StatusOK || ts.name != "user/profile_page.html" || ts.data["activeTab"] != "module" || ts.data["activeModuleTab"] != "tabtest" || ts.data["inboxOf"] != "ann" {
		t.Fatalf("the owner's tab: %d %s %v %v %v", code, ts.name, ts.data["activeTab"], ts.data["activeModuleTab"], ts.data["inboxOf"])
	}
	tabs, _ := ts.data["profileModuleTabs"].([]profileModuleTab)
	if len(tabs) != 1 || tabs[0].URL != "/me/inbox" || tabs[0].Badge != 3 || !tabs[0].Active || tabs[0].Label != "Inbox" {
		t.Errorf("the owner's module tabs: %+v", tabs)
	}
	if views, _ := ts.data["profileViewTabs"].([]profileViewTab); len(views) != 0 {
		t.Errorf("a View tab with the module tab's path is listed: %+v", views)
	}

	// An administrator on another's profile: the tab without the badge
	// (it counts the viewer's own).
	if code, ts := serve(ann, true, "/users/bob/inbox", "bob", "inbox"); code != http.StatusOK || ts.data["inboxOf"] != "bob" {
		t.Errorf("an administrator on bob's tab: %d %v", code, ts.data["inboxOf"])
	} else if tabs, _ := ts.data["profileModuleTabs"].([]profileModuleTab); len(tabs) != 1 || tabs[0].Badge != 0 {
		t.Errorf("an administrator on bob's profile, the module tabs: %+v", tabs)
	}
	// Another user: not listed on the profile, the tab's URL a 403.
	if _, ts := serve(bob, false, "/users/ann", "ann", ""); len(ts.data["profileModuleTabs"].([]profileModuleTab)) != 0 {
		t.Errorf("another user sees the tab: %+v", ts.data["profileModuleTabs"])
	}
	if code, ts := serve(bob, false, "/users/ann/inbox", "ann", "inbox"); code != http.StatusForbidden || ts.name != "user/profile_tab_forbidden.html" || ts.data["profileTabForbiddenAnonymous"] != false {
		t.Errorf("another user's tab: %d %s %v", code, ts.name, ts.data["profileTabForbiddenAnonymous"])
	}
	if code, ts := serve(nil, false, "/users/ann/inbox", "ann", "inbox"); code != http.StatusForbidden || ts.data["profileTabForbiddenAnonymous"] != true {
		t.Errorf("a guest on the tab: %d %v", code, ts.data["profileTabForbiddenAnonymous"])
	}

	// The module off: no tab, its URL a 404 (the View tab with its path is
	// back).
	service.SetModules(map[string]bool{"tabtest": false})
	if code, ts := serve(ann, false, "/me/inbox", "", "inbox"); code != http.StatusOK || ts.data["activeTab"] != "view" {
		t.Errorf("the module off, the View tab of the path: %d %v", code, ts.data["activeTab"])
	}
	if code, ts := serve(ann, false, "/me", "", ""); code != http.StatusOK || len(ts.data["profileModuleTabs"].([]profileModuleTab)) != 0 {
		t.Errorf("the module off, its tab listed: %d %+v", code, ts.data["profileModuleTabs"])
	}
}
