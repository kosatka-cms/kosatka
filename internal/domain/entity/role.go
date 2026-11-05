package entity

import (
	"context"
	"time"
)

// Role represents a user role with permissions
type Role struct {
	ID          int
	Name        string
	Description string
	Permissions map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Label - the role's readable name (Description, set on the roles
// page), or its machine name when it has none: what the interface
// shows wherever roles are listed.
func (r Role) Label() string {
	if r.Description != "" {
		return r.Description
	}
	return r.Name
}

// Roles is a slice of Role
type Roles []Role

// RoleFilter defines filters for role queries
type RoleFilter struct {
	Name string
}

// RoleRepository defines the interface for role data access
type RoleRepository interface {
	Find(ctx context.Context, filter RoleFilter) (*Role, error)
	ByID(ctx context.Context, id int) (*Role, error)
	ByName(ctx context.Context, name string) (*Role, error)
	Create(ctx context.Context, role Role) (*Role, error)
	List(ctx context.Context) (*Roles, error)
	Delete(ctx context.Context, id int) error
	Update(ctx context.Context, role Role) (*Role, error)
	// CountUsers - how many users have the role.
	CountUsers(ctx context.Context, id int) (int, error)
}

// Common role names
const (
	RoleUser      = "user"
	RoleModerator = "moderator"
	RoleAdmin     = "admin"
	// RoleAnonymous - the pseudo role of guests (not signed in), for
	// conditions like block visibility; never stored on a user.
	RoleAnonymous = "anonymous"
)

// UserRoleNames - the roles a visitor has for role conditions: guests -
// RoleAnonymous; signed-in users - the base role plus their own.
func UserRoleNames(user *User) []string {
	if user == nil {
		return []string{RoleAnonymous}
	}
	names := []string{RoleUser}
	if user.Role != nil && user.Role.Name != RoleUser {
		names = append(names, user.Role.Name)
	}
	return names
}
