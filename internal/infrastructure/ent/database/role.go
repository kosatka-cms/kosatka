package database

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/role"
)

// NewRoleRepository - the role repository on client.
func NewRoleRepository(client *generated.RoleClient) *RoleRepository {
	return &RoleRepository{
		roleClient: client,
	}
}

// RoleRepository is the ent-backed entity.RoleRepository.
type RoleRepository struct {
	roleClient *generated.RoleClient
}

// roles - the roles' client of ctx's transaction (the unit of work's),
// else the repository's own.
func (r RoleRepository) roles(ctx context.Context) *generated.RoleClient {
	if tx, ok := ctx.Value(txKey{}).(*generated.Client); ok {
		return tx.Role
	}
	return r.roleClient
}

// Find - the first role matching filter (by name when it has one);
// ErrEntityNotFound if there is none.
func (r RoleRepository) Find(ctx context.Context, filter entity.RoleFilter) (*entity.Role, error) {
	query := r.roles(ctx).Query()

	if filter.Name != "" {
		query = query.Where(role.Name(filter.Name))
	}

	res, err := query.First(ctx)
	return one(res, err, dao.ToRoleEntity, "role matching filter %+v", filter)
}

// ByID - the role with id; ErrEntityNotFound if there is none.
func (r RoleRepository) ByID(ctx context.Context, id int) (*entity.Role, error) {
	res, err := r.roles(ctx).Get(ctx, id)
	return one(res, err, dao.ToRoleEntity, "role with id %d", id)
}

// ByName - the role named name; ErrEntityNotFound if there is none.
func (r RoleRepository) ByName(ctx context.Context, name string) (*entity.Role, error) {
	res, err := r.roles(ctx).Query().
		Where(role.Name(name)).
		First(ctx)

	if err != nil {
		return nil, notFoundAs(err, "role with name %s", name)
	}

	roleEntity := dao.ToRoleEntity(res)
	return &roleEntity, nil
}

// Create adds the role and returns it with its ID.
func (r RoleRepository) Create(ctx context.Context, roleEntity entity.Role) (*entity.Role, error) {
	created, err := r.roles(ctx).
		Create().
		SetName(roleEntity.Name).
		SetDescription(roleEntity.Description).
		SetPermissions(roleEntity.Permissions).
		SetCreatedAt(roleEntity.CreatedAt).
		SetUpdatedAt(roleEntity.UpdatedAt).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	result := dao.ToRoleEntity(created)
	return &result, nil
}

// Update saves the role's name, description and permissions over the role
// with its ID and returns it.
func (r RoleRepository) Update(ctx context.Context, roleEntity entity.Role) (*entity.Role, error) {
	updated, err := r.roles(ctx).
		UpdateOneID(roleEntity.ID).
		SetName(roleEntity.Name).
		SetDescription(roleEntity.Description).
		SetPermissions(roleEntity.Permissions).
		SetUpdatedAt(roleEntity.UpdatedAt).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	result := dao.ToRoleEntity(updated)
	return &result, nil
}

// List - every role.
func (r RoleRepository) List(ctx context.Context) (*entity.Roles, error) {
	res, err := r.roles(ctx).Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("RoleRepository.List: %w", err)
	}

	roles := dao.ToRolesEntity(res)
	return &roles, nil
}

// Delete deletes the role with id.
func (r RoleRepository) Delete(ctx context.Context, id int) error {
	return r.roles(ctx).DeleteOneID(id).Exec(ctx)
}

// CountUsers - see entity.RoleRepository.CountUsers.
func (r RoleRepository) CountUsers(ctx context.Context, id int) (int, error) {
	return r.roles(ctx).Query().Where(role.ID(id)).QueryUsers().Count(ctx)
}
