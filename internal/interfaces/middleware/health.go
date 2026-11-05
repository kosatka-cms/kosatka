package middleware

import (
	"context"
	"net/http"
	"time"
)

// HealthPath - the address answering whether the site is up.
const HealthPath = "/healthz"

// HealthMiddleware answers GET HealthPath before every other layer (the
// maintenance page, a session, the routes): 200 "ok" when ping (the
// database) answers within a few seconds, else 503 - for a load balancer,
// a container's health check, or app sites update waiting for a site it
// restarted. Any other request goes on to next.
func HealthMiddleware(ping func(context.Context) error) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != HealthPath || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			if err := ping(ctx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("database unavailable\n"))
				return
			}
			_, _ = w.Write([]byte("ok\n"))
		})
	}
}
