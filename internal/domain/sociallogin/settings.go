package sociallogin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// Where a social login provider's keys come from (ProviderStatus.Source).
const (
	SourceAdmin  = "admin"
	SourceConfig = "config"
	SourceNone   = "none"
)

// ProviderStatus - a provider's settings in effect, for the page. It
// carries no keys: the server config's keys never reach the admin page.
type ProviderStatus struct {
	Name    string
	Enabled bool
	Source  string
	// Active - the provider is offered on the login page.
	Active bool
}

// oauthRegistrySetter - the part of oauth.Registry the service drives.
type oauthRegistrySetter interface {
	Set(creds map[string]entity.OAuthCredentials)
}

// SettingsService applies the social login keys: the server config's for
// the providers it has keys for (they win: the admin can't replace them),
// the ones saved in the admin for the others; a provider switched off is
// not offered. Reload applies new settings without a restart.
type SettingsService struct {
	repo     entity.OAuthSettingsRepository
	registry oauthRegistrySetter
	config   map[string]entity.OAuthCredentials

	mu     sync.RWMutex
	status []ProviderStatus
}

// NewSettingsService - configured holds the server config's keys by
// provider name (used for a provider instead of the admin's).
func NewSettingsService(repo entity.OAuthSettingsRepository, registry oauthRegistrySetter, configured map[string]entity.OAuthCredentials) *SettingsService {
	s := &SettingsService{repo: repo, registry: registry, config: configured}
	s.apply(nil)
	return s
}

// Reload reads the saved settings and makes them current.
func (s *SettingsService) Reload(ctx context.Context) error {
	saved, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	s.apply(saved)
	return nil
}

// Saved - the settings saved in the admin, by provider name.
func (s *SettingsService) Saved(ctx context.Context) (map[string]entity.OAuthProviderSettings, error) {
	saved, err := s.repo.List(ctx)
	return entity.Wrapped(saved, err, "loading the social login settings")
}

// Save stores the providers' settings and makes them current.
func (s *SettingsService) Save(ctx context.Context, rows []entity.OAuthProviderSettings) error {
	for _, row := range rows {
		if err := s.repo.Upsert(ctx, row); err != nil {
			return fmt.Errorf("saving the social login settings (%s): %w", row.Provider, err)
		}
	}
	if err := s.Reload(ctx); err != nil {
		return fmt.Errorf("applying the social login settings: %w", err)
	}
	return nil
}

// LinkedAccounts - the number of accounts linked to each provider.
func (s *SettingsService) LinkedAccounts(ctx context.Context) (map[string]int, error) {
	linked, err := s.repo.LinkedAccounts(ctx)
	return entity.Wrapped(linked, err, "counting linked accounts")
}

func (s *SettingsService) apply(saved map[string]entity.OAuthProviderSettings) {
	creds := make(map[string]entity.OAuthCredentials, len(entity.OAuthProviders))
	status := make([]ProviderStatus, 0, len(entity.OAuthProviders))
	for _, name := range entity.OAuthProviders {
		row, ok := saved[name]
		if !ok {
			row = entity.OAuthProviderSettings{Provider: name, Enabled: true}
		}
		c, source := s.config[name], SourceConfig
		if !c.Enabled() {
			c, source = entity.OAuthCredentials{ClientID: strings.TrimSpace(row.ClientID), ClientSecret: strings.TrimSpace(row.ClientSecret)}, SourceAdmin
			if !c.Enabled() {
				source = SourceNone
			}
		}
		st := ProviderStatus{Name: name, Enabled: row.Enabled, Source: source}
		if row.Enabled && source != SourceNone {
			creds[name] = c
			st.Active = true
		}
		status = append(status, st)
	}
	if s.registry != nil {
		s.registry.Set(creds)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

// Status - each provider's settings in effect, in the UI order.
func (s *SettingsService) Status() []ProviderStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ProviderStatus(nil), s.status...)
}
