// Package oauth - the site's OAuth providers (internal/lib/oauth) built from the
// config and the admin settings, with the engine's callback route.
package oauth

import (
	"fmt"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/oauth"
)

type (
	// Profile - see internal/lib/oauth.
	Profile = oauth.Profile
	// Provider - see internal/lib/oauth.
	Provider = oauth.Provider
)

// builders create a provider from its credentials and callback URL.
var builders = map[string]func(creds oauth.Credentials, redirectURL string) Provider{
	entity.OAuthProviderGoogle: oauth.NewGoogle,
	entity.OAuthProviderVK:     oauth.NewVK,
	entity.OAuthProviderYandex: oauth.NewYandex,
}

// Registry holds configured OAuth providers by name. Set replaces them
// (settings saved in the admin apply without a restart), SetBaseURL
// rebuilds them for a new site URL.
type Registry struct {
	mu        sync.RWMutex
	baseURL   string
	creds     map[string]entity.OAuthCredentials
	providers map[string]Provider
}

// NewRegistry - the providers with keys in cfg.
func NewRegistry(baseURL string, cfg config.OAuthConfig) *Registry {
	r := &Registry{baseURL: strings.TrimSuffix(strings.TrimSpace(baseURL), "/")}
	r.Set(ConfigCredentials(cfg))
	return r
}

// ConfigCredentials - the config's credentials by provider name.
func ConfigCredentials(cfg config.OAuthConfig) map[string]entity.OAuthCredentials {
	return map[string]entity.OAuthCredentials{
		entity.OAuthProviderGoogle: entity.OAuthCredentials(cfg.Google.Config()),
		entity.OAuthProviderVK:     entity.OAuthCredentials(cfg.VK.Config()),
		entity.OAuthProviderYandex: entity.OAuthCredentials(cfg.Yandex.Config()),
	}
}

// Set replaces the providers: one for each supported name with both keys.
func (r *Registry) Set(creds map[string]entity.OAuthCredentials) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.creds = creds
	r.build()
}

// SetBaseURL rebuilds the providers with callbacks on a new site URL.
func (r *Registry) SetBaseURL(baseURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	r.build()
}

// build - the providers from creds and baseURL; r.mu is held.
func (r *Registry) build() {
	providers := make(map[string]Provider)
	for _, name := range entity.OAuthProviders {
		if c, ok := r.creds[name]; ok && c.Enabled() {
			providers[name] = builders[name](oauth.Credentials{ClientID: c.ClientID, ClientSecret: c.ClientSecret}, redirectURL(r.baseURL, name))
		}
	}
	r.providers = providers
}

// RedirectURL - the provider's callback URL to register in its console.
func (r *Registry) RedirectURL(provider string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return redirectURL(r.baseURL, provider)
}

// Get - the provider of name, if it has both keys.
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// EnabledNames - the names of the providers with both keys, in
// entity.OAuthProviders' order.
func (r *Registry) EnabledNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.providers))
	for _, n := range entity.OAuthProviders {
		if _, ok := r.providers[n]; ok {
			out = append(out, n)
		}
	}
	return out
}

func redirectURL(baseURL, provider string) string {
	return fmt.Sprintf("%s/auth/%s/callback", strings.TrimSuffix(baseURL, "/"), provider)
}
