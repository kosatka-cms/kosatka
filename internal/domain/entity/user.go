package entity

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Users - a list of users.
type Users []User

// UserMetadataKeyRegistrationIP is stored in User.Metadata for self-service signup.
const UserMetadataKeyRegistrationIP = "registration_ip"

// UserMetadataKeyOAuthProvider records which OAuth provider created the account.
const UserMetadataKeyOAuthProvider = "oauth_provider"

// RegistrationIPFromMetadata returns the registration IP string for templates and admin UI.
func RegistrationIPFromMetadata(md map[string]any) string {
	if md == nil {
		return ""
	}
	v, ok := md[UserMetadataKeyRegistrationIP]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// User - a site account (a users row): its name, e-mail, password hash,
// role, status (UserStatus…), picture and profile.
type User struct {
	ID       int
	UUID     uuid.UUID
	Username string
	Slug     string
	// Avatar - the uploaded picture as pages show it (its image style's
	// address); empty - none (the templates show avatarPlaceholder).
	Avatar File
	// AvatarStored - the uploaded picture's stored URI (public://…), for
	// removing its file; "" - none.
	AvatarStored string
	Email        string
	PasswordHash string
	// Timezone - the user's IANA time zone, used when per-user time zones
	// are enabled (see sitetime.ForUser); empty means the site's.
	Timezone      string
	Metadata      map[string]any
	Signature     string
	Role          *Role
	RoleID        int // role_id from DB, used when Role edge not loaded
	Status        int // 0=blocked, 1=active, 2=pending admin approval
	EmailVerified bool
	// SessionVersion - the account's sessions carry it; a session with
	// another is over (UserRepository.BumpSessionVersion).
	SessionVersion   int
	AvatarUpdateHint string // "keep" (default), "clear", "set" - for Update
	LastActivity     time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// UserFilter - the user Find looks for: by e-mail address.
type UserFilter struct {
	Email string
}

// Admin people list sort keys (query param sort=...).
const (
	AdminPeopleSortCreated    = "created"
	AdminPeopleSortLastAccess = "last_access"
)

// User status constants (users.status).
const (
	UserStatusBlocked = 0
	UserStatusActive  = 1
	UserStatusPending = 2 // awaiting admin approval
)

// PasswordUnset - the password hash of an account whose password isn't
// set yet (an invitation not accepted): no password matches it.
const PasswordUnset = "!"

// AdminPeopleListOpts controls ordering and optional filter for UserRepository.ListForAdmin.
type AdminPeopleListOpts struct {
	SortBy string // AdminPeopleSortCreated or AdminPeopleSortLastAccess
	Desc   bool
	Status *int // if non-nil, filter by users.status (e.g. pending = 2)
	RoleID *int // if non-nil, filter by users.role_id
}

// UserRepository - storage port for the users.
type UserRepository interface {
	Find(ctx context.Context, filter UserFilter) (*User, error)
	// FindByEmailOrUsername loads a user where email or username matches identity (case-insensitive).
	FindByEmailOrUsername(ctx context.Context, identity string) (*User, error)
	ByID(ctx context.Context, id int) (*User, error)
	// ByIDs - the users with these ids, by id (a missing one is absent).
	ByIDs(ctx context.Context, ids []int) (map[int]User, error)
	Create(ctx context.Context, ct User) (*User, error)
	List(ctx context.Context) (*Users, error)
	// ListForAdmin returns users for the admin table, totalCount, effectivePage (after clamping), error.
	ListForAdmin(ctx context.Context, page int, perPage int, opts AdminPeopleListOpts) (*Users, int, int, error)
	Delete(ctx context.Context, id int) error
	Update(ctx context.Context, ct User) (*User, error)
	// Search - users whose username or e-mail contains query (the admin's
	// search).
	Search(ctx context.Context, query string, limit int) (*Users, error)
	// SearchActiveByName - active users whose username contains query
	// (with roles - of one of them): a member's search for another (an
	// e-mail address isn't matched, so it can't tell whether an address
	// has an account).
	SearchActiveByName(ctx context.Context, query string, roles []int, limit int) (*Users, error)
	BySlug(ctx context.Context, slug string) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	// ConfirmEmail marks the user's e-mail address confirmed.
	ConfirmEmail(ctx context.Context, userID int) error
	// CountWithStatus returns the number of users with the given status (e.g. UserStatusPending).
	CountWithStatus(ctx context.Context, status int) (int, error)
	// TouchAccessedAtIfStale sets accessed_at (LastActivity) to now only if it is nil or older than minInterval; does not change updated_at.
	TouchAccessedAtIfStale(ctx context.Context, userID int, minInterval time.Duration) error
	// SessionStateByID returns only users.status and session_version (the
	// per-request check of a session); ErrEntityNotFound if the user does
	// not exist.
	SessionStateByID(ctx context.Context, id int) (status, sessionVersion int, err error)
	// BumpSessionVersion raises the user's session version, ending every
	// session of the account; the new version.
	BumpSessionVersion(ctx context.Context, id int) (int, error)
}

// MinPasswordLength - the fewest characters (not bytes) of a password,
// wherever one is set.
const MinPasswordLength = 6

// AvatarPlaceholderURL - the engine's picture for a user without an avatar.
const AvatarPlaceholderURL = "/static/images/no_avatar.png"

// NormalizeEmail - an email as users.email stores it: trimmed and in lower
// case, so the unique index holds one address once on every database
// (PostgreSQL compares case-sensitively).
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// UserPathPrefix - the engine's path of user profiles: /users/<alias>
// (like /login or /admin, not a site setting). users.slug stores the
// path without the leading slash: "users/<alias>".
const UserPathPrefix = "users"

// UserSlug - the path of a user's profile for the username: the name with
// spaces as dashes under UserPathPrefix.
func UserSlug(username string) string {
	return UserSlugFromAlias(strings.ReplaceAll(strings.TrimSpace(username), " ", "-"))
}

// UserSlugFromAlias - the stored slug of the profile /users/<alias>.
func UserSlugFromAlias(alias string) string {
	return UserPathPrefix + "/" + alias
}

// UserAlias - the <alias> of a stored slug.
func UserAlias(slug string) string {
	return strings.TrimPrefix(slug, UserPathPrefix+"/")
}

// UserURL - the profile's URL for a stored slug.
func UserURL(slug string) string {
	return "/" + UserSlugFromAlias(UserAlias(slug))
}
