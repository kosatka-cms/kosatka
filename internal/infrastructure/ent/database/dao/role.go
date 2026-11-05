package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToRoleEntity converts generated.Role to entity.Role.
func ToRoleEntity(r *generated.Role) entity.Role {
	if r == nil {
		return entity.Role{}
	}

	return entity.Role{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Permissions: r.Permissions,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// ToRolesEntity converts generated roles to entity.Roles.
func ToRolesEntity(roles []*generated.Role) entity.Roles {
	var result entity.Roles
	for _, r := range roles {
		result = append(result, ToRoleEntity(r))
	}
	return result
}
