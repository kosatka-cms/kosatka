package feeds

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/modules/feeds/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Wire - the module's service, controller, admin routes and feed pages
// (the 404-path fallback) (modulekit); an enabled feed's address is the
// module's (AddPathOwner).
func Wire(d modulekit.Deps) modulekit.Parts {
	feeds := NewFeedService(NewFeedRepository(d.DB), d.NodeRepo, d.ContentTypes, func(ctx context.Context, path string) (string, error) {
		return modulekit.PathOwner(ctx, path, manifest.Name)
	}, d.Logger)
	modulekit.AddPathOwner(manifest.Name, func(ctx context.Context, path string) (bool, error) {
		return feeds.FeedByPath(ctx, path) != nil, nil
	})
	c := NewFeedController(modulekit.NewBaseController(d.Base), feeds, d.Teasers, d.SiteInformation, d.ContentTypes, d.ImageStyles)
	return modulekit.Parts{
		Routes: func(r modulekit.Routes) {
			RegisterAdminRoutes(r.Mux, c, r.AdminOnly)
		},
		Fallback: c.FeedFallback,
	}
}
