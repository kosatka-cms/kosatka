package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// RecoverMiddleware turns a handler's panic into a 500 and an error line
// with the stack in the site's log; net/http would drop the connection
// and print it to stderr only. http.ErrAbortHandler passes on: it is the
// way to abort a response.
func RecoverMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}
				logger.Error("panic serving a request", "method", r.Method, "path", r.URL.Path, "panic", rec, "stack", string(debug.Stack()))
				httputil.InternalServerError(w)
			}()
			next.ServeHTTP(w, r)
		})
	}
}
