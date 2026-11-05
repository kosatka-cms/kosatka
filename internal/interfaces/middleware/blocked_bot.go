package middleware

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// BlockedBotMiddleware returns 403 Forbidden for requests whose User-Agent matches a blocked pattern
func BlockedBotMiddleware(svc service.BlockedBotService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if svc.IsBlocked(r.UserAgent()) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
