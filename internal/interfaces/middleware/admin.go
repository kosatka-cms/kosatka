package middleware

import (
	"context"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// PermissionMiddleware ensures the user is signed in and has the given
// permission, e.g. service.PermAccessAdmin for /admin.
func PermissionMiddleware(userRepo entity.UserRepository, authService service.AuthService, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(userIDKey).(int)
			if !ok {
				unauthorized(w, r)
				return
			}

			user, err := userRepo.ByID(r.Context(), userID)
			if err != nil || user == nil {
				unauthorized(w, r)
				return
			}

			if !authService.HasPermission(r.Context(), user, permission) {
				forbidden(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUserIDFromContext returns the user ID from the context (set by AuthMiddleware).
func GetUserIDFromContext(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(userIDKey).(int)
	return userID, ok
}
