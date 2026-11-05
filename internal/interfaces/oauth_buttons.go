package interfaces

import (
	"context"
	"net/http"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// OAuthProviderLabels - the providers as the settings page and the login
// buttons name them.
var OAuthProviderLabels = map[string]string{
	entity.OAuthProviderGoogle: "Google",
	entity.OAuthProviderVK:     i18n.N("VK"),
	entity.OAuthProviderYandex: i18n.N("Yandex"),
}

// oauthButton - a provider's login button (user/social_login.html); the
// icon is /static/images/oauth/<Name>.svg.
type oauthButton struct {
	Name, Label string
}

// oauthButtons - the buttons of the providers offered now.
func oauthButtons(r *http.Request) []oauthButton {
	if oauthProviders == nil {
		return nil
	}
	names := oauthProviders()
	out := make([]oauthButton, 0, len(names))
	for _, name := range names {
		out = append(out, oauthButton{Name: name, Label: i18n.T(r.Context(), OAuthProviderLabels[name])})
	}
	return out
}

// oauthServiceRow - a provider on the profile form: connected to the
// account or not.
type oauthServiceRow struct {
	oauthButton
	Connected bool
}

// oauthServiceRows - the providers offered, each marked connected when it
// is among connected.
func oauthServiceRows(buttons []oauthButton, connected []string) []oauthServiceRow {
	out := make([]oauthServiceRow, 0, len(buttons))
	for _, b := range buttons {
		out = append(out, oauthServiceRow{oauthButton: b, Connected: slices.Contains(connected, b.Name)})
	}
	return out
}

// oauthMessage - the profile form's notice after connecting or
// disconnecting a social network (?oauth=).
type oauthMessage struct {
	Text  string
	Error bool
}

// oauthProfileMessage - the notice for the profile form's ?oauth= code;
// nil - none.
func oauthProfileMessage(ctx context.Context, code string) *oauthMessage {
	switch code {
	case "connected":
		return &oauthMessage{Text: i18n.T(ctx, "The social network is connected: you can sign in with it.")}
	case "disconnected":
		return &oauthMessage{Text: i18n.T(ctx, "The social network is disconnected.")}
	case "elsewhere":
		return &oauthMessage{Text: i18n.T(ctx, "That social network account is connected to another account of the site."), Error: true}
	case "other":
		return &oauthMessage{Text: i18n.T(ctx, "Another account of that social network is connected already: disconnect it first."), Error: true}
	case "last":
		return &oauthMessage{Text: i18n.T(ctx, "It is the account's only way to sign in: give the account an e-mail address first."), Error: true}
	case "error":
		return &oauthMessage{Text: i18n.T(ctx, "Error while saving."), Error: true}
	}
	return nil
}
