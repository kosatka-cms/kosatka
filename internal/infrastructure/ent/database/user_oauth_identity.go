package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	generated "github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/useroauthidentity"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
)

// UserOAuthIdentityRepository implements entity.UserOAuthIdentityRepository:
// the provider accounts connected to the users'.
type UserOAuthIdentityRepository struct {
	client *generated.Client
}

// NewUserOAuthIdentityRepository - the connected provider account
// repository on client.
func NewUserOAuthIdentityRepository(client *generated.Client) *UserOAuthIdentityRepository {
	return &UserOAuthIdentityRepository{client: client}
}

func toOAuthIdentityEntity(row *generated.UserOAuthIdentity) *entity.UserOAuthIdentity {
	out := &entity.UserOAuthIdentity{
		ID:             row.ID,
		UserID:         row.UserID,
		Provider:       row.Provider,
		ProviderUserID: row.ProviderUserID,
		CreatedAt:      row.CreatedAt,
	}
	if row.Email != nil {
		out.Email = *row.Email
	}
	return out
}

// FindByProviderUser - the connection of the provider's account
// providerUserID; apperrors.ErrNotFound if there is none.
func (r *UserOAuthIdentityRepository) FindByProviderUser(ctx context.Context, provider, providerUserID string) (*entity.UserOAuthIdentity, error) {
	row, err := GetClient(ctx, r.client).UserOAuthIdentity.Query().
		Where(
			useroauthidentity.ProviderEQ(provider),
			useroauthidentity.ProviderUserIDEQ(providerUserID),
		).
		Only(ctx)
	if err != nil {
		if generated.IsNotFound(err) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return toOAuthIdentityEntity(row), nil
}

// FindByUserAndProvider - the user's connection to the provider;
// apperrors.ErrNotFound if there is none.
func (r *UserOAuthIdentityRepository) FindByUserAndProvider(ctx context.Context, userID int, provider string) (*entity.UserOAuthIdentity, error) {
	row, err := GetClient(ctx, r.client).UserOAuthIdentity.Query().
		Where(
			useroauthidentity.UserIDEQ(userID),
			useroauthidentity.ProviderEQ(provider),
		).
		Only(ctx)
	if err != nil {
		if generated.IsNotFound(err) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return toOAuthIdentityEntity(row), nil
}

// Create stores the connection identity (an empty e-mail address - none)
// and returns it with its ID.
func (r *UserOAuthIdentityRepository) Create(ctx context.Context, identity entity.UserOAuthIdentity) (*entity.UserOAuthIdentity, error) {
	creator := GetClient(ctx, r.client).UserOAuthIdentity.Create().
		SetUserID(identity.UserID).
		SetProvider(identity.Provider).
		SetProviderUserID(identity.ProviderUserID).
		SetCreatedAt(identity.CreatedAt)
	if e := strings.TrimSpace(identity.Email); e != "" {
		creator = creator.SetEmail(e)
	}
	row, err := creator.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserOAuthIdentityRepository.Create: %w", err)
	}
	return toOAuthIdentityEntity(row), nil
}

// ByUser - see entity.UserOAuthIdentityRepository.ByUser; by provider.
func (r *UserOAuthIdentityRepository) ByUser(ctx context.Context, userID int) ([]entity.UserOAuthIdentity, error) {
	rows, err := GetClient(ctx, r.client).UserOAuthIdentity.Query().
		Where(useroauthidentity.UserIDEQ(userID)).
		Order(generated.Asc(useroauthidentity.FieldProvider)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserOAuthIdentityRepository.ByUser: %w", err)
	}
	out := make([]entity.UserOAuthIdentity, 0, len(rows))
	for _, row := range rows {
		out = append(out, *toOAuthIdentityEntity(row))
	}
	return out, nil
}

// Delete - see entity.UserOAuthIdentityRepository.Delete.
func (r *UserOAuthIdentityRepository) Delete(ctx context.Context, userID int, provider string) error {
	if _, err := GetClient(ctx, r.client).UserOAuthIdentity.Delete().
		Where(useroauthidentity.UserIDEQ(userID), useroauthidentity.ProviderEQ(provider)).
		Exec(ctx); err != nil {
		return fmt.Errorf("UserOAuthIdentityRepository.Delete: %w", err)
	}
	return nil
}
