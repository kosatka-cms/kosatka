package interfaces

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestOAuthButtons - a button for each provider offered now, in its
// order, with the provider's name for the label.
func TestOAuthButtons(t *testing.T) {
	t.Cleanup(func() { SetOAuthProviders(nil) })
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	if got := oauthButtons(req); len(got) != 0 {
		t.Fatalf("no providers: %v", got)
	}
	SetOAuthProviders(func() []string { return []string{entity.OAuthProviderGoogle, entity.OAuthProviderYandex} })
	got := oauthButtons(req)
	if len(got) != 2 || got[0] != (oauthButton{Name: "google", Label: "Google"}) || got[1] != (oauthButton{Name: "yandex", Label: "Yandex"}) {
		t.Errorf("buttons: %+v", got)
	}
}
