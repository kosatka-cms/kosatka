package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/paths"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

const lastAccessTouchMinInterval = 4 * time.Minute

func isStaticOrSitesPath(path string) bool {
	return strings.HasPrefix(path, "/static/") || path == "/static" ||
		strings.HasPrefix(path, paths.PublicFilesURL())
}

// UserLastAccessMiddleware updates accessed_at for logged-in users, throttled, and skips static file paths.
// Must run inside OptionalAuthMiddleware so context carries user_id.
func UserLastAccessMiddleware(userRepo entity.UserRepository, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if uid, ok := GetUserIDFromContext(r.Context()); ok && uid != 0 {
				if !isStaticOrSitesPath(r.URL.Path) {
					if err := userRepo.TouchAccessedAtIfStale(r.Context(), uid, lastAccessTouchMinInterval); err != nil {
						log.Warn("touch accessed_at", "user_id", uid, "error", err)
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
