package admin

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

type fakeAdminPermRoles struct {
	entity.RoleRepository
	roles   map[int]*entity.Role
	users   map[int]int
	listErr error
}

func (f *fakeAdminPermRoles) Create(_ context.Context, r entity.Role) (*entity.Role, error) {
	r.ID = len(f.roles) + 100
	f.roles[r.ID] = &r
	return &r, nil
}

func (f *fakeAdminPermRoles) Delete(_ context.Context, id int) error {
	delete(f.roles, id)
	return nil
}

func (f *fakeAdminPermRoles) CountUsers(_ context.Context, id int) (int, error) {
	return f.users[id], nil
}

func (f *fakeAdminPermRoles) ByID(_ context.Context, id int) (*entity.Role, error) {
	r := *f.roles[id]
	return &r, nil
}

func (f *fakeAdminPermRoles) ByName(_ context.Context, name string) (*entity.Role, error) {
	for _, r := range f.roles {
		if r.Name == name {
			return r, nil
		}
	}
	return nil, entity.ErrEntityNotFound
}

func (f *fakeAdminPermRoles) List(context.Context) (*entity.Roles, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out entity.Roles
	for _, r := range f.roles {
		out = append(out, *r)
	}
	return &out, nil
}

func (f *fakeAdminPermRoles) Update(_ context.Context, r entity.Role) (*entity.Role, error) {
	f.roles[r.ID] = &r
	return &r, nil
}

type fakeAdminPermTypes struct{}

func (fakeAdminPermTypes) ContentTypes(context.Context) (entity.ContentTypeDefs, error) {
	return entity.ContentTypeDefs{{MachineName: "article", Label: "Articles"}}, nil
}

func newPermissionsTestController(roles *fakeAdminPermRoles) *Controller {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := testController(interfaces.BaseDeps{Logger: logger}, nil)
	c.permissions = service.NewPermissionService(roles, fakeAdminPermTypes{}, nil, logger)
	return c
}

func postPermissions(c *Controller, grants ...string) *httptest.ResponseRecorder {
	form := url.Values{"grant": grants}
	req := httptest.NewRequest(http.MethodPost, "/admin/users/permissions", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	c.PermissionsSave(rec, req)
	return rec
}

// The matrix form: each checked box is "<role id>|<permission>"; a role
// with no boxes checked loses its catalog permissions; malformed values
// are ignored; a full-access role is left alone.
func TestPermissionsSave(t *testing.T) {
	roles := &fakeAdminPermRoles{roles: map[int]*entity.Role{
		1: {ID: 1, Name: entity.RoleUser, Permissions: map[string]any{"create article content": true}},
		2: {ID: 2, Name: entity.RoleModerator, Permissions: map[string]any{"moderate content": true, "outside the catalog": true}},
		3: {ID: 3, Name: entity.RoleAdmin, Permissions: map[string]any{"full_access": true}},
	}}
	c := newPermissionsTestController(roles)
	rec := postPermissions(c,
		"2|edit any article content",
		"2|publish content",
		"x|publish content",  // bad role id
		"2 publish content",  // no separator
		"3|moderate content", // full-access role - ignored
		"2|not a permission", // outside the catalog - ignored
	)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/users/permissions?success=1" {
		t.Fatalf("response %d %q", rec.Code, rec.Header().Get("Location"))
	}
	user, mod, admin := roles.roles[1], roles.roles[2], roles.roles[3]
	if service.HasGrant(user, "create article content") {
		t.Errorf("unchecked base role permission kept: %v", user.Permissions)
	}
	for _, p := range []string{"edit any article content", "publish content", "outside the catalog"} {
		if !service.HasGrant(mod, p) {
			t.Errorf("moderator lacks %q: %v", p, mod.Permissions)
		}
	}
	for _, p := range []string{"moderate content", "not a permission"} {
		if service.HasGrant(mod, p) {
			t.Errorf("moderator has %q: %v", p, mod.Permissions)
		}
	}
	if len(admin.Permissions) != 1 || !service.HasGrant(admin, "full_access") {
		t.Errorf("full-access role changed: %v", admin.Permissions)
	}
}

func TestPermissionsSaveRolesError(t *testing.T) {
	roles := &fakeAdminPermRoles{roles: map[int]*entity.Role{}, listErr: errors.New("db down")}
	rec := postPermissions(newPermissionsTestController(roles), "1|publish content")
	if rec.Header().Get("Location") != "/admin/users/permissions?error=save" {
		t.Fatalf("Location = %q", rec.Header().Get("Location"))
	}
}

func postRoles(c *Controller, path string, handler func(http.ResponseWriter, *http.Request), id string, form url.Values) string {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if id != "" {
		req.SetPathValue("id", id)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec.Header().Get("Location")
}

// The roles page handlers redirect with a code the page explains.
func TestRolesHandlers(t *testing.T) {
	roles := &fakeAdminPermRoles{roles: map[int]*entity.Role{
		1: {ID: 1, Name: entity.RoleUser, Description: "Registered"},
		2: {ID: 2, Name: entity.RoleModerator, Description: "Moderator"},
	}, users: map[int]int{2: 13}}
	c := newPermissionsTestController(roles)
	for _, tc := range []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		id      string
		form    url.Values
		want    string
	}{
		{"create", c.RoleCreate, "", url.Values{"name": {"forum_moderator"}, "label": {"Forum moderator"}}, "?success=created"},
		{"create bad name", c.RoleCreate, "", url.Values{"name": {"Forum"}, "label": {"x"}}, "?error=name"},
		{"create existing", c.RoleCreate, "", url.Values{"name": {"moderator"}, "label": {"x"}}, "?error=exists"},
		{"rename", c.RoleRename, "2", url.Values{"label": {"Moderators"}}, "?success=saved"},
		{"rename empty", c.RoleRename, "2", url.Values{"label": {""}}, "?error=label"},
		{"delete base", c.RoleDelete, "1", nil, "?error=protected"},
		{"delete in use", c.RoleDelete, "2", nil, "?error=in-use"},
	} {
		if got := postRoles(c, "/admin/users/roles", tc.handler, tc.id, tc.form); got != "/admin/users/roles"+tc.want {
			t.Errorf("%s: Location %q, want %q", tc.name, got, "/admin/users/roles"+tc.want)
		}
	}
	if roles.roles[2].Description != "Moderators" {
		t.Fatalf("rename not saved: %+v", roles.roles[2])
	}
	created := 0
	for id, r := range roles.roles {
		if r.Name == "forum_moderator" {
			created = id
		}
	}
	if created == 0 {
		t.Fatal("role not created")
	}
	if got := postRoles(c, "/admin/users/roles", c.RoleDelete, strconv.Itoa(created), nil); got != "/admin/users/roles?success=deleted" {
		t.Fatalf("delete: %q", got)
	}
}
