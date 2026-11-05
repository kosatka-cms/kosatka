package middleware

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// ActorMiddleware puts who acts in the request's context (audit.Actor):
// the signed-in user (OptionalAuthMiddleware, around it, found them) and
// the client's address - the actions the request records (audit.Record)
// carry them.
func ActorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := audit.Actor{IP: httputil.ClientIP(r)}
		if id, ok := GetUserIDFromContext(r.Context()); ok {
			a.UserID = id
		}
		next.ServeHTTP(w, r.WithContext(audit.WithActor(r.Context(), a)))
	})
}
