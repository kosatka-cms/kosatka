package middleware

import (
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// RedirectMiddleware issues a 301 when the request path matches a configured redirect.
func RedirectMiddleware(svc service.RedirectService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if to, ok := svc.GetRedirect(strings.TrimPrefix(r.URL.Path, "/")); ok {
				http.Redirect(w, r, to, http.StatusMovedPermanently)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
