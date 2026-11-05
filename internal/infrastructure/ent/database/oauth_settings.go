package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/useroauthidentity"
)

// OAuthSettingsRepository implements entity.OAuthSettingsRepository.
type OAuthSettingsRepository struct {
	client *generated.Client
}

var _ entity.OAuthSettingsRepository = (*OAuthSettingsRepository)(nil)

// NewOAuthSettingsRepository - the OAuth settings repository on db.
func NewOAuthSettingsRepository(db *sql.DB) *OAuthSettingsRepository {
	return &OAuthSettingsRepository{client: clientFor(db)}
}

// List - see entity.OAuthSettingsRepository.List.
func (r *OAuthSettingsRepository) List(ctx context.Context) (map[string]entity.OAuthProviderSettings, error) {
	rows, err := GetClient(ctx, r.client).OAuthSettings.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("OAuthSettingsRepository.List: %w", err)
	}
	out := make(map[string]entity.OAuthProviderSettings, len(rows))
	for _, s := range rows {
		out[s.ID] = entity.OAuthProviderSettings{Provider: s.ID, Enabled: s.Enabled, ClientID: s.ClientID, ClientSecret: openSecret(s.ClientSecret)}
	}
	return out, nil
}

// Upsert saves a provider's row (the client secret sealed), creating it
// if it isn't there.
func (r *OAuthSettingsRepository) Upsert(ctx context.Context, s entity.OAuthProviderSettings) error {
	client := GetClient(ctx, r.client)
	secret, err := sealSecret(s.ClientSecret)
	if err != nil {
		return err
	}
	err = client.OAuthSettings.UpdateOneID(s.Provider).
		SetEnabled(s.Enabled).SetClientID(s.ClientID).SetClientSecret(secret).
		Exec(ctx)
	if !generated.IsNotFound(err) {
		return err
	}
	return client.OAuthSettings.Create().SetID(s.Provider).
		SetEnabled(s.Enabled).SetClientID(s.ClientID).SetClientSecret(secret).
		Exec(ctx)
}

// LinkedAccounts - see entity.OAuthSettingsRepository.LinkedAccounts.
func (r *OAuthSettingsRepository) LinkedAccounts(ctx context.Context) (map[string]int, error) {
	var rows []struct {
		Provider string `json:"provider"`
		Count    int    `json:"count"`
	}
	err := GetClient(ctx, r.client).UserOAuthIdentity.Query().
		GroupBy(useroauthidentity.FieldProvider).
		Aggregate(generated.Count()).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("OAuthSettingsRepository.LinkedAccounts: %w", err)
	}
	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.Provider] = row.Count
	}
	return out, nil
}
