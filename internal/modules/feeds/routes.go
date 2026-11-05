package feeds

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// RegisterAdminRoutes registers the feeds admin behind admin (sign-in and
// the admin permission); the feeds themselves are served by
// FeedController.FeedFallback at their paths.
func RegisterAdminRoutes(r modulekit.Mux, c *FeedController, admin func(http.Handler) http.Handler) {
	r.Handle("GET /admin/config/feeds", admin(http.HandlerFunc(c.FeedList)))
	r.Handle("POST /admin/config/feeds", admin(http.HandlerFunc(c.FeedSave)))
	r.Handle("GET /admin/config/feeds/new", admin(http.HandlerFunc(c.FeedNew)))
	r.Handle("GET /admin/config/feeds/{id}", admin(http.HandlerFunc(c.FeedEdit)))
	r.Handle("POST /admin/config/feeds/{id}", admin(http.HandlerFunc(c.FeedSave)))
	r.Handle("POST /admin/config/feeds/{id}/delete", admin(http.HandlerFunc(c.FeedDelete)))
}
