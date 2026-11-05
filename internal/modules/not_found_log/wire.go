package notfoundlog

import (
	"net/http"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/not_found_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Wire - the listener logging the 404s, the job forgetting the old and
// redirected paths, the count by the Redirects menu item and on the tab,
// the log's pages.
func Wire(d modulekit.Deps) modulekit.Parts {
	logger := d.Logger.With("component", "not_found_log")
	repo := NewRepo(d.DB)
	recorder := NewRecorder(repo, d.Redirects, logger)
	modulekit.OnNotFound(manifest.Name, recorder.Record)
	modulekit.AddJob(manifest.Name, modulekit.Job{Name: "not_found_log_purge", Every: time.Hour, Run: recorder.Purge})
	modulekit.AddAdminMenuCount(manifest.Name, manifest.RedirectsPath, recorder.CountNew, i18n.N("Not found (404)"))
	modulekit.AddPageData(manifest.Name, func(r *http.Request, data map[string]any) {
		// The tab's count, on the Redirects page and the log's own.
		if !strings.HasPrefix(r.URL.Path, manifest.RedirectsPath) {
			return
		}
		if n, err := repo.CountNew(r.Context()); err == nil {
			data["notFoundCount"] = n
		}
	})
	c := NewController(modulekit.NewBaseController(d.Base), repo, recorder, d.Redirects)
	return modulekit.Parts{Routes: func(r modulekit.Routes) { RegisterRoutes(r.Mux, c, r.AdminOnly) }}
}
