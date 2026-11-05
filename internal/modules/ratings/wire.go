package ratings

import (
	"bytes"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// Wire - the module's service, its hooks (the content type form, node
// pages), its criteria in Views, the settings page and the votes
// (modulekit).
func Wire(d modulekit.Deps) modulekit.Parts {
	ratings := NewService(NewRepository(d.DB))
	c := NewController(d.Base, ratings, d.Nodes, d.ContentTypes)
	RegisterHooks(ratings, d.Logger.With("component", "ratings.hooks"))
	registerViews(ratings, func(w *bytes.Buffer, block string, data any) error {
		return c.TemplateService().RenderNamed(w, block, data, widgetTemplates)
	})
	return modulekit.Parts{Routes: func(r modulekit.Routes) {
		RegisterAdminRoutes(r.Mux, c, r.AdminOnly)
		RegisterRoutes(r.Mux, r.Store, c, ratelimit.NewRateLimiter(30, 10))
	}}
}
