package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// InactiveUserSessionMiddleware drops user_id from the session when the account is no
// longer allowed to be logged in (blocked, pending approval, deleted), or the session's
// version isn't the account's any more (its sessions were ended: a new password, two-factor
// sign-in turned on or off - UserRepository.BumpSessionVersion).
//
// Sessions are signed cookies, so blocking a user in the admin does not revoke an existing
// login by itself: the status is only checked at login time. Without this middleware a user
// who was logged in before being blocked keeps full access until the cookie expires.
//
// Must be the outermost session-aware middleware: gorilla/sessions caches the session in the
// request context, so every downstream store.Get / GetUserID sees the cleared session and the
// request is handled as anonymous. The updated cookie is also sent back, logging the user out.
func InactiveUserSessionMiddleware(store *sessions.CookieStore, userRepo entity.UserRepository, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isStaticOrSitesPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			session, err := store.Get(r, entity.SessionName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			uid, ok := session.Values["user_id"].(int)
			if !ok || uid == 0 {
				next.ServeHTTP(w, r)
				return
			}

			status, version, err := userRepo.SessionStateByID(r.Context(), uid)
			switch {
			case errors.Is(err, entity.ErrEntityNotFound):
				status = entity.UserStatusBlocked
			case err != nil:
				// Fail open on DB errors: do not log everyone out because of a transient failure.
				log.Warn("inactive user session check", "user_id", uid, "error", err)
				next.ServeHTTP(w, r)
				return
			}

			// A session from before the versions has none: version 0, the
			// accounts' first.
			sessionVersion, _ := session.Values[service.SessionVersionKey].(int)
			if status != entity.UserStatusActive || sessionVersion != version {
				delete(session.Values, "user_id")
				if err := session.Save(r, w); err != nil {
					log.Warn("drop inactive user session", "user_id", uid, "error", err)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
