package oauth

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/oauth"
)

// TestRegistryProviders - every supported name builds the internal/lib/oauth
// provider of that name; only providers with both keys are enabled.
func TestRegistryProviders(t *testing.T) {
	for _, name := range entity.OAuthProviders {
		if p := builders[name](oauth.Credentials{ClientID: "id", ClientSecret: "secret"}, "https://example.test/cb"); p.Name() != name {
			t.Errorf("builder %s makes provider %s", name, p.Name())
		}
	}
	r := NewRegistry("https://example.test/", config.OAuthConfig{})
	r.Set(map[string]entity.OAuthCredentials{entity.OAuthProviders[0]: {ClientID: "id", ClientSecret: "secret"}, entity.OAuthProviders[1]: {ClientID: "id"}})
	if got := r.EnabledNames(); len(got) != 1 || got[0] != entity.OAuthProviders[0] {
		t.Errorf("enabled: %v", got)
	}
	p, _ := r.Get(entity.OAuthProviders[0])
	if u := p.AuthCodeURL("st"); !strings.Contains(u, "redirect_uri=https%3A%2F%2Fexample.test%2Fauth%2F"+entity.OAuthProviders[0]+"%2Fcallback") {
		t.Errorf("auth URL: %s", u)
	}
}

// TestRegistrySetBaseURL - a new site URL rebuilds the enabled providers
// with callbacks on it.
func TestRegistrySetBaseURL(t *testing.T) {
	r := NewRegistry("https://old.test", config.OAuthConfig{})
	r.Set(map[string]entity.OAuthCredentials{entity.OAuthProviders[0]: {ClientID: "id", ClientSecret: "secret"}})
	r.SetBaseURL("https://new.test/")
	if got := r.RedirectURL(entity.OAuthProviders[0]); got != "https://new.test/auth/"+entity.OAuthProviders[0]+"/callback" {
		t.Errorf("redirect URL: %s", got)
	}
	p, ok := r.Get(entity.OAuthProviders[0])
	if !ok {
		t.Fatal("provider dropped")
	}
	if u := p.AuthCodeURL("st"); !strings.Contains(u, "redirect_uri=https%3A%2F%2Fnew.test%2Fauth%2F") {
		t.Errorf("auth URL: %s", u)
	}
}
