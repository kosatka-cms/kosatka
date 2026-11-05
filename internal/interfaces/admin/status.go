package admin

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/buildinfo"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// StatusReportPage GET /admin/reports/status - the engine's build, the
// environment, the modules and every check (service.StatusReport).
func (c *Controller) StatusReportPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	statusReport, statusSources := interfaces.StatusReportSources()
	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Status report")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["build"] = buildinfo.Get()
	data["theme"] = paths.Theme()
	if statusSources.Database != nil {
		driver, version, err := statusSources.Database(ctx)
		c.LogFailure(err, "status report: database version")
		data["dbDriver"], data["dbVersion"] = driver, version
	}
	if statusSources.Modules != nil {
		modules, err := statusSources.Modules(ctx)
		c.LogFailure(err, "status report: modules")
		data["modules"] = modules
	}
	if statusReport != nil {
		data["requirements"] = statusReport.Run(ctx)
		// The bar on this page shows what the run just found.
		interfaces.AddStatusCounts(r, data)
	}
	c.RenderOrFail(w, r, "admin/reports/status.html", data)
}
