package middleware

import (
	"context"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// contextKey is the type for request context keys set by this package.
type contextKey string

const userIDKey contextKey = "user_id"

// AuthMiddleware requires a session-authenticated user, redirecting to login otherwise.
func AuthMiddleware(store *sessions.CookieStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := store.Get(r, entity.SessionName)
			if err != nil {
				// An unreadable session (e.g. invalid cookie after a key change)
				// counts as not authenticated.
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			userID, ok := session.Values["user_id"].(int)
			if !ok || userID == 0 {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthJSONMiddleware requires a logged-in user and responds with JSON 401 if the session is missing or invalid.
func AuthJSONMiddleware(store *sessions.CookieStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := store.Get(r, entity.SessionName)
			if err != nil {
				httputil.JSONError(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			userID, ok := session.Values["user_id"].(int)
			if !ok || userID == 0 {
				httputil.JSONError(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthMiddleware adds the user ID to the context if there is a
// valid session, and continues without redirecting otherwise.
func OptionalAuthMiddleware(store *sessions.CookieStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := store.Get(r, entity.SessionName)
			if err == nil {
				if userID, ok := session.Values["user_id"].(int); ok && userID != 0 {
					ctx := context.WithValue(r.Context(), userIDKey, userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
