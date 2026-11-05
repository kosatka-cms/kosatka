package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
)

// VK ID OAuth 2.0 (id.vk.ru).
var vkEndpoint = oauth2.Endpoint{
	AuthURL:  "https://id.vk.ru/authorize",
	TokenURL: "https://id.vk.ru/oauth2/auth",
}

type vkProvider struct {
	name string
	cfg  *oauth2.Config
}

// NewVK - the VK provider with the app's credentials; redirectURL is
// the callback registered in the provider's console.
func NewVK(creds Credentials, redirectURL string) Provider {
	return timed{&vkProvider{
		name: VK,
		cfg: &oauth2.Config{
			ClientID:     creds.ClientID,
			ClientSecret: creds.ClientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"email"},
			Endpoint:     vkEndpoint,
		},
	}}
}

func (p *vkProvider) Name() string { return p.name }

func (p *vkProvider) AuthCodeURL(state string) string {
	return p.cfg.AuthCodeURL(state)
}

func (p *vkProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.cfg.Exchange(ctx, code)
}

type vkUserResponse struct {
	User struct {
		UserID    int64  `json:"user_id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Avatar    string `json:"avatar"`
		Email     string `json:"email"`
	} `json:"user"`
}

func (p *vkProvider) UserInfo(ctx context.Context, token *oauth2.Token) (*Profile, error) {
	clientID := p.cfg.ClientID
	u := "https://id.vk.ru/oauth2/user_info?client_id=" + url.QueryEscape(clientID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("vk userinfo: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var data vkUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	display := strings.TrimSpace(strings.TrimSpace(data.User.FirstName + " " + data.User.LastName))
	// EmailVerified stays false: VK ID doesn't say the address is
	// confirmed.
	return &Profile{
		ProviderUserID: strconv.FormatInt(data.User.UserID, 10),
		Email:          strings.TrimSpace(strings.ToLower(data.User.Email)),
		DisplayName:    display,
		AvatarURL:      strings.TrimSpace(data.User.Avatar),
	}, nil
}
