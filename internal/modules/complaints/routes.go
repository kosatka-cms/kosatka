package complaints

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/modules/complaints/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// RegisterRoutes registers the complaint's endpoint (behind signedIn,
// rate limited), the queue with its decisions and the settings (behind
// admin; the handlers check the module's permission).
func RegisterRoutes(r modulekit.Mux, c *Controller, submitRL *ratelimit.RateLimiter, signedIn, admin func(http.Handler) http.Handler) {
	rl := ratelimit.RateLimitMiddleware(submitRL, httputil.ClientIP)
	r.Handle("POST /api/complaints", signedIn(rl(http.HandlerFunc(c.Submit))))
	r.Handle("GET "+manifest.AdminQueuePath, admin(http.HandlerFunc(c.Queue)))
	r.Handle("POST "+manifest.AdminQueuePath+"/{id}/{action}", admin(http.HandlerFunc(c.Decide)))
	r.Handle("GET "+manifest.AdminSettingsPath, admin(http.HandlerFunc(c.Settings)))
	r.Handle("POST "+manifest.AdminSettingsPath, admin(http.HandlerFunc(c.Settings)))
}
