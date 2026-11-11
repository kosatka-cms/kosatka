package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Permissions: the engine declares them (general ones
// here, four per node type generated from the content types), a role
// holds the granted ones in Role.Permissions, and the
// /admin/users/permissions page edits the role × permission matrix.
const (
	// PermFullAccess - every permission (the administrator role).
	PermFullAccess = entity.PermFullAccess

	PermAccessAdmin       = "access administration pages"
	PermAdministerContent = "administer content"
	PermAdministerUsers   = "administer users"
	// PermAdministerPermissions - the roles and the permission matrix: a
	// role holding it can grant itself any permission.
	PermAdministerPermissions = "administer permissions"
	PermAdministerComments    = "administer comments"
	// PermSkipCommentApproval - commenting without the new authors'
	// approval (NewAuthorCheck).
	PermSkipCommentApproval = "skip comment approval"
	PermViewUnpublished     = "view unpublished content"
	PermPublishContent      = "publish content"
	PermModerateContent     = "moderate content"
	PermAdministerUserFiles = "administer user files"
	PermUploadDocuments     = "upload documents"
	PermUseMaintenance      = "use_maintenance_mode"
	// PermCancelOwnAccount - deleting one's own account from the profile
	// (UserDeletionService.Cancel; never an administrator's).
	PermCancelOwnAccount = "cancel own account"
	// PermCopyContent - "Make a copy" of content one may edit and create
	// (DynamicContentController.Clone).
	PermCopyContent      = "copy content"
	permTextFormatPrefix = "use_text_format_"
)

// PermCreate (and PermEditOwn, PermEditAny, PermDeleteAny) - a node type's
// permissions ("create article content" etc.).
func PermCreate(nodeType string) string { return fmt.Sprintf("create %s content", nodeType) }

// PermEditOwn - the permission to edit one's own nodes of the type.
func PermEditOwn(nodeType string) string { return fmt.Sprintf("edit own %s content", nodeType) }

// PermEditAny - the permission to edit any node of the type.
func PermEditAny(nodeType string) string { return fmt.Sprintf("edit any %s content", nodeType) }

// PermDeleteAny - the permission to delete any node of the type.
func PermDeleteAny(nodeType string) string { return fmt.Sprintf("delete any %s content", nodeType) }

// PermUseTextFormat - choosing a text format when saving a field value.
func PermUseTextFormat(machineName string) string { return permTextFormatPrefix + machineName }

// PermissionDef - one permission as the matrix shows it.
type PermissionDef struct {
	Key   string
	Label string
}

// PermissionGroup - permissions under one heading.
type PermissionGroup struct {
	Label       string
	Permissions []PermissionDef
}

// permissionContentTypes / permissionTextFormats - what the catalog is
// generated from (ContentTypeRepository, TextFormatService).
type permissionContentTypes interface {
	ContentTypes(ctx context.Context) (entity.ContentTypeDefs, error)
}

type permissionTextFormats interface {
	TextFormats(ctx context.Context) (entity.TextFormatDefs, error)
}

// PermissionService - the permission catalog, the roles' grants and the
// checks. Every signed-in user also has the permissions of the base role
// entity.RoleUser ("authenticated user"), whatever role is set
// on the account; a role with full_access has every permission.
type PermissionService struct {
	roles        entity.RoleRepository
	contentTypes permissionContentTypes
	textFormats  permissionTextFormats
	logger       *slog.Logger

	mu       sync.Mutex
	baseRole *entity.Role
}

// NewPermissionService - the permission service on roles (the roles and
// their grants), contentTypes and textFormats (what the catalog's
// per-type and per-format permissions come from) and logger.
func NewPermissionService(roles entity.RoleRepository, contentTypes permissionContentTypes, textFormats permissionTextFormats, logger *slog.Logger) *PermissionService {
	return &PermissionService{roles: roles, contentTypes: contentTypes, textFormats: textFormats, logger: logger.With("component", "permission.service")}
}

// generalPermissions - the "General" group, the registered modules'
// permissions (entity.Module.Permissions) after the core's.
func generalPermissions(ctx context.Context) []PermissionDef {
	out := []PermissionDef{
		{PermAccessAdmin, i18n.T(ctx, "Access administration (/admin)")},
		{PermAdministerContent, i18n.T(ctx, "Administer content: URL, meta, status and author in the form, \"staff only\" blocks")},
		{PermAdministerUsers, i18n.T(ctx, "Administer users: accounts, user fields, account settings")},
		{PermAdministerPermissions, i18n.T(ctx, "Administer roles and permissions (lets a role grant itself any permission)")},
		{PermAdministerComments, i18n.T(ctx, "Administer comments: edit and delete any")},
		{PermSkipCommentApproval, i18n.T(ctx, "Comment without approval (no new authors' check)")},
		{PermViewUnpublished, i18n.T(ctx, "View unpublished content")},
		{PermBypassContentAccess, i18n.T(ctx, "View the content of every type, whoever its type is for")},
		{PermPublishContent, i18n.T(ctx, "Publish and unpublish")},
		{PermModerateContent, i18n.T(ctx, "Moderation (moderation queue)")},
		{PermViewRevisions, i18n.T(ctx, "View the history of changes of content they may edit")},
		{PermRevertRevisions, i18n.T(ctx, "Bring back an earlier version of content they may edit")},
		{PermCopyContent, i18n.T(ctx, "Make copies of content they may edit (and create)")},
		{PermAdministerUserFiles, i18n.T(ctx, "Administer users' files: view and manage any user's files")},
		{PermUploadDocuments, i18n.T(ctx, "Upload documents to files (without it - images only)")},
		{PermUseMaintenance, i18n.T(ctx, "Access the site in maintenance mode")},
		{PermCancelOwnAccount, i18n.T(ctx, "Delete their own account (what happens to the content - account settings)")},
	}
	for _, m := range entity.Modules {
		for _, p := range m.Permissions {
			out = append(out, PermissionDef{p.Key, i18n.T(ctx, p.Label)})
		}
	}
	return out
}

// Catalog - every permission, grouped: general, then per node type
// (content types by label, then the built-in modules' node types), then
// text formats.
func (s *PermissionService) Catalog(ctx context.Context) ([]PermissionGroup, error) {
	groups := []PermissionGroup{{Label: i18n.TC(ctx, "permission", "General"), Permissions: generalPermissions(ctx)}}
	groups[0].Permissions = slices.DeleteFunc(groups[0].Permissions, func(p PermissionDef) bool {
		// A module's permission is hidden while it is off (entity.Module.Permissions).
		module, ok := entity.ModuleOfPermission(p.Key)
		return ok && !ModuleEnabled(module)
	})

	types, err := s.contentTypes.ContentTypes(ctx)
	if err != nil {
		return nil, err
	}
	type nodeType struct{ machineName, label string }
	var nodeTypes []nodeType
	for _, t := range types {
		nodeTypes = append(nodeTypes, nodeType{t.MachineName, t.Label})
	}
	sort.Slice(nodeTypes, func(i, j int) bool { return nodeTypes[i].label < nodeTypes[j].label })
	var modules []nodeType
	for machineName, label := range entity.ModuleNodeTypeLabels() {
		if !NodeTypeEnabled(machineName) {
			continue
		}
		modules = append(modules, nodeType{machineName, i18n.T(ctx, label)})
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].label < modules[j].label })
	for _, t := range append(nodeTypes, modules...) {
		groups = append(groups, PermissionGroup{Label: t.label, Permissions: []PermissionDef{
			{PermCreate(t.machineName), i18n.TC(ctx, "permission", "Create")},
			{PermEditOwn(t.machineName), i18n.TC(ctx, "permission", "Edit own")},
			{PermEditAny(t.machineName), i18n.TC(ctx, "permission", "Edit any")},
			{PermDeleteAny(t.machineName), i18n.TC(ctx, "permission", "Delete any")},
		}})
	}

	if s.textFormats != nil {
		formats, err := s.textFormats.TextFormats(ctx)
		if err != nil {
			return nil, err
		}
		var defs []PermissionDef
		for _, f := range formats {
			defs = append(defs, PermissionDef{PermUseTextFormat(f.MachineName), f.Label})
		}
		if len(defs) > 0 {
			groups = append(groups, PermissionGroup{Label: i18n.T(ctx, "Text formats"), Permissions: defs})
		}
	}
	return groups, nil
}

// Roles - every role, by ID.
func (s *PermissionService) Roles(ctx context.Context) (entity.Roles, error) {
	roles, err := s.roles.List(ctx)
	if err != nil || roles == nil {
		return nil, err
	}
	out := append(entity.Roles{}, *roles...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// SaveGrants sets a role's grants for the catalog permissions to granted
// (missing - not granted); keys outside the catalog are kept as they are.
// A full_access role is left alone.
func (s *PermissionService) SaveGrants(ctx context.Context, roleID int, granted map[string]bool) error {
	role, err := s.roles.ByID(ctx, roleID)
	if err != nil {
		return err
	}
	if HasGrant(role, PermFullAccess) {
		return nil
	}
	catalog, err := s.Catalog(ctx)
	if err != nil {
		return err
	}
	perms := make(map[string]any, len(role.Permissions))
	maps.Copy(perms, role.Permissions)
	var changes []audit.Change
	yes, no := i18n.T(ctx, "yes"), i18n.T(ctx, "no")
	for _, g := range catalog {
		for _, p := range g.Permissions {
			had := HasGrant(role, p.Key)
			delete(perms, p.Key)
			if granted[p.Key] {
				perms[p.Key] = true
			}
			if had != granted[p.Key] {
				from, to := no, yes
				if had {
					from, to = yes, no
				}
				changes = append(changes, audit.Change{Field: i18n.T(ctx, p.Label), From: from, To: to})
			}
		}
	}
	role.Permissions = perms
	role.UpdatedAt = time.Now()
	if _, err := s.roles.Update(ctx, *role); err != nil {
		return err
	}
	if len(changes) > 0 {
		audit.Record(ctx, audit.Action{Type: audit.RolePermissions, Object: roleObject(*role, true), Changes: changes})
	}
	s.mu.Lock()
	s.baseRole = nil
	s.mu.Unlock()
	return nil
}

// Revoke removes the permission keys from every role (a module's
// uninstall: its permissions no longer exist).
func (s *PermissionService) Revoke(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	roles, err := s.roles.List(ctx)
	if err != nil {
		return err
	}
	for _, role := range *roles {
		changed := false
		perms := make(map[string]any, len(role.Permissions))
		maps.Copy(perms, role.Permissions)
		for _, k := range keys {
			if _, ok := perms[k]; ok {
				delete(perms, k)
				changed = true
			}
		}
		if !changed {
			continue
		}
		role.Permissions = perms
		role.UpdatedAt = time.Now()
		if _, err := s.roles.Update(ctx, role); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.baseRole = nil
	s.mu.Unlock()
	return nil
}

// HasPermission - whether user (nil - anonymous: no permissions) has
// key through their role or the base role.
func (s *PermissionService) HasPermission(ctx context.Context, user *entity.User, key string) bool {
	if user == nil {
		return false
	}
	if HasGrant(user.Role, PermFullAccess) || HasGrant(user.Role, key) {
		return true
	}
	return HasGrant(s.base(ctx), key)
}

// base - the base role (entity.RoleUser), cached until grants change.
func (s *PermissionService) base(ctx context.Context) *entity.Role {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.baseRole == nil {
		role, err := s.roles.ByName(ctx, entity.RoleUser)
		if err != nil {
			LogFallback(s.logger, err, "error loading the base role")
			return nil
		}
		s.baseRole = role
	}
	return s.baseRole
}

// Role management errors.
var (
	ErrRoleName      = errors.New("role machine name: lowercase latin letters, digits and _, starting with a letter")
	ErrRoleLabel     = errors.New("role label is required")
	ErrRoleExists    = errors.New("a role with this machine name exists")
	ErrRoleProtected = errors.New("the base role and full-access roles cannot be deleted")
	ErrRoleInUse     = errors.New("the role still has users")
)

var roleNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,49}$`)

// CreateRole adds a role without permissions (granted on the matrix).
func (s *PermissionService) CreateRole(ctx context.Context, name, label string) (*entity.Role, error) {
	name, label = strings.TrimSpace(name), strings.TrimSpace(label)
	if !roleNameRe.MatchString(name) {
		return nil, ErrRoleName
	}
	if label == "" {
		return nil, ErrRoleLabel
	}
	if existing, err := s.roles.ByName(ctx, name); err == nil && existing != nil {
		return nil, ErrRoleExists
	}
	now := time.Now()
	role, err := s.roles.Create(ctx, entity.Role{Name: name, Description: label, Permissions: map[string]any{}, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return nil, err
	}
	audit.Record(ctx, audit.Action{Type: audit.RoleCreate, Object: roleObject(*role, true)})
	return role, nil
}

// roleObject - the role r as an action log's object; withURL - it still
// exists.
func roleObject(r entity.Role, withURL bool) audit.Object {
	o := audit.Object{Kind: audit.KindRole, ID: r.Name, Label: r.Label()}
	if withURL {
		o.URL = "/admin/users/permissions"
	}
	return o
}

// RenameRole changes a role's label (the machine name stays).
func (s *PermissionService) RenameRole(ctx context.Context, id int, label string) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return ErrRoleLabel
	}
	role, err := s.roles.ByID(ctx, id)
	if err != nil {
		return err
	}
	from := role.Label()
	role.Description = label
	role.UpdatedAt = time.Now()
	if _, err := s.roles.Update(ctx, *role); err != nil {
		return err
	}
	if from != role.Label() {
		audit.Record(ctx, audit.Action{Type: audit.RoleRename, Object: roleObject(*role, true), Changes: []audit.Change{
			{Field: i18n.T(ctx, "Name"), From: from, To: role.Label()},
		}})
	}
	return nil
}

// DeleteRole removes a role that has no users; the base role and
// full-access roles are kept.
func (s *PermissionService) DeleteRole(ctx context.Context, id int) error {
	role, err := s.roles.ByID(ctx, id)
	if err != nil {
		return err
	}
	if role.Name == entity.RoleUser || HasGrant(role, PermFullAccess) {
		return ErrRoleProtected
	}
	n, err := s.roles.CountUsers(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrRoleInUse
	}
	if err := s.roles.Delete(ctx, id); err != nil {
		return err
	}
	audit.Record(ctx, audit.Action{Type: audit.RoleDelete, Object: roleObject(*role, false)})
	return nil
}

// RoleUserCounts - users per role ID.
func (s *PermissionService) RoleUserCounts(ctx context.Context, roles entity.Roles) map[int]int {
	out := make(map[int]int, len(roles))
	for _, r := range roles {
		n, err := s.roles.CountUsers(ctx, r.ID)
		if err != nil {
			LogFallback(s.logger, err, "error counting role users", "roleID", r.ID)
			continue
		}
		out[r.ID] = n
	}
	return out
}

// CreatableNodeTypes - the "create content" links for user: enabled
// content types (by label) and modules' node types with a create page,
// each where user may create it.
func (s *PermissionService) CreatableNodeTypes(ctx context.Context, user *entity.User) []entity.CreatableMaterialLink {
	if user == nil {
		return nil
	}
	types, err := s.contentTypes.ContentTypes(ctx)
	if err != nil {
		LogFallback(s.logger, err, "error loading content types for the create menu")
		return nil
	}
	types = slices.Clone(types)
	sort.SliceStable(types, func(i, j int) bool { return types[i].Label < types[j].Label })
	var out []entity.CreatableMaterialLink
	for _, t := range types {
		if t.Creatable() && s.HasPermission(ctx, user, PermCreate(t.MachineName)) {
			out = append(out, entity.CreatableMaterialLink{URL: "/create/" + t.MachineName, Label: t.Label})
		}
	}
	createPaths := entity.ModuleNodeTypeCreatePaths()
	var modules []string
	for machineName := range createPaths {
		if !NodeTypeEnabled(machineName) {
			continue
		}
		modules = append(modules, machineName)
	}
	sort.Strings(modules)
	for _, machineName := range modules {
		if s.HasPermission(ctx, user, PermCreate(machineName)) {
			out = append(out, entity.CreatableMaterialLink{URL: createPaths[machineName], Label: i18n.T(ctx, entity.ModuleNodeTypeLabels()[machineName])})
		}
	}
	return out
}

// HasGrant - whether role grants key (nil - no role, nothing).
func HasGrant(role *entity.Role, key string) bool {
	return role != nil && role.Grants(key)
}

// permissionChecker - the check of a user's permission (AuthService,
// PermissionService).
type permissionChecker interface {
	HasPermission(ctx context.Context, user *entity.User, permission string) bool
}

// MayAssignRole - whether editor may give an account role or take it
// away: the registered users' role (entity.RoleUser: every signed-in
// user has its permissions) whoever administers users; another role only
// who administers permissions, a full-access one only a full-access
// editor - giving a role grants its permissions.
func MayAssignRole(ctx context.Context, perms permissionChecker, editor *entity.User, role *entity.Role) bool {
	if role == nil || editor == nil {
		return false
	}
	if role.Name == entity.RoleUser {
		return true
	}
	if !perms.HasPermission(ctx, editor, PermAdministerPermissions) {
		return false
	}
	return !HasGrant(role, PermFullAccess) || HasGrant(editor.Role, PermFullAccess)
}

// AssignableRoles - the roles editor may assign (MayAssignRole).
func AssignableRoles(ctx context.Context, perms permissionChecker, editor *entity.User, roles entity.Roles) entity.Roles {
	out := make(entity.Roles, 0, len(roles))
	for _, role := range roles {
		if MayAssignRole(ctx, perms, editor, &role) {
			out = append(out, role)
		}
	}
	return out
}

// MayManageAccount - whether editor may change an account (its status,
// role, profile, password, two-factor sign-in; deleting it): their own,
// or one whose role editor may give (MayAssignRole) - taking over an
// account with more rights would give them to editor; a full-access
// account only a full-access editor.
func MayManageAccount(ctx context.Context, perms permissionChecker, editor, target *entity.User) bool {
	if target == nil || target.Role == nil || (editor != nil && editor.ID == target.ID) {
		return true
	}
	return MayAssignRole(ctx, perms, editor, target.Role)
}
