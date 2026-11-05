package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// ContentAccessMiddleware puts in the request's context what of the
// content its visitor may not view (entity.WithContentVisitor): every
// node and comment query of the request leaves it out. visitor - that of
// the signed-in user userID (0 - a guest). A failed check is a 500, never
// a request that sees everything. Inside OptionalAuth: it sees the user.
func ContentAccessMiddleware(visitor func(ctx context.Context, userID int) (entity.ContentVisitor, error), logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid, _ := GetUserIDFromContext(r.Context())
			v, err := visitor(r.Context(), uid)
			if err != nil {
				logger.Error("what of the content the visitor may not view", "error", err, "user_id", uid)
				httputil.InternalServerError(w)
				return
			}
			// Set even when nothing is hidden: the request is a visitor's
			// (entity.VisitorOf), not a background job's.
			next.ServeHTTP(w, r.WithContext(entity.WithContentVisitor(r.Context(), v)))
		})
	}
}
