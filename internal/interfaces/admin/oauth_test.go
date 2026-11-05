package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestOAuthSettingsFromForm - a provider's keys as submitted, the saved
// secret kept when its field is empty, a half pair refused; a provider
// with keys in the server config keeps its saved keys, only its switch
// changes.
func TestOAuthSettingsFromForm(t *testing.T) {
	saved := map[string]entity.OAuthProviderSettings{
		entity.OAuthProviderGoogle: {Provider: entity.OAuthProviderGoogle, Enabled: true, ClientID: "g-id", ClientSecret: "g-secret"},
		entity.OAuthProviderVK:     {Provider: entity.OAuthProviderVK, Enabled: true, ClientID: "vk-id", ClientSecret: "vk-secret"},
	}
	post := func(form url.Values) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return r
	}
	form := url.Values{
		"google_enabled": {"1"}, "google_client_id": {"g-new"},
		"vk_client_id": {"vk-new"}, "vk_client_secret": {"vk-new-secret"},
		"yandex_client_id": {"y-id"},
	}
	if _, bad := oauthSettingsFromForm(post(form), saved, nil); bad != entity.OAuthProviderYandex {
		t.Errorf("a half pair: %q", bad)
	}
	delete(form, "yandex_client_id")
	rows, bad := oauthSettingsFromForm(post(form), saved, map[string]bool{entity.OAuthProviderVK: true})
	if bad != "" {
		t.Fatal(bad)
	}
	got := map[string]entity.OAuthProviderSettings{}
	for _, r := range rows {
		got[r.Provider] = r
	}
	if g := got[entity.OAuthProviderGoogle]; g.ClientID != "g-new" || g.ClientSecret != "g-secret" || !g.Enabled {
		t.Errorf("google: %+v", g)
	}
	if vk := got[entity.OAuthProviderVK]; vk.ClientID != "vk-id" || vk.ClientSecret != "vk-secret" || vk.Enabled {
		t.Errorf("vk with the config's keys: %+v, want its saved keys, switched off", vk)
	}
}
