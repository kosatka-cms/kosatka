package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestInvitePage - the invitation's card: the site's name, as whom the
// person is invited, the login offered, the password twice, a field to
// correct marked; a link no good - said so, no form.
func TestInvitePage(t *testing.T) {
	zzChdirToProjectRoot(t)
	render := func(data map[string]any) string {
		t.Helper()
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, "content", data, "user/invite.html", "partials/password_strength.html"); err != nil {
			t.Fatal(err)
		}
		return strings.Join(strings.Fields(buf.String()), " ")
	}
	out := render(map[string]any{"siteName": "Example Site", "token": "tok", "username": "maria",
		"invited": &entity.User{Role: &entity.Role{Name: "editor", Description: "Editor"}}, "errors": map[string]string{"password": "Too short"}})
	for _, want := range []string{
		`<h1 class="auth-card-title">Invitation</h1>`, `<p class="auth-card-sub">to the site Example Site</p>`,
		`<p class="auth-card-muted">You are invited as "Editor".</p>`,
		`<input type="hidden" name="token" value="tok">`, `name="username" value="maria"`,
		`class="form-control is-invalid" id="password"`, `<div class="invalid-feedback d-block">Too short</div>`,
		`>Accept and sign in</button>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if out := render(map[string]any{"errors": map[string]string{}}); !strings.Contains(out, "This invitation link is invalid") || strings.Contains(out, "<form") {
		t.Errorf("a link no good: %s", out)
	}
}
