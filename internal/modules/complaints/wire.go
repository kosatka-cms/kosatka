package complaints

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/modules/complaints/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// Wire - the module's service, its e-mail (the Mail page), the Report
// links of content pages and the window's reasons (page data), the admin
// menu's count of the queue, the complaints about deleted content
// removed, the controller and routes (modulekit).
func Wire(d modulekit.Deps) modulekit.Parts {
	modulekit.AddMail(mailDef())
	repo := NewRepo(d.DB)
	svc := NewService(ServiceDeps{Repo: repo, Nodes: d.NodeRepo, Comments: d.Comments, Deleter: d.NodeDeleter,
		Site: d.SiteInformation, Users: d.Users, Mail: d.Mail, Logger: d.Logger.With("component", "complaints")})
	c := NewController(d.Base, ControllerDeps{Service: svc, Repo: repo, Auth: d.Auth, Users: d.Users, Nodes: d.NodeRepo,
		Comments: d.Comments, ContentTypes: d.ContentTypes})
	d.NodeDeleter.AfterDelete(func(ctx context.Context, n modulekit.Node) {
		if !modulekit.ModuleEnabled(manifest.Name) {
			return
		}
		if err := repo.DeleteByNode(ctx, n.ID); err != nil {
			d.Logger.Warn("complaints about deleted content not removed", "nodeID", n.ID, "error", err)
		}
	})
	modulekit.AddNodePage(manifest.Name, c.NodePage)
	modulekit.AddPageData(manifest.Name, c.PageData)
	modulekit.AddAdminMenuCount(manifest.Name, manifest.AdminQueuePath, svc.CountNew)
	return modulekit.Parts{Routes: func(r modulekit.Routes) {
		RegisterRoutes(r.Mux, c, ratelimit.NewRateLimiter(10, 5), modulekit.AuthJSONMiddleware(r.Store), r.AdminOnly)
	}}
}
