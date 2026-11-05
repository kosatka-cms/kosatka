package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestTwoFactorSetupMiddleware - a signed-in user who must turn two-factor
// sign-in on: a page goes to its page, a post is refused, its own page,
// signing out and the static files pass; others pass.
func TestTwoFactorSetupMiddleware(t *testing.T) {
	needed := func(_ context.Context, id int) bool { return id == 5 }
	h := TwoFactorSetupMiddleware(needed, "/me/two-factor")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	for _, tc := range []struct {
		user         int
		method, path string
		want         int
		location     string
	}{
		{5, http.MethodGet, "/admin", http.StatusFound, "/me/two-factor"},
		{5, http.MethodPost, "/admin/content/bulk", http.StatusForbidden, ""},
		{5, http.MethodGet, "/me/two-factor", http.StatusTeapot, ""},
		{5, http.MethodPost, "/me/two-factor/enable", http.StatusTeapot, ""},
		{5, http.MethodPost, "/logout", http.StatusTeapot, ""},
		{5, http.MethodGet, "/static/css/profile.min.css", http.StatusTeapot, ""},
		{6, http.MethodGet, "/admin", http.StatusTeapot, ""},
		{0, http.MethodGet, "/admin", http.StatusTeapot, ""},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.user != 0 {
			r = r.WithContext(context.WithValue(r.Context(), userIDKey, tc.user))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want || w.Header().Get("Location") != tc.location {
			t.Errorf("user %d %s %s: %d %q", tc.user, tc.method, tc.path, w.Code, w.Header().Get("Location"))
		}
	}
}
