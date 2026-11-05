package actionlog

import (
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/action_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Wire - the listener keeping the actions the site records, the job
// deleting the old entries, the log beside what it's about (the content
// form's row, the users list's button), its block on the admin's
// Overview and the log's pages.
func Wire(d modulekit.Deps) modulekit.Parts {
	logger := d.Logger.With("component", "action_log")
	repo := NewRepo(d.DB)
	recorder := NewRecorder(repo, d.Users, logger)
	modulekit.OnAction(manifest.Name, recorder.Record)
	modulekit.AddJob(manifest.Name, modulekit.Job{Name: "action_log_purge", Every: time.Hour, Run: recorder.Purge})
	hooks := Hooks{repo: repo, users: d.Users, auth: d.Auth}
	modulekit.AddPageData(manifest.Name, hooks.PageData)
	modulekit.AddNodeSettings(hooks.NodeSettings())
	modulekit.AddDashboardBlock(modulekit.DashboardBlock{Name: "action_log", Module: manifest.Name, Title: i18n.N("Recent actions"),
		Link: manifest.AdminPath, LinkLabel: i18n.N("Action log"), Permission: manifest.PermView, Data: dashboardRows(repo)})
	c := NewController(modulekit.NewBaseController(d.Base), repo, recorder, d.ContentTypes)
	return modulekit.Parts{Routes: func(r modulekit.Routes) { RegisterRoutes(r.Mux, c, r.AdminOnly) }}
}
