package entity

import (
	"context"
	"strings"
	"time"
)

// The supported social login providers' names (OAuthProviders).
const (
	OAuthProviderGoogle = "google"
	OAuthProviderVK     = "vk"
	OAuthProviderYandex = "yandex"
)

// OAuthProviders - the supported social login providers, in the order the
// UI lists them.
var OAuthProviders = []string{OAuthProviderGoogle, OAuthProviderVK, OAuthProviderYandex}

// OAuthConsoleURLs - where each provider's app (client ID and secret) is
// registered, for the settings page.
var OAuthConsoleURLs = map[string]string{
	OAuthProviderGoogle: "https://console.cloud.google.com/apis/credentials",
	OAuthProviderVK:     "https://id.vk.com/about/business/go",
	OAuthProviderYandex: "https://oauth.yandex.ru/",
}

// OAuthCredentials - a provider app's keys, from the server config or the
// admin settings.
type OAuthCredentials struct {
	ClientID     string
	ClientSecret string
}

// Enabled - both keys are set.
func (c OAuthCredentials) Enabled() bool {
	return strings.TrimSpace(c.ClientID) != "" && strings.TrimSpace(c.ClientSecret) != ""
}

// UserOAuthIdentity - a provider's account (ProviderUserID) connected to a
// user's, with the e-mail the provider gave.
type UserOAuthIdentity struct {
	ID             int64
	UserID         int
	Provider       string
	ProviderUserID string
	Email          string
	CreatedAt      time.Time
}

// UserOAuthIdentityRepository - storage port for the provider accounts
// connected to the users'.
type UserOAuthIdentityRepository interface {
	FindByProviderUser(ctx context.Context, provider, providerUserID string) (*UserOAuthIdentity, error)
	FindByUserAndProvider(ctx context.Context, userID int, provider string) (*UserOAuthIdentity, error)
	// ByUser - the provider accounts connected to the user's.
	ByUser(ctx context.Context, userID int) ([]UserOAuthIdentity, error)
	Create(ctx context.Context, identity UserOAuthIdentity) (*UserOAuthIdentity, error)
	// Delete disconnects the provider's account from the user's (none
	// connected - nothing to do).
	Delete(ctx context.Context, userID int, provider string) error
}
