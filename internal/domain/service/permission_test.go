package service

import (
	"context"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

type fakePermRoles struct {
	entity.RoleRepository
	roles     map[int]*entity.Role
	users     map[int]int
	byNameN   int
	byIDErr   error
	updateErr error
}

func (f *fakePermRoles) ByID(_ context.Context, id int) (*entity.Role, error) {
	if f.byIDErr != nil {
		return nil, f.byIDErr
	}
	r := *f.roles[id]
	return &r, nil
}

func (f *fakePermRoles) ByName(_ context.Context, name string) (*entity.Role, error) {
	f.byNameN++
	for _, r := range f.roles {
		if r.Name == name {
			c := *r
			return &c, nil
		}
	}
	return nil, entity.ErrEntityNotFound
}

func (f *fakePermRoles) List(context.Context) (*entity.Roles, error) {
	var out entity.Roles
	for _, r := range f.roles {
		out = append(out, *r)
	}
	return &out, nil
}

func (f *fakePermRoles) Create(_ context.Context, r entity.Role) (*entity.Role, error) {
	r.ID = len(f.roles) + 100
	f.roles[r.ID] = &r
	return &r, nil
}

func (f *fakePermRoles) Delete(_ context.Context, id int) error {
	delete(f.roles, id)
	return nil
}

func (f *fakePermRoles) CountUsers(_ context.Context, id int) (int, error) {
	return f.users[id], nil
}

func (f *fakePermRoles) Update(_ context.Context, r entity.Role) (*entity.Role, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.roles[r.ID] = &r
	return &r, nil
}

type fakePermTypes entity.ContentTypeDefs

func (f fakePermTypes) ContentTypes(context.Context) (entity.ContentTypeDefs, error) {
	return entity.ContentTypeDefs(f), nil
}

func newTestPermissions() (*PermissionService, *fakePermRoles) {
	roles := &fakePermRoles{roles: map[int]*entity.Role{
		1: {ID: 1, Name: entity.RoleUser, Permissions: map[string]any{PermCreate("post"): true}},
		2: {ID: 2, Name: entity.RoleModerator, Permissions: map[string]any{PermModerateContent: true, "legacy": true}},
		3: {ID: 3, Name: entity.RoleAdmin, Permissions: map[string]any{PermFullAccess: true}},
	}}
	types := fakePermTypes{{MachineName: "article", Label: "Articles"}, {MachineName: "post", Label: "Posts"}}
	return NewPermissionService(roles, types, nil, slog.New(slog.NewTextHandler(io.Discard, nil))), roles
}

func TestPermissionServiceHasPermission(t *testing.T) {
	s, roles := newTestPermissions()
	ctx := t.Context()
	noRole := &entity.User{ID: 10}
	moderator := &entity.User{ID: 11, Role: roles.roles[2]}
	admin := &entity.User{ID: 12, Role: roles.roles[3]}
	for _, tc := range []struct {
		name string
		user *entity.User
		perm string
		want bool
	}{
		{"anonymous has nothing", nil, PermCreate("post"), false},
		{"any signed-in user has the base role", noRole, PermCreate("post"), true},
		{"not granted", noRole, PermModerateContent, false},
		{"own role", moderator, PermModerateContent, true},
		{"own role plus the base role", moderator, PermCreate("post"), true},
		{"full access", admin, PermDeleteAny("article"), true},
	} {
		if got := s.HasPermission(ctx, tc.user, tc.perm); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPermissionServiceCatalogAndSave(t *testing.T) {
	withModules(t, nil) // all on, the forms (off by default) too
	s, roles := newTestPermissions()
	ctx := t.Context()
	catalog, err := s.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var labels []string
	for _, g := range catalog {
		labels = append(labels, g.Label)
	}
	// General first, content types by label, then the modules' node types.
	if got := strings.Join(labels, ","); !strings.HasPrefix(got, "General,Articles,Posts,") || !strings.Contains(got, "Form") || !strings.Contains(got, "Poll") {
		t.Fatalf("groups = %s", got)
	}

	if err := s.SaveGrants(ctx, 2, map[string]bool{PermEditAny("article"): true}); err != nil {
		t.Fatal(err)
	}
	mod := roles.roles[2]
	if !HasGrant(mod, PermEditAny("article")) || HasGrant(mod, PermModerateContent) || !HasGrant(mod, "legacy") {
		t.Fatalf("moderator permissions = %v (catalog keys replaced, others kept)", mod.Permissions)
	}
	if err := s.SaveGrants(ctx, 3, nil); err != nil || !HasGrant(roles.roles[3], PermFullAccess) {
		t.Fatalf("a full-access role must be left alone: %v %v", err, roles.roles[3].Permissions)
	}

	// The base role's cache is reset by a save.
	user := &entity.User{ID: 10}
	if !s.HasPermission(ctx, user, PermCreate("post")) {
		t.Fatal("base role grant")
	}
	if err := s.SaveGrants(ctx, 1, nil); err != nil {
		t.Fatal(err)
	}
	if s.HasPermission(ctx, user, PermCreate("post")) {
		t.Fatal("the base role's revoked grant still applies (stale cache)")
	}
}

func TestPermissionsPageRenders(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	s, roles := newTestPermissions()
	catalog, _ := s.Catalog(t.Context())
	type column struct {
		Role       entity.Role
		FullAccess bool
		Base       bool
	}
	data := map[string]any{
		"catalog": catalog,
		"columns": []column{{Role: *roles.roles[1], Base: true}, {Role: *roles.roles[3], FullAccess: true}},
	}
	var buf strings.Builder
	if err := svc.RenderNamed(&buf, "content", data, "admin/people/permissions.html", "admin/alerts.html"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `value="1|create post content" checked`) || !strings.Contains(out, `checked disabled title="Full access"`) {
		t.Fatalf("matrix:\n%s", out)
	}
}

type fakePermFormats entity.TextFormatDefs

func (f fakePermFormats) TextFormats(context.Context) (entity.TextFormatDefs, error) {
	return entity.TextFormatDefs(f), nil
}

type failingPermTypes struct{}

func (failingPermTypes) ContentTypes(context.Context) (entity.ContentTypeDefs, error) {
	return nil, errors.New("db down")
}

func catalogKeys(groups []PermissionGroup) map[string][]string {
	out := map[string][]string{}
	for _, g := range groups {
		for _, p := range g.Permissions {
			out[g.Label] = append(out[g.Label], p.Key)
		}
	}
	return out
}

// Every node type - content types and the modules' node types - gets
// exactly the four node permissions; text formats get theirs
// only when there are formats; general permissions come first.
func TestPermissionCatalogContents(t *testing.T) {
	// Every module on, including those off by default.
	withModules(t, nil)
	_, roles := newTestPermissions()
	s := NewPermissionService(roles, fakePermTypes{{MachineName: "article", Label: "Articles"}},
		fakePermFormats{{MachineName: "full_html", Label: "Full HTML"}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	catalog, err := s.Catalog(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	keys := catalogKeys(catalog)
	want := []string{"create article content", "edit own article content", "edit any article content", "delete any article content"}
	if !slices.Equal(keys["Articles"], want) {
		t.Fatalf("article permissions = %v", keys["Articles"])
	}
	for machine, label := range entity.ModuleNodeTypeLabels() {
		label = i18n.T(t.Context(), label)
		if !slices.Contains(keys[label], PermDeleteAny(machine)) {
			t.Errorf("module node type %s lacks its permissions: %v", machine, keys[label])
		}
	}
	if !slices.Equal(keys["Text formats"], []string{"use_text_format_full_html"}) {
		t.Fatalf("text formats = %v", keys["Text formats"])
	}
	if catalog[0].Label != "General" || !slices.Contains(keys["General"], PermAccessAdmin) || !slices.Contains(keys["General"], PermUseMaintenance) {
		t.Fatalf("general group = %+v", catalog[0])
	}
	// No formats - no empty group.
	s = NewPermissionService(roles, fakePermTypes{}, fakePermFormats{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if catalog, _ := s.Catalog(t.Context()); slices.ContainsFunc(catalog, func(g PermissionGroup) bool { return g.Label == "Text formats" }) {
		t.Fatal("an empty text formats group")
	}
	// A content types error is returned, not swallowed.
	s = NewPermissionService(roles, failingPermTypes{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := s.Catalog(t.Context()); err == nil {
		t.Fatal("expected the content types error")
	}
}

func TestPermissionCheckEdgeCases(t *testing.T) {
	s, roles := newTestPermissions()
	ctx := t.Context()
	// A non-bool value is not a grant.
	roles.roles[2].Permissions["moderate content"] = 1
	if s.HasPermission(ctx, &entity.User{ID: 1, Role: roles.roles[2]}, PermModerateContent) {
		t.Fatal("a non-bool value granted the permission")
	}
	// The base role is loaded once and cached.
	user := &entity.User{ID: 2}
	for range 5 {
		s.HasPermission(ctx, user, PermCreate("post"))
	}
	if roles.byNameN != 1 {
		t.Fatalf("base role loaded %d times, want 1", roles.byNameN)
	}
	// No base role - only the account's own role counts, no panic.
	delete(roles.roles, 1)
	s2, _ := NewPermissionService(roles, fakePermTypes{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil))), roles
	if s2.HasPermission(ctx, user, PermCreate("post")) {
		t.Fatal("granted without any role")
	}
	if !s2.HasPermission(ctx, &entity.User{ID: 3, Role: &entity.Role{Permissions: map[string]any{"view poll votes": true}}}, "view poll votes") {
		t.Fatal("own role grant lost without a base role")
	}
	// full_access in the account's own role covers anything, even
	// permissions outside the catalog.
	if !s.HasPermission(ctx, &entity.User{ID: 4, Role: roles.roles[3]}, "something new") {
		t.Fatal("full access")
	}
}

func TestPermissionSaveGrantsEdgeCases(t *testing.T) {
	s, roles := newTestPermissions()
	ctx := t.Context()
	before := time.Now()
	// Keys outside the catalog are not added from the form.
	if err := s.SaveGrants(ctx, 2, map[string]bool{"made up permission": true, PermPublishContent: true}); err != nil {
		t.Fatal(err)
	}
	mod := roles.roles[2]
	if HasGrant(mod, "made up permission") || !HasGrant(mod, PermPublishContent) {
		t.Fatalf("permissions = %v", mod.Permissions)
	}
	if mod.UpdatedAt.Before(before) {
		t.Fatal("UpdatedAt not set")
	}
	roles.byIDErr = errors.New("no role")
	if err := s.SaveGrants(ctx, 2, nil); err == nil {
		t.Fatal("expected the ByID error")
	}
	roles.byIDErr, roles.updateErr = nil, errors.New("write failed")
	if err := s.SaveGrants(ctx, 2, nil); err == nil {
		t.Fatal("expected the Update error")
	}
	if list, _ := s.Roles(ctx); len(list) != 3 || list[0].ID != 1 || list[2].ID != 3 {
		t.Fatalf("roles not sorted by ID: %+v", list)
	}
}

func TestRoleManagement(t *testing.T) {
	s, roles := newTestPermissions()
	ctx := t.Context()
	for _, tc := range []struct {
		name, label string
		want        error
	}{
		{"Forum Mod", "Forum moderator", ErrRoleName},
		{"1forum", "Forum moderator", ErrRoleName},
		{"forum_moderator", "  ", ErrRoleLabel},
		{"moderator", "Another moderator", ErrRoleExists},
	} {
		if _, err := s.CreateRole(ctx, tc.name, tc.label); !errors.Is(err, tc.want) {
			t.Errorf("CreateRole(%q, %q) = %v, want %v", tc.name, tc.label, err, tc.want)
		}
	}
	created, err := s.CreateRole(ctx, " forum_moderator ", " Forum moderator ")
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "forum_moderator" || created.Description != "Forum moderator" || len(created.Permissions) != 0 {
		t.Fatalf("created = %+v", created)
	}

	if err := s.RenameRole(ctx, created.ID, ""); !errors.Is(err, ErrRoleLabel) {
		t.Fatalf("empty label: %v", err)
	}
	if err := s.RenameRole(ctx, created.ID, "Forum"); err != nil || roles.roles[created.ID].Description != "Forum" || roles.roles[created.ID].Name != "forum_moderator" {
		t.Fatalf("rename: %v %+v", err, roles.roles[created.ID])
	}

	roles.users = map[int]int{2: 13}
	for _, tc := range []struct {
		id   int
		want error
	}{
		{1, ErrRoleProtected}, // the base role
		{3, ErrRoleProtected}, // full access
		{2, ErrRoleInUse},     // has users
		{created.ID, nil},
	} {
		if err := s.DeleteRole(ctx, tc.id); !errors.Is(err, tc.want) {
			t.Errorf("DeleteRole(%d) = %v, want %v", tc.id, err, tc.want)
		}
	}
	if _, ok := roles.roles[created.ID]; ok {
		t.Fatal("the role was not deleted")
	}
	if counts := s.RoleUserCounts(ctx, entity.Roles{{ID: 2}, {ID: 1}}); counts[2] != 13 || counts[1] != 0 {
		t.Fatalf("counts = %v", counts)
	}
}

func TestRolesPageRenders(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	type row struct {
		Role             entity.Role
		Users            int
		FullAccess, Base bool
	}
	data := map[string]any{"roles": []row{
		{Role: entity.Role{ID: 1, Name: "user", Description: "Registered"}, Users: 114, Base: true},
		{Role: entity.Role{ID: 2, Name: "moderator", Description: "Moderator"}, Users: 13},
		{Role: entity.Role{ID: 3, Name: "admin", Description: "Administrator"}, Users: 3, FullAccess: true},
		{Role: entity.Role{ID: 4, Name: "forum_moderator", Description: "Forum moderator"}},
	}, "error": "in-use"}
	var buf strings.Builder
	if err := svc.RenderNamed(&buf, "content", data, "admin/people/roles.html", "admin/alerts.html", "admin/icons.html"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`action="/admin/users/roles/4/delete"`, "has users", "cannot be deleted", "The role has users", `value="Forum moderator"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, `action="/admin/users/roles/2/delete"`) || strings.Contains(out, `action="/admin/users/roles/1/delete"`) {
		t.Error("a delete button for a role that can't be deleted")
	}
}

// TestMayAssignRole - giving a role grants its permissions: the
// registered users' role whoever administers users, another only who
// administers permissions, a full-access one only a full-access editor;
// and an account only one who may give its role changes.
func TestMayAssignRole(t *testing.T) {
	s, roles := newTestPermissions()
	ctx := t.Context()
	base, moderator, admin := roles.roles[1], roles.roles[2], roles.roles[3]
	usersOnly := &entity.User{ID: 20, Role: &entity.Role{ID: 4, Name: "people", Permissions: map[string]any{PermAdministerUsers: true}}}
	permissions := &entity.User{ID: 21, Role: &entity.Role{ID: 5, Name: "staff", Permissions: map[string]any{PermAdministerPermissions: true}}}
	full := &entity.User{ID: 22, Role: admin}
	for _, tc := range []struct {
		name   string
		editor *entity.User
		role   *entity.Role
		want   bool
	}{
		{"the base role, administering users", usersOnly, base, true},
		{"another role, administering users", usersOnly, moderator, false},
		{"another role, administering permissions", permissions, moderator, true},
		{"full access, administering permissions", permissions, admin, false},
		{"full access by full access", full, admin, true},
		{"no editor", nil, base, false},
	} {
		if got := MayAssignRole(ctx, s, tc.editor, tc.role); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
	if got := AssignableRoles(ctx, s, permissions, entity.Roles{*base, *moderator, *admin}); len(got) != 2 || got[1].ID != moderator.ID {
		t.Errorf("assignable by permissions: %+v", got)
	}
	member := &entity.User{ID: 23, Role: base}
	for _, tc := range []struct {
		name           string
		editor, target *entity.User
		want           bool
	}{
		{"a full-access account, administering permissions", permissions, full, false},
		{"a full-access account by full access", full, full, true},
		{"an account administering permissions, administering users", usersOnly, permissions, false},
		{"a member, administering users", usersOnly, member, true},
		{"an account administering users, administering permissions", permissions, usersOnly, true},
		{"one's own account", usersOnly, usersOnly, true},
	} {
		if got := MayManageAccount(ctx, s, tc.editor, tc.target); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}
