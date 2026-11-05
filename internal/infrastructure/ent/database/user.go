package database

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/user"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/userfieldvalue"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
)

// UserRepository is the ent-backed entity.UserRepository.
type UserRepository struct {
	userClient *generated.UserClient
	// userFields - the users' profile field values, deleted with a user.
	userFields *UserFieldRepository
}

// users - the users' client: the context's transaction's (a unit of
// work, GetClient), else the repository's own.
func (r UserRepository) users(ctx context.Context) *generated.UserClient {
	if tx, ok := ctx.Value(txKey{}).(*generated.Client); ok {
		return tx.User
	}
	return r.userClient
}

// NewUserRepository - the user repository on client; userFields (nil -
// none) holds the users' profile field values, deleted with a user.
func NewUserRepository(client *generated.UserClient, userFields *UserFieldRepository) *UserRepository {
	return &UserRepository{userClient: client, userFields: userFields}
}

// Find loads the user with filter.Email (emails are unique, stored in
// lower case).
func (r UserRepository) Find(ctx context.Context, filter entity.UserFilter) (*entity.User, error) {
	email := entity.NormalizeEmail(filter.Email)
	if email == "" {
		return nil, &generated.NotFoundError{}
	}
	res, err := r.users(ctx).Query().
		Where(user.Email(email)).
		WithRole().
		First(ctx)

	if err != nil {
		return nil, fmt.Errorf("UserRepository.Find: %w", err)
	}

	userEntity := dao.ToUserEntity(res)

	return &userEntity, nil
}

// FindByEmailOrUsername - see entity.UserRepository.FindByEmailOrUsername;
// an exact match first, apperrors.ErrNotFound if none matches.
func (r UserRepository) FindByEmailOrUsername(ctx context.Context, identity string) (*entity.User, error) {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return nil, fmt.Errorf("%w", apperrors.ErrNotFound)
	}
	// Sequential lookups: a single OR query can match two different users
	// (one user's email equals another's username), which yields
	// ent.NotSingularError and breaks login.
	attempts := []func() (*generated.User, error){
		func() (*generated.User, error) {
			return r.users(ctx).Query().Where(user.Email(entity.NormalizeEmail(identity))).WithRole().First(ctx)
		},
		func() (*generated.User, error) {
			return r.users(ctx).Query().Where(user.Username(identity)).WithRole().First(ctx)
		},
		func() (*generated.User, error) {
			return r.users(ctx).Query().Where(user.EmailEqualFold(identity)).WithRole().First(ctx)
		},
		func() (*generated.User, error) {
			return r.users(ctx).Query().Where(user.UsernameEqualFold(identity)).WithRole().First(ctx)
		},
	}
	for _, attempt := range attempts {
		res, err := attempt()
		if err == nil {
			u := dao.ToUserEntity(res)
			return &u, nil
		}
		if !generated.IsNotFound(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w", apperrors.ErrNotFound)
}

// BySlug - the user with slug, with the role; ErrEntityNotFound if there
// is none.
func (r UserRepository) BySlug(ctx context.Context, slug string) (*entity.User, error) {
	res, err := r.users(ctx).Query().
		Where(
			user.Slug(slug),
		).
		WithRole().
		First(ctx)

	if err != nil {
		return nil, notFoundAs(err, "user with slug %s", slug)
	}

	userEntity := dao.ToUserEntity(res)

	return &userEntity, nil
}

// ByID - the user id with its role; outside a transaction, once per
// request (entity.WithUserMemo).
func (r UserRepository) ByID(ctx context.Context, id int) (*entity.User, error) {
	_, inTx := ctx.Value(txKey{}).(*generated.Client)
	if u, ok := entity.MemoizedUser(ctx, id); ok && !inTx {
		return u, nil
	}
	res, err := r.users(ctx).Query().
		Where(user.ID(id)).
		WithRole().
		First(ctx)

	if err != nil {
		return nil, notFoundAs(err, "user with id %d", id)
	}

	requestEntity := dao.ToUserEntity(res)
	if !inTx {
		entity.MemoizeUser(ctx, requestEntity)
	}
	return &requestEntity, nil
}

// ByIDs - the users with these ids, by id (a missing one is absent).
func (r UserRepository) ByIDs(ctx context.Context, ids []int) (map[int]entity.User, error) {
	out := make(map[int]entity.User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.users(ctx).Query().Where(user.IDIn(ids...)).WithRole().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading users by id: %w", err)
	}
	for _, row := range rows {
		out[row.ID] = dao.ToUserEntity(row)
	}
	return out, nil
}

// Create adds the user (an empty e-mail address is stored as none) and
// returns it with its ID.
func (r UserRepository) Create(ctx context.Context, ct entity.User) (*entity.User, error) {
	now := time.Now()
	creator := r.users(ctx).Create().
		SetUsername(ct.Username).
		SetSlug(ct.Slug).
		SetPasswordHash(ct.PasswordHash).
		SetTimezone(ct.Timezone).
		SetStatus(ct.Status).
		SetEmailVerified(ct.EmailVerified).
		SetCreatedAt(now).
		SetUpdatedAt(now)

	if email := entity.NormalizeEmail(ct.Email); email != "" {
		creator = creator.SetEmail(email)
	}
	if ct.Role != nil {
		creator = creator.SetRoleID(ct.Role.ID)
	}
	if len(ct.Metadata) > 0 {
		creator = creator.SetMetadata(ct.Metadata)
	}
	if ct.Signature != "" {
		creator = creator.SetSignature(ct.Signature)
	}

	created, err := creator.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserRepository.Create: %w", err)
	}

	result := dao.ToUserEntity(created)
	return &result, nil
}

// Update saves ct's name, slug, password hash, timezone, signature,
// status, e-mail address and its confirmation, role and avatar (as
// AvatarUpdateHint says) over the user ct.ID and returns ct as given.
func (r UserRepository) Update(ctx context.Context, ct entity.User) (*entity.User, error) {
	upd := r.users(ctx).UpdateOneID(ct.ID).
		SetUsername(ct.Username).
		SetSlug(ct.Slug).
		SetPasswordHash(ct.PasswordHash).
		SetTimezone(ct.Timezone).
		SetSignature(ct.Signature).
		SetStatus(ct.Status).
		SetUpdatedAt(time.Now())

	// No email is NULL (the column is unique; "" isn't an address).
	if email := entity.NormalizeEmail(ct.Email); email != "" {
		upd = upd.SetEmail(email)
	} else {
		upd = upd.ClearEmail()
	}

	switch ct.AvatarUpdateHint {
	case "clear":
		upd = upd.ClearAvatar()
	case "set":
		if ct.Avatar.URI != "" {
			upd = upd.SetAvatar(ct.Avatar.URI)
		}
	}
	if ct.Role != nil {
		upd = upd.SetRoleID(ct.Role.ID)
	}
	upd = upd.SetEmailVerified(ct.EmailVerified)

	entity.ForgetMemoizedUsers(ctx)
	_, err := upd.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserRepository.Update: %w", err)
	}
	return &ct, nil
}

// List - every user, with the role.
func (r UserRepository) List(ctx context.Context) (*entity.Users, error) {
	users, err := r.users(ctx).
		Query().
		WithRole().
		All(ctx)

	if err != nil {
		return nil, err
	}

	var usersEntity entity.Users
	for _, u := range users {
		usersEntity = append(usersEntity, dao.ToUserEntity(u))
	}

	return &usersEntity, err
}

// CountWithStatus - see entity.UserRepository.CountWithStatus.
func (r UserRepository) CountWithStatus(ctx context.Context, status int) (int, error) {
	return r.users(ctx).Query().Where(user.Status(status)).Count(ctx)
}

// ListForAdmin - see entity.UserRepository.ListForAdmin.
func (r UserRepository) ListForAdmin(ctx context.Context, page int, perPage int, opts entity.AdminPeopleListOpts) (*entity.Users, int, int, error) {
	query := r.users(ctx).Query().WithRole()
	if opts.Status != nil {
		query = query.Where(user.Status(*opts.Status))
	}
	if opts.RoleID != nil {
		query = query.Where(user.RoleID(*opts.RoleID))
	}

	switch opts.SortBy {
	case entity.AdminPeopleSortLastAccess:
		if opts.Desc {
			query = query.Order(generated.Desc(user.FieldLastActivity))
		} else {
			query = query.Order(generated.Asc(user.FieldLastActivity))
		}
	default:
		if opts.Desc {
			query = query.Order(generated.Desc(user.FieldCreatedAt))
		} else {
			query = query.Order(generated.Asc(user.FieldCreatedAt))
		}
	}

	totalCount, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, page, fmt.Errorf("UserRepository.ListForAdmin: %w", err)
	}

	if perPage < 1 {
		perPage = 25
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * perPage

	query = query.Limit(perPage).Offset(offset)

	users, err := query.All(ctx)
	if err != nil {
		return nil, 0, page, fmt.Errorf("UserRepository.ListForAdmin: %w", err)
	}

	var usersEntity entity.Users
	for _, u := range users {
		usersEntity = append(usersEntity, dao.ToUserEntity(u))
	}

	return &usersEntity, totalCount, page, nil
}

// Delete deletes the user with id and the user's profile field values,
// in one transaction (no key ties the values to the user: a failure
// between the two would leave them behind).
func (r UserRepository) Delete(ctx context.Context, id int) error {
	entity.ForgetMemoizedUsers(ctx)
	if r.userFields == nil {
		if err := r.users(ctx).DeleteOneID(id).Exec(ctx); err != nil {
			return fmt.Errorf("UserRepository.Delete: %w", err)
		}
		return nil
	}
	return inTx(ctx, r.userFields.client, func(c *generated.Client) error {
		if _, err := c.UserFieldValue.Delete().Where(userfieldvalue.UserID(id)).Exec(ctx); err != nil {
			return fmt.Errorf("deleting user %d's field values: %w", id, err)
		}
		if err := c.User.DeleteOneID(id).Exec(ctx); err != nil {
			return fmt.Errorf("UserRepository.Delete: %w", err)
		}
		return nil
	})
}

// FindByUsername - the user named username, with the role;
// ErrEntityNotFound if there is none.
func (r UserRepository) FindByUsername(ctx context.Context, username string) (*entity.User, error) {
	res, err := r.users(ctx).Query().
		Where(user.Username(username)).
		WithRole().
		First(ctx)

	if err != nil {
		return nil, notFoundAs(err, "user with username %s", username)
	}

	userEntity := dao.ToUserEntity(res)
	return &userEntity, nil
}

// ConfirmEmail - see entity.UserRepository.ConfirmEmail.
func (r UserRepository) ConfirmEmail(ctx context.Context, userID int) error {
	entity.ForgetMemoizedUsers(ctx)
	_, err := r.users(ctx).UpdateOneID(userID).
		SetEmailVerified(true).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	return notFoundAs(err, "user with id %d", userID)
}

// TouchAccessedAtIfStale - see entity.UserRepository.TouchAccessedAtIfStale.
func (r UserRepository) TouchAccessedAtIfStale(ctx context.Context, userID int, minInterval time.Duration) error {
	now := time.Now()
	cutoff := now.Add(-minInterval)
	_, err := r.users(ctx).UpdateOneID(userID).
		Where(user.Or(user.LastActivityIsNil(), user.LastActivityLT(cutoff))).
		SetLastActivity(now).
		Save(ctx)
	if err != nil {
		if generated.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

// SessionStateByID - see entity.UserRepository.SessionStateByID.
func (r UserRepository) SessionStateByID(ctx context.Context, id int) (int, int, error) {
	u, err := r.users(ctx).Query().
		Where(user.ID(id)).
		Select(user.FieldStatus, user.FieldSessionVersion).
		Only(ctx)
	if err != nil {
		return 0, 0, notFoundAs(err, "user with id %d", id)
	}
	return u.Status, u.SessionVersion, nil
}

// BumpSessionVersion - see entity.UserRepository.BumpSessionVersion.
func (r UserRepository) BumpSessionVersion(ctx context.Context, id int) (int, error) {
	entity.ForgetMemoizedUsers(ctx)
	u, err := r.users(ctx).UpdateOneID(id).AddSessionVersion(1).Save(ctx)
	if err != nil {
		return 0, notFoundAs(err, "user with id %d", id)
	}
	return u.SessionVersion, nil
}

// Search - see entity.UserRepository.Search; at most limit (0 - 10).
func (r UserRepository) Search(ctx context.Context, query string, limit int) (*entity.Users, error) {
	if limit <= 0 {
		limit = 10
	}

	users, err := r.users(ctx).
		Query().
		Where(
			user.Or(
				user.UsernameContainsFold(query),
				user.EmailContainsFold(query),
			),
		).
		Limit(limit).
		All(ctx)

	if err != nil {
		return nil, err
	}

	var usersEntity entity.Users
	for _, u := range users {
		usersEntity = append(usersEntity, dao.ToUserEntity(u))
	}

	return &usersEntity, nil
}

// SearchActiveByName - see entity.UserRepository.SearchActiveByName; by
// name, at most limit (0 - 10).
func (r UserRepository) SearchActiveByName(ctx context.Context, query string, roles []int, limit int) (*entity.Users, error) {
	if limit <= 0 {
		limit = 10
	}
	where := []predicate.User{user.UsernameContainsFold(query), user.Status(entity.UserStatusActive)}
	if len(roles) > 0 {
		where = append(where, user.RoleIDIn(roles...))
	}
	users, err := r.users(ctx).Query().
		Where(where...).
		Order(generated.Asc(user.FieldUsername)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("searching active users by name: %w", err)
	}
	out := make(entity.Users, 0, len(users))
	for _, u := range users {
		out = append(out, dao.ToUserEntity(u))
	}
	return &out, nil
}
