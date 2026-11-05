package actionlog

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/modules/action_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// RegisterRoutes registers the log and its settings (behind admin; the
// handlers check the log's permission).
func RegisterRoutes(r modulekit.Mux, c *Controller, admin func(http.Handler) http.Handler) {
	r.Handle("GET "+manifest.AdminPath, admin(http.HandlerFunc(c.List)))
	r.Handle("GET "+manifest.SettingsPath, admin(http.HandlerFunc(c.Settings)))
	r.Handle("POST "+manifest.SettingsPath, admin(http.HandlerFunc(c.Settings)))
}
