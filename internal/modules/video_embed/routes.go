package videoembed

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// RegisterRoutes registers the settings page (behind admin) and the
// editor's check of a link (signed in, rate limited: it may ask a video
// service).
func RegisterRoutes(r modulekit.Mux, store *sessions.CookieStore, c *Controller, admin func(http.Handler) http.Handler) {
	r.Handle("GET "+manifest.AdminPath, admin(http.HandlerFunc(c.Settings)))
	r.Handle("POST "+manifest.AdminPath, admin(http.HandlerFunc(c.SaveSettings)))
	rl := ratelimit.RateLimitMiddleware(ratelimit.NewRateLimiter(30, 10), httputil.ClientIP)
	r.Handle("GET /api/video-embed", modulekit.AuthMiddleware(store)(rl(http.HandlerFunc(c.Resolve))))
}
