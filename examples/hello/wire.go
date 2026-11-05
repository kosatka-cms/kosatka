package hello

import (
	"context"
	"net/http"
	"strconv"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// latestCount - the latest greetings block's setting: how many it shows.
var latestCount = modulekit.PluginBlockSetting{
	Key: "count", Label: i18n.N("How many"), Type: modulekit.PluginBlockSettingNumber, Default: "3", Min: 1, Max: 20,
}

// Wire - the module's repository, controller and route, its block (its
// setting, its data) and its line of the status report (modulekit).
func Wire(d modulekit.Deps) modulekit.Parts {
	greetings := NewRepository(d.DB)
	c := &Controller{BaseController: modulekit.NewBaseController(d.Base), greetings: greetings}
	modulekit.SetPluginBlockSettings(PluginBlockLatest, func(context.Context) ([]modulekit.PluginBlockSetting, error) {
		return []modulekit.PluginBlockSetting{latestCount}, nil
	})
	// The block's greetings, loaded with the blocks' cache: its template
	// gets them as .Data.
	modulekit.SetPluginBlockData(PluginBlockLatest, func(ctx context.Context, p modulekit.BlockPlacement) (any, error) {
		n, _ := strconv.Atoi(p.PluginSettingOr(latestCount))
		return greetings.Latest(ctx, n)
	})
	return modulekit.Parts{
		Routes: func(r modulekit.Routes) {
			r.Mux.Handle("GET /hello", http.HandlerFunc(c.Page))
		},
		Requirements: func(ctx context.Context) []modulekit.Requirement {
			req := modulekit.Requirement{Title: i18n.T(ctx, "Greetings")}
			n, err := greetings.Count(ctx)
			if err != nil {
				req.Severity, req.Value = modulekit.RequirementError, i18n.T(ctx, "Unavailable")
				return []modulekit.Requirement{req}
			}
			req.Value = strconv.Itoa(n)
			if n == 0 {
				req.Severity = modulekit.RequirementWarning
				req.Description = i18n.T(ctx, "No greetings yet.")
			}
			return []modulekit.Requirement{req}
		},
	}
}
