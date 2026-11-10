package accounts

import (
	"context"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// EnsureUserRole returns the built-in role of registered users
// (entity.RoleUser), creating it on first use with no permissions
// (granted on /admin/users/permissions). Registration and social login
// share it.
func EnsureUserRole(ctx context.Context, repo entity.RoleRepository) (*entity.Role, error) {
	role, err := repo.ByName(ctx, entity.RoleUser)
	if err == nil && role != nil {
		return role, nil
	}
	now := time.Now()
	return repo.Create(ctx, entity.Role{
		Name:        entity.RoleUser,
		Description: i18n.T(ctx, "Registered user"),
		Permissions: map[string]any{},
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}
