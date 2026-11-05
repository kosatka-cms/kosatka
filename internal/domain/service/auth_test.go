package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// testNodeTypes - node types used by the permission tests: content types
// plus the forum/poll modules.
var testNodeTypes = []string{"article", "group", "event", "item", "kind", "post", "product", "page", "video", "story", "news", "forum", "poll"}

// migration208Roles - the default roles with their default grants; the
// tests below assert the behaviour the role-based checks had.
func migration208Roles() map[string]*entity.Role {
	user := map[string]any{}
	for _, t := range []string{"article", "group", "event", "item", "kind", "post"} {
		user[PermCreate(t)] = true
	}
	// Moderators also administer content.
	moderator := map[string]any{PermViewUnpublished: true, PermPublishContent: true, PermModerateContent: true, "manage quotes": true, "view poll votes": true, PermAdministerContent: true}
	for _, t := range testNodeTypes {
		user[PermEditOwn(t)] = true
		for _, p := range []string{PermCreate(t), PermEditOwn(t), PermEditAny(t), PermDeleteAny(t)} {
			moderator[p] = true
		}
	}
	return map[string]*entity.Role{
		"user":      {ID: 1, Name: "user", Permissions: user},
		"moderator": {ID: 2, Name: "moderator", Permissions: moderator},
		"admin":     {ID: 3, Name: "admin", Permissions: map[string]any{PermFullAccess: true}},
	}
}

var testRoles = migration208Roles()

func adminUser() *entity.User     { return &entity.User{ID: 1, Role: testRoles["admin"]} }
func moderatorUser() *entity.User { return &entity.User{ID: 2, Role: testRoles["moderator"]} }
func regularUser(id int) *entity.User {
	return &entity.User{ID: id, Role: testRoles["user"]}
}
func noRoleUser(id int) *entity.User { return &entity.User{ID: id} }

// newPermAuth - an AuthService checking permissions against the default
// roles.
func newPermAuth() *DefaultAuthService {
	roles := &fakePermRoles{roles: map[int]*entity.Role{}}
	for _, r := range testRoles {
		roles.roles[r.ID] = r
	}
	s := &DefaultAuthService{}
	s.SetPermissions(NewPermissionService(roles, fakePermTypes{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil))))
	return s
}

func TestCanViewNode(t *testing.T) {
	s := newPermAuth()
	author := regularUser(10)
	other := regularUser(20)

	cases := []struct {
		name string
		node entity.Node
		user *entity.User
		want bool
	}{
		{"published, no moderation, anonymous", entity.Node{Type: "article", Status: 1, OnModeration: 0, CreatedBy: *author}, nil, true},
		{"unpublished, anonymous", entity.Node{Type: "article", Status: 0, CreatedBy: *author}, nil, false},
		{"unpublished, admin", entity.Node{Type: "article", Status: 0, CreatedBy: *author}, adminUser(), true},
		{"unpublished, moderator", entity.Node{Type: "article", Status: 0, CreatedBy: *author}, moderatorUser(), true},
		{"pending own content, author", entity.Node{Type: "article", Status: 1, OnModeration: 1, CreatedBy: *author}, author, true},
		{"pending content, different user", entity.Node{Type: "article", Status: 1, OnModeration: 1, CreatedBy: *author}, other, false},
		{"pending content, anonymous", entity.Node{Type: "article", Status: 1, OnModeration: 1, CreatedBy: *author}, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.CanViewNode(t.Context(), &tc.node, tc.user); got != tc.want {
				t.Errorf("CanViewNode() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanEditNode(t *testing.T) {
	s := newPermAuth()
	author := regularUser(10)
	other := regularUser(20)

	cases := []struct {
		name string
		node entity.Node
		user *entity.User
		want bool
	}{
		{"anonymous", entity.Node{Type: "article", Status: 1, CreatedBy: *author}, nil, false},
		{"admin can always edit", entity.Node{Type: "article", Status: 1, CreatedBy: *author}, adminUser(), true},
		{"moderator can always edit", entity.Node{Type: "article", Status: 1, CreatedBy: *author}, moderatorUser(), true},
		{"author can edit own published content", entity.Node{Type: "article", Status: 1, CreatedBy: *author}, author, true},
		{"author cannot edit own unpublished content", entity.Node{Type: "article", Status: 0, CreatedBy: *author}, author, false},
		{"non-author cannot edit", entity.Node{Type: "article", Status: 1, CreatedBy: *author}, other, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.CanEditNode(t.Context(), &tc.node, tc.user); got != tc.want {
				t.Errorf("CanEditNode() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanDeleteNode(t *testing.T) {
	s := newPermAuth()
	author := regularUser(10)
	node := entity.Node{Type: "article", Status: 1, CreatedBy: *author}

	cases := []struct {
		name string
		user *entity.User
		want bool
	}{
		{"anonymous", nil, false},
		{"admin", adminUser(), true},
		{"moderator", moderatorUser(), true},
		{"author cannot delete their own content", author, false},
		{"other regular user", regularUser(20), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.CanDeleteNode(t.Context(), &node, tc.user); got != tc.want {
				t.Errorf("CanDeleteNode() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanCreateNode(t *testing.T) {
	s := newPermAuth()
	cases := []struct {
		name        string
		contentType entity.ContentType
		user        *entity.User
		want        bool
	}{
		{"anonymous", entity.ContentType("article"), nil, false},
		{"admin can create anything", entity.ContentType("product"), adminUser(), true},
		{"moderator can create anything", entity.ContentType("product"), moderatorUser(), true},
		{"regular user, user-creatable type", entity.ContentType("article"), regularUser(1), true},
		{"regular user, admin-only type", entity.ContentType("product"), regularUser(1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.CanCreateNode(t.Context(), tc.contentType, tc.user); got != tc.want {
				t.Errorf("CanCreateNode(%s) = %v, want %v", tc.contentType, got, tc.want)
			}
		})
	}
}

func TestCanPublishNode(t *testing.T) {
	s := newPermAuth()
	cases := []struct {
		name string
		user *entity.User
		want bool
	}{
		{"anonymous", nil, false},
		{"admin", adminUser(), true},
		{"moderator", moderatorUser(), true},
		{"regular user", regularUser(1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.CanPublishNode(t.Context(), tc.user); got != tc.want {
				t.Errorf("CanPublishNode() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanUseSiteInMaintenanceMode(t *testing.T) {
	s := newPermAuth()
	withPermission := &entity.User{ID: 1, Role: &entity.Role{
		Name:        "user",
		Permissions: map[string]any{"use_maintenance_mode": true},
	}}
	withoutPermission := &entity.User{ID: 2, Role: &entity.Role{
		Name:        "user",
		Permissions: map[string]any{"use_maintenance_mode": false},
	}}

	cases := []struct {
		name string
		user *entity.User
		want bool
	}{
		{"anonymous", nil, false},
		{"admin always allowed", adminUser(), true},
		{"no role", noRoleUser(3), false},
		{"role without permissions map", regularUser(4), false},
		{"explicit permission granted", withPermission, true},
		{"explicit permission denied", withoutPermission, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.CanUseSiteInMaintenanceMode(t.Context(), tc.user); got != tc.want {
				t.Errorf("CanUseSiteInMaintenanceMode() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCanUseTextFormat_ScriptFormatIsAdminOnlyRegardlessOfPermission guards
// the security boundary for htmlsanitize.SanitizeWithFormat's script/style
// passthrough: such a format stays admin-only even if a role is granted
// "use_text_format_<name>".
func TestCanUseTextFormat_ScriptFormatIsAdminOnlyRegardlessOfPermission(t *testing.T) {
	s := newPermAuth()
	scriptFormat := entity.TextFormatDef{MachineName: "unsafe_html", AllowedTags: "script style div a img"}
	safeFormat := entity.TextFormatDef{MachineName: "basic_html", AllowedTags: "p a em strong"}
	grantedForBoth := &entity.User{ID: 1, Role: &entity.Role{
		Name: "user",
		Permissions: map[string]any{
			"use_text_format_unsafe_html": true,
			"use_text_format_basic_html":  true,
		},
	}}

	cases := []struct {
		name   string
		user   *entity.User
		format entity.TextFormatDef
		want   bool
	}{
		{"admin always allowed, script format", adminUser(), scriptFormat, true},
		{"admin always allowed, safe format", adminUser(), safeFormat, true},
		{"non-admin with explicit grant - safe format allowed", grantedForBoth, safeFormat, true},
		{"non-admin with explicit grant - script format still denied", grantedForBoth, scriptFormat, false},
		{"anonymous", nil, scriptFormat, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.CanUseTextFormat(t.Context(), tc.user, tc.format); got != tc.want {
				t.Errorf("CanUseTextFormat() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Permissions are per node type: a role can get one type without the
// others; "edit own" needs its own grant; a user without a role has the
// base (user) role.
func TestNodePermissionsPerType(t *testing.T) {
	forumModerator := &entity.Role{ID: 4, Name: "forum_moderator", Permissions: map[string]any{PermEditAny("forum"): true, PermDeleteAny("forum"): true}}
	roles := &fakePermRoles{roles: map[int]*entity.Role{
		1: {ID: 1, Name: entity.RoleUser, Permissions: map[string]any{PermCreate("article"): true, PermEditOwn("article"): true}},
		4: forumModerator,
	}}
	s := &DefaultAuthService{}
	s.SetPermissions(NewPermissionService(roles, fakePermTypes{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil))))
	ctx := t.Context()
	fm := &entity.User{ID: 5, Role: forumModerator}
	author := &entity.User{ID: 6}
	topic := &entity.Node{Type: "forum", Status: 1, CreatedBy: entity.User{ID: 99}}
	article := &entity.Node{Type: "article", Status: 1, CreatedBy: entity.User{ID: 99}}
	ownArticle := &entity.Node{Type: "article", Status: 1, CreatedBy: *author}
	ownProduct := &entity.Node{Type: "product", Status: 1, CreatedBy: *author}
	for _, tc := range []struct {
		name string
		got  bool
		want bool
	}{
		{"forum moderator edits any forum topic", s.CanEditNode(ctx, topic, fm), true},
		{"forum moderator deletes any forum topic", s.CanDeleteNode(ctx, topic, fm), true},
		{"forum moderator cannot edit someone's article", s.CanEditNode(ctx, article, fm), false},
		{"forum moderator cannot delete a article", s.CanDeleteNode(ctx, article, fm), false},
		{"no-role user creates articles via the base role", s.CanCreateNode(ctx, "article", author), true},
		{"no-role user edits own article (edit own article)", s.CanEditNode(ctx, ownArticle, author), true},
		{"no edit own product - cannot edit own product", s.CanEditNode(ctx, ownProduct, author), false},
		{"no publish permission", s.CanPublishNode(ctx, fm), false},
		{"nobody without a permission service", (&DefaultAuthService{}).CanCreateNode(ctx, "article", adminUser()), false},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

// The "Create content" menu: enabled content types the user may create,
// by label; modules with a create page (poll) when permitted.
func TestCreatableNodeTypes(t *testing.T) {
	roles := &fakePermRoles{roles: map[int]*entity.Role{}}
	for _, r := range testRoles {
		roles.roles[r.ID] = r
	}
	types := fakePermTypes{
		{MachineName: "article", Label: "Articles", Status: 1},
		{MachineName: "post", Label: "Posts", Status: 1},
		{MachineName: "product", Label: "Products", Status: 1},
		{MachineName: "group", Label: "Groups", Status: 0}, // disabled
	}
	p := NewPermissionService(roles, types, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	labels := func(links []entity.CreatableMaterialLink) string {
		var out []string
		for _, l := range links {
			out = append(out, l.Label+"="+l.URL)
		}
		return strings.Join(out, ",")
	}
	ctx := t.Context()
	if got := labels(p.CreatableNodeTypes(ctx, regularUser(1))); got != "Articles=/create/article,Posts=/create/post" {
		t.Errorf("regular user: %s", got)
	}
	if got := labels(p.CreatableNodeTypes(ctx, moderatorUser())); got != "Articles=/create/article,Posts=/create/post,Products=/create/product,Poll=/create/poll" {
		t.Errorf("moderator: %s", got)
	}
	if got := p.CreatableNodeTypes(ctx, nil); got != nil {
		t.Errorf("anonymous: %v", got)
	}
}

// TestAdminPermissionsMatchOldRoleChecks - admin permissions give the same
// answers as the equivalent IsAdmin/IsModerator role checks for the
// default roles.
func TestAdminPermissionsMatchOldRoleChecks(t *testing.T) {
	s := newPermAuth()
	ctx := t.Context()
	admin, moderator, user, none := adminUser(), moderatorUser(), regularUser(5), noRoleUser(6)
	for _, tc := range []struct {
		perm                           string
		admin, moderator, user, noRole bool
		was                            string
	}{
		{PermAccessAdmin, true, false, false, false, "IsAdmin (admin pages, admin bar)"},
		{PermAdministerUsers, true, false, false, false, "IsAdmin (people, other profiles)"},
		{PermAdministerComments, true, false, false, false, "role name admin (others' comments)"},
		{PermModerateContent, true, true, false, false, "admin or moderator (moderation queue)"},
		{PermAdministerContent, true, true, false, false, "admin or moderator (node settings, staff blocks, poll status)"},
		{"manage quotes", true, true, false, false, "admin or moderator (quotes)"},
		{"view poll votes", true, true, false, false, "admin or moderator (poll votes)"},
		{PermViewUnpublished, true, true, false, false, "admin or moderator (unpublished galleries, View preview)"},
		{PermEditAny("forum"), true, true, false, false, "admin or moderator (others' forum topics)"},
		{PermDeleteAny("forum"), true, true, false, false, "admin or moderator (delete forum topic)"},
	} {
		for _, c := range []struct {
			name string
			u    *entity.User
			want bool
		}{{"admin", admin, tc.admin}, {"moderator", moderator, tc.moderator}, {"user", user, tc.user}, {"no role", none, tc.noRole}} {
			if got := s.HasPermission(ctx, c.u, tc.perm); got != c.want {
				t.Errorf("%s for %s = %v, want %v (was: %s)", tc.perm, c.name, got, c.want, tc.was)
			}
		}
		if s.HasPermission(ctx, nil, tc.perm) {
			t.Errorf("%s for anonymous", tc.perm)
		}
	}
}

// verifyPolicy - account settings that require a confirmed e-mail or
// not, or can't be read.
type verifyPolicy struct {
	require bool
	err     error
}

func (p verifyPolicy) RequireEmailVerification(context.Context) (bool, error) {
	return p.require, p.err
}

// TestValidateUserForLogin - an unconfirmed e-mail stops a sign-in only
// when the account settings require a confirmed one; settings that can't
// be read stop it with ErrLoginFailed instead of letting it through.
func TestValidateUserForLogin(t *testing.T) {
	ctx := context.Background()
	unconfirmed := &entity.User{ID: 7, Status: entity.UserStatusActive}
	confirmed := &entity.User{ID: 8, Status: entity.UserStatusActive, EmailVerified: true}
	check := func(policy emailVerificationPolicy, u *entity.User) error {
		return (&DefaultAuthService{policy: policy}).validateUserForLogin(ctx, u)
	}
	if err := check(nil, unconfirmed); err != nil {
		t.Errorf("no settings: %v", err)
	}
	if err := check(verifyPolicy{}, unconfirmed); err != nil {
		t.Errorf("not required: %v", err)
	}
	if err := check(verifyPolicy{require: true}, unconfirmed); err == nil {
		t.Error("required, unconfirmed: signed in")
	}
	if err := check(verifyPolicy{require: true}, confirmed); err != nil {
		t.Errorf("required, confirmed: %v", err)
	}
	if err := check(verifyPolicy{err: errors.New("db down")}, confirmed); !errors.Is(err, ErrLoginFailed) {
		t.Errorf("settings unread: %v, want ErrLoginFailed", err)
	}
}
