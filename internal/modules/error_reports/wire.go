package errorreports

import (
	"github.com/kosatka-cms/kosatka/internal/modules/error_reports/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// Wire - the module's service, its e-mail (the Mail page), page data, the
// admin menu's count of new reports, controller and routes (modulekit).
func Wire(d modulekit.Deps) modulekit.Parts {
	modulekit.AddMail(mailDef())
	reports := NewErrorReportService(NewErrorReportRepository(d.DB, d.Users), d.SiteInformation, d.Users, d.Mail, d.Logger)
	modulekit.AddPageData(manifest.Name, PageData)
	modulekit.AddAdminMenuCount(manifest.Name, adminListURL, reports.CountNew)
	c := NewErrorReportController(d.Base, ErrorReportDeps{ErrorReportService: reports})
	return modulekit.Parts{Routes: func(r modulekit.Routes) {
		RegisterRoutes(r.Mux, c, ratelimit.NewRateLimiter(10, 3), r.AdminOnly)
	}}
}
