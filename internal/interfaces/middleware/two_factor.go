package middleware

import (
	"context"
	"net/http"
	"strings"
)

// twoFactorOpenPaths - what a user who must turn two-factor sign-in on
// may still open: its page, signing in and out, the static files.
var twoFactorOpenPaths = []string{"/me/two-factor", "/login", "/logout", "/static/", "/i18n/", "/favicon"}

// TwoFactorSetupMiddleware sends a signed-in user who must turn
// two-factor sign-in on (needed: their role needs it, it's off) to its
// page (setupPath): a page they open goes there, any other request is
// refused. Signed-out visitors and everyone else pass.
func TwoFactorSetupMiddleware(needed func(ctx context.Context, userID int) bool, setupPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := GetUserIDFromContext(r.Context())
			if !ok || id == 0 || twoFactorOpen(r.URL.Path) || !needed(r.Context(), id) {
				next.ServeHTTP(w, r)
				return
			}
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				http.Redirect(w, r, setupPath, http.StatusFound)
				return
			}
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		})
	}
}

func twoFactorOpen(path string) bool {
	for _, p := range twoFactorOpenPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
