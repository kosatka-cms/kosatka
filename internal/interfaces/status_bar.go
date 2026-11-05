package interfaces

import (
	"context"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// StatusSources - what the status report shows besides its checks:
// the modules' state and the database (wiring).
type StatusSources struct {
	Modules  func(ctx context.Context) ([]entity.StatusModule, error)
	Database func(ctx context.Context) (driver, version string, err error)
}

var (
	statusReport  *service.StatusReport
	statusSources StatusSources
)

// StatusReportSources - the status checks and sources SetStatusReport
// set (the admin's status report page).
func StatusReportSources() (*service.StatusReport, StatusSources) {
	return statusReport, statusSources
}

// SetStatusReport sets the status checks and sources (wiring).
func SetStatusReport(r *service.StatusReport, s StatusSources) {
	statusReport, statusSources = r, s
}

// AddStatusCounts - the warning bar's counts on an admin page, for those
// who may open the status report.
func AddStatusCounts(r *http.Request, data map[string]any) {
	if statusReport == nil || data["isAdmin"] != true {
		return
	}
	warnings, errors := statusReport.Counts(r.Context())
	data["statusWarnings"], data["statusErrors"] = warnings, errors
}
