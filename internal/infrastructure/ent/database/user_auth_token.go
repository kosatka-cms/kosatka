package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/userauthtoken"
)

// UserAuthTokenRepository stores hashed one-time login and password-reset tokens.
type UserAuthTokenRepository struct {
	client *generated.Client
}

// NewUserAuthTokenRepository - the auth token repository on db.
func NewUserAuthTokenRepository(db *sql.DB) *UserAuthTokenRepository {
	return &UserAuthTokenRepository{client: clientFor(db)}
}

// InvalidateUserKind removes unused tokens of a kind for a user (e.g. before issuing a new reset).
func (r *UserAuthTokenRepository) InvalidateUserKind(ctx context.Context, userID int, kind string) error {
	_, err := GetClient(ctx, r.client).UserAuthToken.Delete().
		Where(userauthtoken.UserID(userID), userauthtoken.Kind(kind), userauthtoken.UsedAtIsNil()).
		Exec(ctx)
	return err
}

// Insert stores the hash of a token of kind for the user, valid until
// expiresAt.
func (r *UserAuthTokenRepository) Insert(ctx context.Context, userID int, tokenHash, kind string, expiresAt time.Time) error {
	return GetClient(ctx, r.client).UserAuthToken.Create().
		SetUserID(userID).SetTokenHash(tokenHash).SetKind(kind).SetExpiresAt(expiresAt).SetCreatedAt(time.Now()).
		Exec(ctx)
}

// Consume marks the newest matching unused token as used and returns user_id (single-use).
func (r *UserAuthTokenRepository) Consume(ctx context.Context, tokenHash, kind string) (userID int, ok bool, err error) {
	err = inTx(ctx, r.client, func(c *generated.Client) error {
		now := time.Now()
		usable := []predicate.UserAuthToken{userauthtoken.TokenHash(tokenHash), userauthtoken.Kind(kind), userauthtoken.UsedAtIsNil(), userauthtoken.ExpiresAtGT(now)}
		t, err := c.UserAuthToken.Query().Where(usable...).Order(generated.Desc(userauthtoken.FieldID)).First(ctx)
		if generated.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("UserAuthTokenRepository.Consume: %w", err)
		}
		n, err := c.UserAuthToken.Update().Where(usable...).SetUsedAt(now).Save(ctx)
		if err != nil || n == 0 {
			return err
		}
		userID, ok = t.UserID, true
		return nil
	})
	if err != nil || !ok {
		return 0, false, err
	}
	return userID, true, nil
}

// Valid - the user of an unused, unexpired token (left unused).
func (r *UserAuthTokenRepository) Valid(ctx context.Context, tokenHash, kind string) (userID int, ok bool, err error) {
	t, err := GetClient(ctx, r.client).UserAuthToken.Query().
		Where(userauthtoken.TokenHash(tokenHash), userauthtoken.Kind(kind), userauthtoken.UsedAtIsNil(), userauthtoken.ExpiresAtGT(time.Now())).
		Order(generated.Desc(userauthtoken.FieldID)).First(ctx)
	if generated.IsNotFound(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("UserAuthTokenRepository.Valid: %w", err)
	}
	return t.UserID, true, nil
}

// Pending - when each user's newest unused token of kind expires.
func (r *UserAuthTokenRepository) Pending(ctx context.Context, kind string, userIDs []int) (map[int]time.Time, error) {
	out := map[int]time.Time{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := GetClient(ctx, r.client).UserAuthToken.Query().
		Where(userauthtoken.Kind(kind), userauthtoken.UsedAtIsNil(), userauthtoken.UserIDIn(userIDs...)).
		Order(generated.Asc(userauthtoken.FieldID)).
		Select(userauthtoken.FieldUserID, userauthtoken.FieldExpiresAt).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserAuthTokenRepository.Pending: %w", err)
	}
	for _, t := range rows {
		out[t.UserID] = t.ExpiresAt
	}
	return out, nil
}
