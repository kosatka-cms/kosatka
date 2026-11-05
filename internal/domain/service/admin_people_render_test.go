package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestAdminPeopleRolesRender - the people pages show roles by their
// readable name (Role.Label), the machine name only for a role without one.
func TestAdminPeopleRolesRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	render := func(page string, data map[string]any) string {
		t.Helper()
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		files, err := svc.pageFilesToParse(page, strings.Split(page, "/"))
		if err != nil {
			t.Fatal(err)
		}
		set, err := svc.getOrParse(page, nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatal(err)
		}
		var buf strings.Builder
		if err := set.ExecuteTemplate(&buf, "content", data); err != nil {
			t.Fatalf("%s: %v", page, err)
		}
		return buf.String()
	}
	editors := entity.Role{ID: 2, Name: "editor", Description: "Editors"}
	roles := &entity.Roles{editors, {ID: 3, Name: "reviewer"}}

	out := render("admin/people/index.html", map[string]any{
		"roles": roles, "filterRoleID": 2, "pagination": pagination.New(1, 1, 50, "/admin/people", nil),
		"users": []entity.User{{ID: 7, Username: "ann", Slug: "users/ann", Status: entity.UserStatusActive, Role: &editors}},
	})
	for _, want := range []string{`<option value="2" selected>Editors</option>`, `<option value="3" >reviewer</option>`, `<td>Editors</td>`} {
		if !strings.Contains(out, want) {
			t.Errorf("people list lacks %q", want)
		}
	}

	out = render("admin/people/create.html", map[string]any{
		"roles": roles, "form": map[string]string{"role_id": "2"}, "errors": map[string]string{},
	})
	for _, want := range []string{`<option value="2" selected>Editors</option>`, `<option value="3" >reviewer</option>`} {
		if !strings.Contains(out, want) {
			t.Errorf("new user form lacks %q", want)
		}
	}
}

// TestAdminAccountSettingsRender - the first section's heading names what
// it holds (no cancellation settings there).
func TestAdminAccountSettingsRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	page := "admin/people/account_settings.html"
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse(page, strings.Split(page, "/"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse(page, nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	if err := set.ExecuteTemplate(&buf, "content", map[string]any{"account": &entity.AccountSettings{}}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, ">Registration and profiles</h2>") || strings.Contains(out, "cancellation") {
		t.Errorf("account settings heading:\n%s", out)
	}
}
