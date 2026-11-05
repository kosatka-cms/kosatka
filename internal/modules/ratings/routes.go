package ratings

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/modules/ratings/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// RegisterAdminRoutes registers the settings page (administrators).
func RegisterAdminRoutes(r modulekit.Mux, c *Controller, admin func(http.Handler) http.Handler) {
	r.Handle("GET "+manifest.SettingsPath, admin(http.HandlerFunc(c.SettingsPage)))
	r.Handle("POST "+manifest.SettingsPath, admin(http.HandlerFunc(c.SettingsPage)))
}

// RegisterRoutes registers the widget of a content page and its votes
// (any visitor, the votes rate limited per client IP).
func RegisterRoutes(r modulekit.Mux, store *sessions.CookieStore, c *Controller, voteRL *ratelimit.RateLimiter) {
	optional := modulekit.OptionalAuthMiddleware(store)
	r.Handle("GET /node/{id}/rating", optional(http.HandlerFunc(c.Widget)))
	r.Handle("POST /node/{id}/rating", ratelimit.RateLimitMiddleware(voteRL, httputil.ClientIP)(optional(http.HandlerFunc(c.Vote))))
}
