package middleware

import (
	"html"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// databaseDownRetryAfter - the seconds a client is asked to wait before
// trying again (Retry-After) while the database is gone.
const databaseDownRetryAfter = "30"

// DatabaseDownMiddleware answers 503 at once while down says the database
// doesn't answer (dbwatch.Watch.Down), instead of each request waiting
// for a connection and failing with a 500: a page "temporarily
// unavailable" titled by the site's information in memory (site, nil -
// none), JSON for /api/; never cached, Retry-After set. Every request:
// the static files too would wait in the layers inside, which read the
// database (in production the web server in front serves them).
func DatabaseDownMiddleware(down func() bool, site func() *entity.SiteInformation) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if down == nil || !down() {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Retry-After", databaseDownRetryAfter)
			w.Header().Set("Cache-Control", "no-store")
			msg := i18n.T(r.Context(), "The site is temporarily unavailable. Please try again in a few minutes.")
			if strings.HasPrefix(r.URL.Path, "/api/") {
				httputil.JSONError(w, msg, http.StatusServiceUnavailable)
				return
			}
			var info *entity.SiteInformation
			if site != nil {
				info = site()
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write(unavailableHTML("<p>"+html.EscapeString(msg)+"</p>", info))
		})
	}
}
