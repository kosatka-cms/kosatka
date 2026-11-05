package middleware

import "net/http"

// MaxRequestBody - the largest request body the application reads (64
// MiB): the CSRF check parses a form's body before any route sees it, so a
// route's own limit comes too late for an oversized upload. A proxy in
// front (nginx client_max_body_size) usually limits it more.
const MaxRequestBody = 64 << 20

// BodyLimitMiddleware caps every request body at limit bytes: reading past
// it fails (and the form parse with it).
func BodyLimitMiddleware(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
