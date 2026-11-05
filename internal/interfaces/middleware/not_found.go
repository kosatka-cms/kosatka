package middleware

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// NotFoundHooksMiddleware tells the hooks of service.OnNotFound of a GET
// or HEAD answered 404 (its path and referrer). It sits inside the
// redirects and the blocks: a redirected path or a blocked bot never gets
// here. Without hooks it only passes the request on.
func NotFoundHooksMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hooks := service.NotFoundHooks()
		if len(hooks) == 0 || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			next.ServeHTTP(w, r)
			return
		}
		rec := &redirectRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status != http.StatusNotFound {
			return
		}
		for _, fn := range hooks {
			fn(r.Context(), r.URL.Path, r.Referer())
		}
	})
}
