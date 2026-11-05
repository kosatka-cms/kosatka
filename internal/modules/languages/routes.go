package languages

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/modules/languages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// RegisterRoutes registers the Languages page and its actions (behind
// admin).
func RegisterRoutes(r modulekit.Mux, c *Controller, admin func(http.Handler) http.Handler) {
	r.Handle("GET "+manifest.AdminPath, admin(http.HandlerFunc(c.List)))
	r.Handle("POST "+manifest.AdminPath, admin(http.HandlerFunc(c.Add)))
	r.Handle("POST "+manifest.AdminPath+"/settings", admin(http.HandlerFunc(c.Settings)))
	r.Handle("POST "+manifest.AdminPath+"/{code}/remove", admin(http.HandlerFunc(c.Remove)))
	r.Handle("POST "+manifest.AdminPath+"/{code}/default", admin(http.HandlerFunc(c.MakeDefault)))
}

// RegisterTranslationRoutes registers a content item's Translations tab
// and making a translation (behind auth, the sign-in).
func RegisterTranslationRoutes(r modulekit.Mux, c *TranslationsController, auth func(http.Handler) http.Handler) {
	r.Handle("GET /node/{id}/translations", auth(http.HandlerFunc(c.Page)))
	r.Handle("POST /node/{id}/translations/{lang}", auth(http.HandlerFunc(c.Translate)))
}
