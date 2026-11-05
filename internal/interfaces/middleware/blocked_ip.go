package middleware

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// BlockedIPMiddleware returns 403 Forbidden for requests from blocked IPs
func BlockedIPMiddleware(svc service.BlockedIPService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := httputil.ClientIP(r)
			if svc.IsBlocked(clientIP) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
