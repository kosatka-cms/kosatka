package admin

import (
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// MaintenanceSettings handles GET and POST /admin/config/maintenance: the
// maintenance mode and its message, shown and saved.
func (c *Controller) MaintenanceSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method == http.MethodPost {
		enabled := r.FormValue("enabled") == "1"
		message := strings.TrimSpace(r.FormValue("message"))
		if message == "" {
			message = i18n.T(ctx, "<p>The site is currently under maintenance. We should be back shortly. Thank you for your patience.</p>")
		}
		if err := c.maintenanceService.UpdateSettings(ctx, enabled, message); err != nil {
			c.LogFailure(err, "error updating maintenance settings")
			http.Redirect(w, r, "/admin/config/maintenance?error=1", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/config/maintenance?saved=1", http.StatusSeeOther)
		return
	}

	settings, err := c.maintenanceService.GetSettings(ctx)
	if err != nil {
		c.LogFailure(err, "error getting maintenance settings")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Maintenance mode")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["settings"] = settings
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error") == "1"

	c.RenderOrFail(w, r, "admin/config/maintenance.html", data)
}

// viewDaysKeepChoices - how many days the views by day are kept (0 - for
// ever), the Statistics page's choices.
var viewDaysKeepChoices = []int{30, 90, 365, 0}

// StatisticsSettings GET/POST /admin/config/statistics
func (c *Controller) StatisticsSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method == http.MethodPost {
		keep, ok := formChoice(r, "daily_views_keep_days", viewDaysKeepChoices)
		if !ok {
			http.Redirect(w, r, "/admin/config/statistics?error=1", http.StatusSeeOther)
			return
		}
		settings := entity.StatisticsSettings{ViewCountingEnabled: r.FormValue("view_counting_enabled") == "1", DailyViewsKeepDays: keep}
		if err := c.viewCountingService.UpdateSettings(ctx, settings); err != nil {
			c.LogFailure(err, "error updating statistics settings")
			http.Redirect(w, r, "/admin/config/statistics?error=1", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/config/statistics?saved=1", http.StatusSeeOther)
		return
	}

	settings, err := c.viewCountingService.GetSettings(ctx)
	if err != nil {
		c.LogFailure(err, "error getting statistics settings")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Statistics")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["viewCountingEnabled"] = settings.ViewCountingEnabled
	data["viewDaysKeepOptions"] = settingOptions(viewDaysKeepChoices, settings.DailyViewsKeepDays, func(v int) string {
		if v == 0 {
			return i18n.T(ctx, "for ever")
		}
		return i18n.Plural(ctx, v, "@count day", "@count days")
	})
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error") == "1"

	c.RenderOrFail(w, r, "admin/config/statistics.html", data)
}

const perfErrFlashCategory = "admin_perf_err"

const perfErrFlashMaxLen = 600

func (c *Controller) savePerfErrFlash(w http.ResponseWriter, r *http.Request, msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	if len(msg) > perfErrFlashMaxLen {
		msg = msg[:perfErrFlashMaxLen] + "…"
	}
	sess, err := c.store.Get(r, entity.SessionName)
	if err != nil {
		return
	}
	sess.AddFlash(msg, perfErrFlashCategory)
	if err := sess.Save(r, w); err != nil {
		c.logger.Warn("perf err flash: session save failed", "error", err)
	}
}

func (c *Controller) takePerfErrFlash(w http.ResponseWriter, r *http.Request) string {
	sess, err := c.store.Get(r, entity.SessionName)
	if err != nil {
		return ""
	}
	flashes := sess.Flashes(perfErrFlashCategory)
	if len(flashes) == 0 {
		return ""
	}
	if err := sess.Save(r, w); err != nil {
		c.logger.Warn("perf err flash: session save failed", "error", err)
	}
	last := flashes[len(flashes)-1]
	if s, ok := last.(string); ok {
		return s
	}
	return ""
}

// PerformanceClearThumbnails handles POST: removes resize/ and the image style caches (styles/).
func (c *Controller) PerformanceClearThumbnails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if c.imageStorage == nil {
		c.logger.Error("performance clear thumbnails: image storage is nil")
		c.savePerfErrFlash(w, r, i18n.T(r.Context(), "image storage is not initialized"))
		http.Redirect(w, r, "/admin/config/performance?perf=err", http.StatusSeeOther)
		return
	}
	if err := c.imageStorage.ClearAllGeneratedImages(r.Context()); err != nil {
		c.LogFailure(err, "performance clear thumbnails")
		c.savePerfErrFlash(w, r, i18n.ErrorText(r.Context(), err))
		http.Redirect(w, r, "/admin/config/performance?perf=err", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/performance?perf=thumbs_ok", http.StatusSeeOther)
}

// PerformanceRefreshMemoryCache handles POST: synchronously refreshes CacheService (sidebar, View blocks).
func (c *Controller) PerformanceRefreshMemoryCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if c.cacheService == nil {
		c.logger.Error("performance refresh memory cache: cache service is nil")
		http.Redirect(w, r, "/admin/config/performance?perf=err", http.StatusSeeOther)
		return
	}
	if err := c.cacheService.RefreshNow(r.Context()); err != nil {
		c.LogFailure(err, "performance refresh memory cache")
		http.Redirect(w, r, "/admin/config/performance?perf=err", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/performance?perf=cache_ok", http.StatusSeeOther)
}

// PerformanceSettings renders GET /admin/config/performance - the performance actions page.
func (c *Controller) PerformanceSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Performance")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", data["title"])}
	switch r.URL.Query().Get("perf") {
	case "thumbs_ok":
		data["perfMessage"] = i18n.T(r.Context(), "Thumbnails and the image style cache on disk have been cleared.")
		data["perfAlertSuccess"] = true
	case "cache_ok":
		data["perfMessage"] = i18n.T(r.Context(), "The in-memory cache has been refreshed.")
		data["perfAlertSuccess"] = true
	case "err":
		data["perfMessage"] = i18n.T(r.Context(), "The action could not be completed.")
		data["perfAlertSuccess"] = false
		if detail := c.takePerfErrFlash(w, r); detail != "" {
			data["perfDetail"] = detail
		}
	}
	c.RenderOrFail(w, r, "admin/config/performance.html", data)
}
