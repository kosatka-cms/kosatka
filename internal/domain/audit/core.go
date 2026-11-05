package audit

import "github.com/kosatka-cms/kosatka/pkg/i18n"

// The core's groups of actions.
const (
	GroupContent  = "content"
	GroupComments = "comments"
	GroupUsers    = "users"
	GroupSettings = "settings"
	GroupLogins   = "logins"
)

// The core's action types.
const (
	NodeCreate = "node.create"
	NodeDelete = "node.delete"
	// NodeTrash, NodeRestore - content put in the trash, taken out of it.
	NodeTrash     = "node.trash"
	NodeRestore   = "node.restore"
	NodePublish   = "node.publish"
	NodeUnpublish = "node.unpublish"
	NodeAuthor    = "node.author"
	NodeAddress   = "node.address"

	CommentDelete = "comment.delete"
	// CommentApprove, CommentSpam - a comment published after approval,
	// marked as spam.
	CommentApprove = "comment.approve"
	CommentSpam    = "comment.spam"

	UserCreate   = "user.create"
	UserBlock    = "user.block"
	UserUnblock  = "user.unblock"
	UserActivate = "user.activate"
	UserDelete   = "user.delete"
	UserRole     = "user.role"
	// TwoFactorOn, TwoFactorOff - a user turned two-factor sign-in on or
	// off; TwoFactorReset - an administrator turned it off for them.
	TwoFactorOn     = "user.two_factor_on"
	TwoFactorOff    = "user.two_factor_off"
	TwoFactorReset  = "user.two_factor_reset"
	RoleCreate      = "role.create"
	RoleDelete      = "role.delete"
	RoleRename      = "role.rename"
	RolePermissions = "role.permissions"

	SettingsSave    = "settings.save"
	ModuleEnable    = "module.enable"
	ModuleDisable   = "module.disable"
	ModuleUninstall = "module.uninstall"

	Login       = "login"
	LoginFailed = "login.failed"
)

// The kinds of the core's objects (Object.Kind).
const (
	KindNode     = "node"
	KindComment  = "comment"
	KindUser     = "user"
	KindRole     = "role"
	KindModule   = "module"
	KindSettings = "settings"
)

func init() {
	for _, g := range []Group{
		{Name: GroupContent, Label: i18n.N("Content"), Description: i18n.N("Creating, deleting, publishing and unpublishing, a new author or address. Edits of the text and fields are in the content's history.")},
		{Name: GroupComments, Label: i18n.N("Comments"), Description: i18n.N("Deleting comments, with the replies to them.")},
		{Name: GroupUsers, Label: i18n.N("Users, roles and permissions"), Description: i18n.N("Creating, blocking, deleting, a new role; the roles' permissions - what was and what is.")},
		{Name: GroupSettings, Label: i18n.N("Settings and structure"), Description: i18n.N("Modules, content types and fields, Views, blocks, menus, text formats, the configuration pages.")},
		{Name: GroupLogins, Label: i18n.N("Sign-ins"), Description: i18n.N("Sign-ins of those with access to the admin area, and failed attempts under any name.")},
	} {
		RegisterGroup(g)
	}
	for _, t := range []Type{
		{Name: NodeCreate, Group: GroupContent, Label: i18n.N("Content created"), Tone: ToneCreate},
		{Name: NodeDelete, Group: GroupContent, Label: i18n.N("Content deleted"), Tone: ToneDelete},
		{Name: NodeTrash, Group: GroupContent, Label: i18n.N("Content moved to the trash"), Tone: ToneDelete},
		{Name: NodeRestore, Group: GroupContent, Label: i18n.N("Content restored from the trash"), Tone: ToneCreate},
		{Name: NodePublish, Group: GroupContent, Label: i18n.N("Content published"), Tone: ToneChange},
		{Name: NodeUnpublish, Group: GroupContent, Label: i18n.N("Content unpublished"), Tone: ToneChange},
		{Name: NodeAuthor, Group: GroupContent, Label: i18n.N("Author changed"), Tone: ToneChange},
		{Name: NodeAddress, Group: GroupContent, Label: i18n.N("Address changed"), Tone: ToneChange},
		{Name: CommentDelete, Group: GroupComments, Label: i18n.N("Comment deleted"), Tone: ToneDelete},
		{Name: CommentApprove, Group: GroupComments, Label: i18n.N("Comment approved"), Tone: ToneChange},
		{Name: CommentSpam, Group: GroupComments, Label: i18n.N("Comment marked as spam"), Tone: ToneDelete},
		{Name: UserCreate, Group: GroupUsers, Label: i18n.N("User created"), Tone: ToneCreate},
		{Name: UserBlock, Group: GroupUsers, Label: i18n.N("User blocked"), Tone: ToneAccess},
		{Name: UserUnblock, Group: GroupUsers, Label: i18n.N("User unblocked"), Tone: ToneAccess},
		{Name: UserActivate, Group: GroupUsers, Label: i18n.N("User activated"), Tone: ToneAccess},
		{Name: UserDelete, Group: GroupUsers, Label: i18n.N("User deleted"), Tone: ToneDelete},
		{Name: UserRole, Group: GroupUsers, Label: i18n.N("Role changed"), Tone: ToneAccess},
		{Name: TwoFactorOn, Group: GroupUsers, Label: i18n.N("Two-factor sign-in turned on"), Tone: ToneAccess},
		{Name: TwoFactorOff, Group: GroupUsers, Label: i18n.N("Two-factor sign-in turned off"), Tone: ToneAccess},
		{Name: TwoFactorReset, Group: GroupUsers, Label: i18n.N("Two-factor sign-in reset"), Tone: ToneAccess},
		{Name: RoleCreate, Group: GroupUsers, Label: i18n.N("Role created"), Tone: ToneCreate},
		{Name: RoleDelete, Group: GroupUsers, Label: i18n.N("Role deleted"), Tone: ToneDelete},
		{Name: RoleRename, Group: GroupUsers, Label: i18n.N("Role renamed"), Tone: ToneChange},
		{Name: RolePermissions, Group: GroupUsers, Label: i18n.N("Permissions changed"), Tone: ToneAccess},
		{Name: SettingsSave, Group: GroupSettings, Label: i18n.N("Settings changed"), Tone: ToneChange},
		{Name: ModuleEnable, Group: GroupSettings, Label: i18n.N("Module enabled"), Tone: ToneChange},
		{Name: ModuleDisable, Group: GroupSettings, Label: i18n.N("Module disabled"), Tone: ToneChange},
		{Name: ModuleUninstall, Group: GroupSettings, Label: i18n.N("Module uninstalled"), Tone: ToneDelete},
		{Name: Login, Group: GroupLogins, Label: i18n.N("Signed in"), Tone: ToneLogin},
		{Name: LoginFailed, Group: GroupLogins, Label: i18n.N("Failed sign-in"), Tone: ToneFail},
	} {
		RegisterType(t)
	}
}
