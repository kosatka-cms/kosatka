package sociallogin

import (
	"context"
	"sync"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	oauthinfra "github.com/kosatka-cms/kosatka/internal/infrastructure/oauth"
)

type stubOAuthSettingsRepo struct {
	rows map[string]entity.OAuthProviderSettings
}

func (r *stubOAuthSettingsRepo) List(context.Context) (map[string]entity.OAuthProviderSettings, error) {
	out := map[string]entity.OAuthProviderSettings{}
	for k, v := range r.rows {
		out[k] = v
	}
	return out, nil
}

func (r *stubOAuthSettingsRepo) Upsert(_ context.Context, s entity.OAuthProviderSettings) error {
	r.rows[s.Provider] = s
	return nil
}

func (r *stubOAuthSettingsRepo) LinkedAccounts(context.Context) (map[string]int, error) {
	return map[string]int{}, nil
}

func statusOf(svc *SettingsService, name string) ProviderStatus {
	for _, st := range svc.Status() {
		if st.Name == name {
			return st
		}
	}
	return ProviderStatus{}
}

// TestSettingsService - the config's keys win over the ones saved in the
// admin, which apply to the providers the config has no keys for; a
// provider switched off leaves the login page even with the config's
// keys.
func TestSettingsService(t *testing.T) {
	ctx := context.Background()
	cfg := config.OAuthConfig{
		Google: config.GoogleOAuthConfig{ClientID: "g-config", ClientSecret: "gs"},
		VK:     config.VKOAuthConfig{ClientID: "vk-config", ClientSecret: "vs"},
	}
	repo := &stubOAuthSettingsRepo{rows: map[string]entity.OAuthProviderSettings{}}
	registry := oauthinfra.NewRegistry("https://example.com", config.OAuthConfig{})
	svc := NewSettingsService(repo, registry, oauthinfra.ConfigCredentials(cfg))
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if got := registry.EnabledNames(); len(got) != 2 || got[0] != entity.OAuthProviderGoogle || got[1] != entity.OAuthProviderVK {
		t.Fatalf("config keys: %v", got)
	}
	if st := statusOf(svc, entity.OAuthProviderYandex); st.Source != SourceNone || st.Active {
		t.Errorf("yandex without keys: %+v", st)
	}

	repo.rows[entity.OAuthProviderGoogle] = entity.OAuthProviderSettings{Provider: entity.OAuthProviderGoogle, Enabled: true, ClientID: "g-admin", ClientSecret: "ga"}
	repo.rows[entity.OAuthProviderVK] = entity.OAuthProviderSettings{Provider: entity.OAuthProviderVK, Enabled: false}
	repo.rows[entity.OAuthProviderYandex] = entity.OAuthProviderSettings{Provider: entity.OAuthProviderYandex, Enabled: true, ClientID: "y-admin", ClientSecret: "ya"}
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if st := statusOf(svc, entity.OAuthProviderGoogle); st.Source != SourceConfig || !st.Active {
		t.Errorf("google, keys in both: %+v, want the config's", st)
	}
	if st := statusOf(svc, entity.OAuthProviderYandex); st.Source != SourceAdmin || !st.Active {
		t.Errorf("yandex from the admin: %+v", st)
	}
	if st := statusOf(svc, entity.OAuthProviderVK); st.Source != SourceConfig || st.Active {
		t.Errorf("vk switched off: %+v", st)
	}
	if got := registry.EnabledNames(); len(got) != 2 || got[0] != entity.OAuthProviderGoogle || got[1] != entity.OAuthProviderYandex {
		t.Fatalf("after reload: %v", got)
	}
	if _, ok := registry.Get(entity.OAuthProviderVK); ok {
		t.Error("vk is still offered")
	}
}

// TestRegistrySetConcurrent - Set while logins read the registry (run
// with -race).
func TestRegistrySetConcurrent(t *testing.T) {
	registry := oauthinfra.NewRegistry("https://example.com", config.OAuthConfig{})
	on := map[string]entity.OAuthCredentials{entity.OAuthProviderGoogle: {ClientID: "a", ClientSecret: "b"}}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				registry.Set(on)
				registry.Set(nil)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				registry.Get(entity.OAuthProviderGoogle)
				registry.EnabledNames()
			}
		}()
	}
	wg.Wait()
}
