package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestBotTrap - a form shown with the trap's fields passes once enough
// time went by; it is caught sent too soon, with the hidden field filled,
// without the time or with one the site didn't sign; with the trap off,
// nothing is added or checked.
func TestBotTrap(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	settings := commentSettingsStub{entity.CommentSettings{BotTrap: true, BotMinSeconds: 3}}
	trap := NewBotTrap("secret", settings)
	trap.now = func() time.Time { return now }
	fields := string(trap.Fields(ctx))
	shown := regexp.MustCompile(`name="form_shown" value="([^"]+)"`).FindStringSubmatch(fields)
	if shown == nil || !strings.Contains(fields, `name="address_line_2"`) || !strings.Contains(fields, `aria-hidden="true"`) {
		t.Fatalf("the fields: %s", fields)
	}
	post := func(v url.Values) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return r
	}
	sent := func(after time.Duration, v url.Values) error {
		trap.now = func() time.Time { return now.Add(after) }
		return trap.Check(ctx, post(v))
	}
	if err := sent(5*time.Second, url.Values{"form_shown": {shown[1]}}); err != nil {
		t.Errorf("a person's form: %v", err)
	}
	for name, c := range map[string]struct {
		after time.Duration
		v     url.Values
	}{
		"too soon":          {time.Second, url.Values{"form_shown": {shown[1]}}},
		"the field filled":  {5 * time.Second, url.Values{"form_shown": {shown[1]}, "address_line_2": {"12 Main St"}}},
		"no time":           {5 * time.Second, url.Values{}},
		"a time not signed": {5 * time.Second, url.Values{"form_shown": {"1793000000.0123456789abcdef0123456789abcdef"}}},
		"another site's":    {5 * time.Second, url.Values{"form_shown": {strings.Replace(shown[1], ".", ".0", 1)}}},
	} {
		if err := sent(c.after, c.v); !errors.Is(err, ErrBotTrap) {
			t.Errorf("%s: %v", name, err)
		}
	}

	off := NewBotTrap("secret", commentSettingsStub{entity.CommentSettings{}})
	if off.Fields(ctx) != "" || off.Check(ctx, post(url.Values{"address_line_2": {"x"}})) != nil {
		t.Error("the trap off still adds or checks")
	}
	var none *BotTrap
	if none.Fields(ctx) != "" || none.Check(ctx, post(nil)) != nil {
		t.Error("no trap set still adds or checks")
	}
}
