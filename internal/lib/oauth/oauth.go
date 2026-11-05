// Package oauth - OAuth 2.0 sign-in with Google, VK ID and Yandex: the
// authorization URL, the code exchange and the user's profile normalized
// to Profile. Knows nothing of the application: credentials and the
// callback URL come from the caller.
package oauth

import (
	"context"
	"time"

	"golang.org/x/oauth2"
)

// Provider names (Provider.Name).
const (
	Google = "google"
	VK     = "vk"
	Yandex = "yandex"
)

// Credentials - an app registered with a provider.
type Credentials struct {
	ClientID     string
	ClientSecret string
}

// Profile is normalized user data from an OAuth provider.
type Profile struct {
	ProviderUserID string
	Email          string
	// EmailVerified - the provider vouches the account owns Email (it
	// confirmed the address); only then may Email sign in to an existing
	// account with that address.
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
}

// Provider implements OAuth2 authorization for one social network.
type Provider interface {
	Name() string
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	UserInfo(ctx context.Context, token *oauth2.Token) (*Profile, error)
}

// requestTimeout - a call to a provider (the code's exchange, the
// profile): one that doesn't answer doesn't hold the sign-in.
const requestTimeout = 30 * time.Second

// timed - a provider whose calls end by requestTimeout.
type timed struct{ Provider }

func (t timed) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	return t.Provider.Exchange(ctx, code)
}

func (t timed) UserInfo(ctx context.Context, token *oauth2.Token) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	return t.Provider.UserInfo(ctx, token)
}
