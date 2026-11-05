package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

var yandexEndpoint = oauth2.Endpoint{
	AuthURL:  "https://oauth.yandex.ru/authorize",
	TokenURL: "https://oauth.yandex.ru/token",
}

type yandexProvider struct {
	name string
	cfg  *oauth2.Config
}

// NewYandex - the Yandex provider with the app's credentials; redirectURL is
// the callback registered in the provider's console.
func NewYandex(creds Credentials, redirectURL string) Provider {
	return timed{&yandexProvider{
		name: Yandex,
		cfg: &oauth2.Config{
			ClientID:     creds.ClientID,
			ClientSecret: creds.ClientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"login:email", "login:info"},
			Endpoint:     yandexEndpoint,
		},
	}}
}

func (p *yandexProvider) Name() string { return p.name }

func (p *yandexProvider) AuthCodeURL(state string) string {
	return p.cfg.AuthCodeURL(state)
}

func (p *yandexProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.cfg.Exchange(ctx, code)
}

type yandexUserInfo struct {
	ID           string `json:"id"`
	DefaultEmail string `json:"default_email"`
	Login        string `json:"login"`
	DisplayName  string `json:"display_name"`
	RealName     string `json:"real_name"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
}

func (p *yandexProvider) UserInfo(ctx context.Context, token *oauth2.Token) (*Profile, error) {
	client := p.cfg.Client(ctx, token)
	resp, err := client.Get("https://login.yandex.ru/info?format=json")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("yandex userinfo: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var info yandexUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	email := strings.TrimSpace(strings.ToLower(info.DefaultEmail))
	display := strings.TrimSpace(info.DisplayName)
	if display == "" {
		display = strings.TrimSpace(strings.TrimSpace(info.FirstName + " " + info.LastName))
	}
	if display == "" {
		display = strings.TrimSpace(info.RealName)
	}
	if display == "" {
		display = strings.TrimSpace(info.Login)
	}
	return &Profile{
		ProviderUserID: info.ID,
		Email:          email,
		// Yandex ID gives only the addresses the user confirmed.
		EmailVerified: email != "",
		DisplayName:   display,
	}, nil
}
