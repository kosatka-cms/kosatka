package notfoundlog

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/modules/not_found_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// RegisterRoutes registers the log, its actions and its settings (behind
// admin, like the Redirects page).
func RegisterRoutes(r modulekit.Mux, c *Controller, admin func(http.Handler) http.Handler) {
	r.Handle("GET "+manifest.AdminPath, admin(http.HandlerFunc(c.List)))
	r.Handle("POST "+manifest.AdminPath+"/{id}/redirect", admin(http.HandlerFunc(c.Redirect)))
	r.Handle("POST "+manifest.AdminPath+"/{id}/ignore", admin(http.HandlerFunc(c.Ignore)))
	r.Handle("POST "+manifest.AdminPath+"/{id}/restore", admin(http.HandlerFunc(c.Ignore)))
	r.Handle("POST "+manifest.AdminPath+"/clear", admin(http.HandlerFunc(c.Clear)))
	r.Handle("GET "+manifest.SettingsPath, admin(http.HandlerFunc(c.Settings)))
	r.Handle("POST "+manifest.SettingsPath, admin(http.HandlerFunc(c.Settings)))
}
