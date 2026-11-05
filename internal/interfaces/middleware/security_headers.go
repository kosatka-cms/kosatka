package middleware

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// hstsMaxAge - how long a browser keeps to HTTPS for the site after one
// visit over it (a year; the site's own host only: another site of the
// same domain may still be on HTTP).
const hstsMaxAge = "max-age=31536000"

// SecurityHeadersMiddleware sets the headers of every response: no
// guessing of a type other than the one sent (nosniff), no framing by
// another site (a page under someone else's buttons), the address sent
// to another site without its path and query (they may carry a token),
// and, over HTTPS, HSTS.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if httputil.IsHTTPS(r) {
			h.Set("Strict-Transport-Security", hstsMaxAge)
		}
		next.ServeHTTP(w, r)
	})
}
