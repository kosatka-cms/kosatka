package errorreports

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// RegisterRoutes registers the report form's endpoint (rate limited) and
// the admin list and its "fixed" and "back to the new ones" actions (behind admin - sign-in and the
// admin permission).
func RegisterRoutes(r modulekit.Mux, erc *ErrorReportController, submitRL *ratelimit.RateLimiter, admin func(http.Handler) http.Handler) {
	rl := ratelimit.RateLimitMiddleware(submitRL, httputil.ClientIP)
	r.Handle("POST /api/error-report", rl(http.HandlerFunc(erc.Submit)))
	r.Handle("GET "+adminListURL, admin(http.HandlerFunc(erc.AdminList)))
	r.Handle("POST "+adminListURL+"/{id}/resolve", admin(http.HandlerFunc(erc.Resolve)))
	r.Handle("POST "+adminListURL+"/{id}/reopen", admin(http.HandlerFunc(erc.Reopen)))
}
