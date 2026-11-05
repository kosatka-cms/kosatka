package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestRegisterPageLoginLink - the registration page links to the login
// page, keeping the destination.
func TestRegisterPageLoginLink(t *testing.T) {
	zzChdirToProjectRoot(t)
	for dest, want := range map[string]string{
		"":          `<a href="/login">Log in</a>`,
		"/node/5?a": `<a href="/login?destination=%2fnode%2f5%3fa">Log in</a>`,
	} {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, "content", map[string]any{"destination": dest}, "user/register.html", "user/social_login.html", "partials/required_user_fields.html", "partials/password_strength.html", "partials/recaptcha.html"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), want) {
			t.Errorf("destination %q: no %s", dest, want)
		}
	}
}

// TestRegisterPageRequiredFields - the registration page asks the
// required profile fields, keeping what was submitted, and shows the
// message of the one to correct.
func TestRegisterPageRequiredFields(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	var buf strings.Builder
	data := map[string]any{
		"signupFields": []entity.UserFieldValue{{Field: entity.UserFieldDef{ID: 3, Label: "City", MaxLen: 100, Required: true}, Value: "Paris"}},
		"errors":       map[string]string{"user_fields": "\"City\" is required"},
	}
	if err := svc.RenderNamed(&buf, "content", data, "user/register.html", "user/social_login.html", "partials/required_user_fields.html", "partials/password_strength.html", "partials/recaptcha.html"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<label for="user_field_3" class="form-label">City *</label>`, `name="user_field_3"`, `value="Paris" required maxlength="100"`, "&#34;City&#34; is required"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("no %s", want)
		}
	}
}

// TestLoginPageRegisterLink - the login page links to registration only
// while it is open, keeping the destination.
func TestLoginPageRegisterLink(t *testing.T) {
	zzChdirToProjectRoot(t)
	render := func(data map[string]any) string {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, "content", data, "user/login.html", "user/social_login.html", "partials/recaptcha.html", "partials/password_strength.html"); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	if out := render(map[string]any{"registrationOpen": true, "destination": "/node/5"}); !strings.Contains(out, `<a href="/user/register?destination=%2fnode%2f5">Create an account</a>`) {
		t.Errorf("no register link:\n%s", out)
	}
	if out := render(map[string]any{"registrationOpen": false}); strings.Contains(out, "/user/register") {
		t.Error("register link while registration is closed")
	}
}

// TestRegisterLinkOnClosedTab - a guest on a closed profile tab is
// offered to register only while registration is open; to log in always.
func TestRegisterLinkOnClosedTab(t *testing.T) {
	zzChdirToProjectRoot(t)
	t.Cleanup(func() { entity.SetRegistrationMode(entity.RegistrationModeVisitors) })
	render := func() string {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		var buf strings.Builder
		data := map[string]any{"profileTabForbiddenAnonymous": true, "profileTabForbiddenDestination": "/users/ann/notes"}
		if err := svc.RenderNamed(&buf, "content", data, "user/profile_tab_forbidden.html"); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	entity.SetRegistrationMode(entity.RegistrationModeVisitors)
	if out := render(); !strings.Contains(out, `/user/register?destination=/users/ann/notes`) {
		t.Errorf("no register link while registration is open:\n%s", out)
	}
	entity.SetRegistrationMode(entity.RegistrationModeAdminOnly)
	if out := render(); strings.Contains(out, "/user/register") || !strings.Contains(out, `/login?destination=/users/ann/notes`) {
		t.Errorf("registration closed: a register link, or no log in link:\n%s", out)
	}
}

// TestLoginCard - the login card: the site's name, the password shown on
// a click, the social networks' buttons by name (the full words for
// screen readers) side by side.
func TestLoginCard(t *testing.T) {
	zzChdirToProjectRoot(t)
	type provider struct{ Name, Label string }
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	var buf strings.Builder
	data := map[string]any{"siteName": "Example Site", "oauthCompact": true,
		"oauth_providers": []provider{{Name: "google", Label: "Google"}, {Name: "vk", Label: "VK"}}}
	if err := svc.RenderNamed(&buf, "content", data, "user/login.html", "user/social_login.html", "partials/recaptcha.html", "partials/password_strength.html"); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		`<h1 class="auth-card-title">Log in</h1>`, `<p class="auth-card-sub">to the site Example Site</p>`,
		`data-password-toggle="loginPassword" aria-label="Show the password"`, `<a href="/user/password" class="auth-link-small">`,
		`class="oauth-buttons oauth-buttons--row"`, `aria-label="Log in with Google">`, `</span> Google </a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}

// TestLogoutPage - GET /logout's page logs out with a POST carrying the
// CSRF token, not with a link.
func TestLogoutPage(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	var buf strings.Builder
	if err := svc.RenderNamed(&buf, "content", map[string]any{"CSRFToken": "tok"}, "user/logout.html"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<form method="post" action="/logout"`, `name="csrf_token" value="tok"`, `type="submit"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("no %s in %s", want, buf.String())
		}
	}
}

// TestUserMenuRegisterLink - the guest's menu offers "Register" only while
// visitors may sign up (the account settings' mode).
func TestUserMenuRegisterLink(t *testing.T) {
	zzChdirToProjectRoot(t)
	t.Cleanup(func() { entity.SetRegistrationMode(entity.RegistrationModeVisitors) })
	render := func() string {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, "block_user_menu", map[string]any{}, "blocks/user_menu.html"); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	entity.SetRegistrationMode(entity.RegistrationModeVisitorsApproval)
	if out := render(); !strings.Contains(out, `href="/user/register"`) {
		t.Errorf("open: no register link:\n%s", out)
	}
	entity.SetRegistrationMode(entity.RegistrationModeAdminOnly)
	if out := render(); strings.Contains(out, "/user/register") {
		t.Error("closed: a register link")
	}
}
