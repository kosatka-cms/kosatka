package middleware

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// UserMemoMiddleware - a request reads a user by ID from the database once
// (entity.WithUserMemo): its session's, its handler's and its page data's
// lookups of the signed-in user are one query.
func UserMemoMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(entity.WithUserMemo(r.Context())))
	})
}
