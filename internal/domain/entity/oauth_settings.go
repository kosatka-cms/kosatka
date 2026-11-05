package entity

import "context"

// OAuthProviderSettings - a social login provider's settings saved in the
// admin. Empty keys mean the server config's keys apply.
type OAuthProviderSettings struct {
	Provider string
	// Enabled - off hides the provider even with the config's keys.
	Enabled      bool
	ClientID     string
	ClientSecret string
}

// OAuthSettingsRepository stores the providers' settings, a row each.
type OAuthSettingsRepository interface {
	// List - the saved rows by provider name.
	List(ctx context.Context) (map[string]OAuthProviderSettings, error)
	Upsert(ctx context.Context, s OAuthProviderSettings) error
	// LinkedAccounts - the number of accounts linked to each provider.
	LinkedAccounts(ctx context.Context) (map[string]int, error)
}
