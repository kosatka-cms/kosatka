package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// TestUserTimezones - a logged-in user's own zone goes into the request
// context while user time zones are on (cached until Forget); guests and
// the setting off - the site's zone.
func TestUserTimezones(t *testing.T) {
	t.Cleanup(func() { sitetime.SetUserZones(false); _ = sitetime.Set("UTC") })
	_ = sitetime.Set("Europe/Moscow")
	zone, lookups := "Asia/Vladivostok", 0
	u := NewUserTimezones(func(context.Context, int) (string, error) { lookups++; return zone, nil })
	var got string
	h := u.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = sitetime.FromContext(r.Context()).String()
	}))
	serve := func(userID int) string {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if userID != 0 {
			r = r.WithContext(context.WithValue(r.Context(), userIDKey, userID))
		}
		h.ServeHTTP(httptest.NewRecorder(), r)
		return got
	}
	if serve(5) != "Europe/Moscow" {
		t.Fatal("setting off - the site's zone")
	}
	sitetime.SetUserZones(true)
	if serve(5) != "Asia/Vladivostok" || serve(0) != "Europe/Moscow" {
		t.Fatal("user / guest zone")
	}
	zone = "Asia/Omsk"
	if serve(5) != "Asia/Vladivostok" || lookups != 1 {
		t.Fatalf("cached until Forget, lookups = %d", lookups)
	}
	u.Forget(5)
	if serve(5) != "Asia/Omsk" {
		t.Fatal("after Forget")
	}
}
