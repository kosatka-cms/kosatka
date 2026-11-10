package admin

import (
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// renderPeople renders the content of an admin people page with data.
func renderPeople(t *testing.T, page string, data map[string]any) string {
	t.Helper()
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	if err := ts.RenderPageBlock(&buf, page, "content", data); err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// TestPeopleInvitesRender - the list says an invitation is sent, or that
// email was off; an invited user's status is "Invited" with until when
// its link works (or that it expired, or none was sent) and the buttons
// to send it again and to withdraw it; the others, their status and no
// such buttons.
func TestPeopleInvitesRender(t *testing.T) {
	until := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	users := []entity.User{
		{ID: 7, Username: "maria", Slug: "users/maria", Status: entity.UserStatusActive, PasswordHash: entity.PasswordUnset},
		{ID: 8, Username: "pavel", Slug: "users/pavel", Status: entity.UserStatusActive, PasswordHash: entity.PasswordUnset},
		{ID: 9, Username: "ann", Slug: "users/ann", Status: entity.UserStatusActive, PasswordHash: "x"},
		{ID: 10, Username: "cat", Slug: "users/cat", Status: entity.UserStatusActive, PasswordHash: entity.PasswordUnset},
	}
	out := renderPeople(t, "admin/people/index.html", map[string]any{
		"users": users, "pagination": pagination.New(1, 3, 50, "/admin/users", nil), "invited": "off",
		"invites": map[int]*peopleInvite{7: {Until: until}, 8: {Until: until.AddDate(0, 0, -14), Expired: true}, 10: {NotSent: true}},
	})
	for _, want := range []string{
		`Email is off: the account is created, the invitation wasn't sent.`,
		`<span class="admin-pill admin-pill--scheduled">Invited</span><span class="admin-table-sub">link until`,
		`<span class="admin-pill admin-pill--warn">Invitation expired</span>`,
		`<span class="admin-pill admin-pill--warn">Invitation not sent</span>`, `formaction="/admin/users/10/invite"`,
		`formaction="/admin/users/7/invite" data-bulk-skip class="btn btn-sm btn-outline-primary">Send again</button>`,
		`formaction="/admin/users/8/invite/cancel" data-bulk-skip data-confirm="Withdraw the invitation?`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if strings.Contains(out, "/admin/users/9/invite") || strings.Count(out, `admin-pill--ok">Active`) != 1 {
		t.Errorf("a user with a password shown as invited: %s", out)
	}
}

// TestPeopleCreateRender - the new user form: the invitation by email
// chosen by default, the password's fields shown only for a password set
// now, the login not required.
func TestPeopleCreateRender(t *testing.T) {
	out := renderPeople(t, "admin/people/create.html", map[string]any{"form": map[string]string{}, "errors": map[string]string{}})
	for _, want := range []string{
		`<option value="invite" selected>Invite by email</option> <option value="set">Set a password now</option>`,
		`<div class="admin-setting" data-show-when="password_mode=set">`,
		`id="username" name="username" value="" autocomplete="username">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	out = renderPeople(t, "admin/people/create.html", map[string]any{"form": map[string]string{"password_mode": "set"}, "errors": map[string]string{}})
	if !strings.Contains(out, `<option value="set" selected>Set a password now</option>`) {
		t.Errorf("a password chosen: %s", out)
	}
}
