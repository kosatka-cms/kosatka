package oauth

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// TestProviders - each provider has its name and builds the authorization
// URL of its service with the app's client ID, callback, scopes and state.
func TestProviders(t *testing.T) {
	creds := Credentials{ClientID: "client-1", ClientSecret: "secret"}
	for _, tc := range []struct {
		p      Provider
		name   string
		host   string
		scopes string
	}{
		{NewGoogle(creds, "https://example.test/cb"), Google, "accounts.google.com", "openid email profile"},
		{NewVK(creds, "https://example.test/cb"), VK, "id.vk.ru", "email"},
		{NewYandex(creds, "https://example.test/cb"), Yandex, "oauth.yandex.ru", "login:email login:info"},
	} {
		if tc.p.Name() != tc.name {
			t.Errorf("name %q, want %q", tc.p.Name(), tc.name)
		}
		u, err := url.Parse(tc.p.AuthCodeURL("state-1"))
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		if u.Host != tc.host || q.Get("client_id") != "client-1" || q.Get("redirect_uri") != "https://example.test/cb" ||
			q.Get("scope") != tc.scopes || q.Get("state") != "state-1" || strings.Contains(u.String(), "secret") {
			t.Errorf("%s: %s", tc.name, u)
		}
	}
}

// deadlineProvider reports the deadline its calls get.
type deadlineProvider struct {
	Provider
	got []time.Duration
}

func (d *deadlineProvider) Exchange(ctx context.Context, _ string) (*oauth2.Token, error) {
	dl, _ := ctx.Deadline()
	d.got = append(d.got, time.Until(dl))
	return nil, nil
}

func (d *deadlineProvider) UserInfo(ctx context.Context, _ *oauth2.Token) (*Profile, error) {
	dl, _ := ctx.Deadline()
	d.got = append(d.got, time.Until(dl))
	return nil, nil
}

// TestTimed - each call to a provider gets requestTimeout.
func TestTimed(t *testing.T) {
	d := &deadlineProvider{}
	p := timed{d}
	_, _ = p.Exchange(context.Background(), "code")
	_, _ = p.UserInfo(context.Background(), nil)
	for _, left := range d.got {
		if left <= 0 || left > requestTimeout {
			t.Fatalf("deadlines %v", d.got)
		}
	}
	if len(d.got) != 2 {
		t.Fatalf("calls %v", d.got)
	}
}
