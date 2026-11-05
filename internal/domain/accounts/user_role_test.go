package accounts

import (
	"context"
	"errors"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type userRoleRepo struct {
	entity.RoleRepository
	existing *entity.Role
	created  []entity.Role
}

func (r *userRoleRepo) ByName(_ context.Context, name string) (*entity.Role, error) {
	if r.existing != nil && r.existing.Name == name {
		return r.existing, nil
	}
	return nil, errors.New("not found")
}

func (r *userRoleRepo) Create(_ context.Context, role entity.Role) (*entity.Role, error) {
	role.ID = 2
	r.created = append(r.created, role)
	return &role, nil
}

// TestEnsureUserRole - the registered users' role is reused when it exists
// and created without permissions otherwise.
func TestEnsureUserRole(t *testing.T) {
	repo := &userRoleRepo{existing: &entity.Role{ID: 7, Name: entity.RoleUser}}
	if role, err := EnsureUserRole(context.Background(), repo); err != nil || role.ID != 7 || len(repo.created) != 0 {
		t.Fatalf("existing role: %+v %v %d", role, err, len(repo.created))
	}

	repo = &userRoleRepo{}
	role, err := EnsureUserRole(context.Background(), repo)
	if err != nil || role.Name != entity.RoleUser || len(repo.created) != 1 {
		t.Fatalf("new role: %+v %v", role, err)
	}
	if role.Description != "Registered user" || len(role.Permissions) != 0 {
		t.Errorf("new role fields: %+v", role)
	}
}
