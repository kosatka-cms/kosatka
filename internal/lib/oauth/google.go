package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type googleProvider struct {
	name string
	cfg  *oauth2.Config
}

// NewGoogle - the Google provider with the app's credentials; redirectURL is
// the callback registered in the provider's console.
func NewGoogle(creds Credentials, redirectURL string) Provider {
	return timed{&googleProvider{
		name: Google,
		cfg: &oauth2.Config{
			ClientID:     creds.ClientID,
			ClientSecret: creds.ClientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}}
}

func (p *googleProvider) Name() string { return p.name }

func (p *googleProvider) AuthCodeURL(state string) string {
	return p.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *googleProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.cfg.Exchange(ctx, code)
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (p *googleProvider) UserInfo(ctx context.Context, token *oauth2.Token) (*Profile, error) {
	client := p.cfg.Client(ctx, token)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("google userinfo: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &Profile{
		ProviderUserID: info.Sub,
		Email:          strings.TrimSpace(strings.ToLower(info.Email)),
		EmailVerified:  info.EmailVerified,
		DisplayName:    strings.TrimSpace(info.Name),
		AvatarURL:      strings.TrimSpace(info.Picture),
	}, nil
}
