package notfoundlog

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/modules/not_found_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// perPage - paths on a page of the log.
const perPage = 50

// keepChoices - the settings page's choice of how long a path not asked
// for is kept (days).
var keepChoices = []int{30, 90, 180, 365}

// Controller - the log (a tab of the Redirects page) and its settings.
type Controller struct {
	*modulekit.BaseController
	repo      *Repo
	recorder  *Recorder
	redirects modulekit.RedirectService
}

// NewController creates a Controller.
func NewController(base *modulekit.BaseController, repo *Repo, recorder *Recorder, redirects modulekit.RedirectService) *Controller {
	base.SetLogger(base.Logger().With("component", "not_found_log.handler"))
	return &Controller{BaseController: base, repo: repo, recorder: recorder, redirects: redirects}
}

// row - a path as the log shows it: where the last visitor came from as
// a link (RefURL, "" - none) and a label (RefLabel - a path for this
// site's own pages).
type row struct {
	Path
	RefURL   string
	RefLabel string
	OwnRef   bool
}

// referrer - a referrer as shown: this site's own page as its path.
func referrer(raw, host string) (link, label string, own bool) {
	u, err := url.Parse(raw)
	if raw == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", raw, false
	}
	if strings.EqualFold(u.Host, host) {
		return u.RequestURI(), u.RequestURI(), true
	}
	return raw, u.Host + strings.TrimSuffix(u.RequestURI(), "/"), false
}

// List renders GET /admin/config/redirects/not-found: the paths the
// filter (a part of the path; new, ignored or all) shows, the most asked
// for first; a path a redirect answers now is left out.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	f := Filter{Q: q.Get("q"), Status: q.Get("status")}
	if !slices.Contains([]string{StatusNew, StatusIgnored, StatusAll}, f.Status) {
		f.Status = StatusNew
	}
	page := c.GetPageNumber(r)
	paths, total, err := c.repo.List(ctx, f, perPage, (page-1)*perPage)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "listing the 404 log")
		return
	}
	var rows []row
	for _, p := range paths {
		if c.recorder.Redirected(p.Path) {
			continue
		}
		rw := row{Path: p}
		rw.RefURL, rw.RefLabel, rw.OwnRef = referrer(p.Referrer, r.Host)
		rows = append(rows, rw)
	}
	data := c.AdminPageData(r)
	data["title"] = i18n.T(ctx, "Not found (404)")
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["rows"] = rows
	data["total"] = total
	data["filter"] = f
	data["back"] = r.URL.RequestURI()
	data["keepDays"] = c.recorder.Settings(ctx).KeepDays
	data["notFoundCurrent"] = true
	data["error"] = q.Get("error")
	switch q.Get("done") {
	case "redirect":
		data["doneRedirect"] = true
	case "ignored":
		data["doneIgnored"] = true
	case "restored":
		data["doneRestored"] = true
	}
	data["pagination"] = pagination.New(page, total, perPage, manifest.AdminPath, q)
	c.RenderOrFail(w, r, "admin/not_found_log.html", data)
}

// backTo - where an action returns: the list it came from (its "back"
// field, the log's own pages only) with done or error set.
func backTo(r *http.Request, key, value string) string {
	to := r.FormValue("back")
	if to != manifest.AdminPath && !strings.HasPrefix(to, manifest.AdminPath+"?") {
		to = manifest.AdminPath
	}
	u, _ := url.Parse(to)
	q := u.Query()
	q.Del("done")
	q.Del("error")
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

// Redirect handles POST /admin/config/redirects/not-found/{id}/redirect:
// a redirect (the site's, on the Redirects page too) from the path to the
// address given; the path leaves the log.
func (c *Controller) Redirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, backTo(r, "error", "redirect"), http.StatusSeeOther)
		return
	}
	p, err := c.repo.ByID(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading a 404 path", "id", id)
		http.Redirect(w, r, backTo(r, "error", "redirect"), http.StatusSeeOther)
		return
	}
	to := strings.TrimSpace(r.FormValue("to"))
	if to == "" || to == p.Path {
		http.Redirect(w, r, backTo(r, "error", "target"), http.StatusSeeOther)
		return
	}
	if !strings.HasPrefix(to, "/") && !strings.HasPrefix(to, "http://") && !strings.HasPrefix(to, "https://") {
		to = "/" + to
	}
	if err := c.redirects.Create(ctx, strings.TrimPrefix(p.Path, "/"), to); err != nil {
		c.LogFailure(err, "creating a redirect from a 404 path", "path", p.Path)
		http.Redirect(w, r, backTo(r, "error", "redirect"), http.StatusSeeOther)
		return
	}
	if err := c.repo.Delete(ctx, id); err != nil {
		c.LogFailure(err, "forgetting a redirected 404 path", "id", id)
	}
	http.Redirect(w, r, backTo(r, "done", "redirect"), http.StatusSeeOther)
}

// Ignore handles POST …/{id}/ignore and …/{id}/restore: the path leaves
// the new ones, or comes back to them.
func (c *Controller) Ignore(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	ignored := strings.HasSuffix(r.URL.Path, "/ignore")
	if ok {
		if err := c.repo.SetIgnored(r.Context(), id, ignored); err != nil {
			c.LogFailure(err, "ignoring a 404 path", "id", id)
			http.Redirect(w, r, backTo(r, "error", "save"), http.StatusSeeOther)
			return
		}
	}
	done := "restored"
	if ignored {
		done = "ignored"
	}
	http.Redirect(w, r, backTo(r, "done", done), http.StatusSeeOther)
}

// Settings renders GET /admin/config/redirects/not-found/settings and
// saves it (POST): how long paths are kept, the menu's count, the paths
// never logged.
func (c *Controller) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		old := c.recorder.Settings(ctx)
		st := Settings{ShowCount: r.FormValue("show_count") == "1", Ignore: patternLines(r.FormValue("ignore"))}
		st.KeepDays, _ = strconv.Atoi(r.FormValue("keep_days"))
		if !slices.Contains(keepChoices, st.KeepDays) {
			st.KeepDays = old.KeepDays
		}
		if err := c.recorder.SaveSettings(ctx, st); err != nil {
			c.LogFailure(err, "saving the 404 log settings")
			http.Redirect(w, r, manifest.SettingsPath+"?error=save", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, manifest.SettingsPath+"?saved=1", http.StatusSeeOther)
		return
	}
	st := c.recorder.Settings(ctx)
	data := c.AdminPageData(r)
	data["title"] = i18n.T(ctx, "404 log settings")
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["settings"] = st
	data["ignoreText"] = strings.Join(st.Ignore, "\n")
	data["keepChoices"] = keepChoices
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error")
	data["cleared"] = r.URL.Query().Get("cleared")
	c.RenderOrFail(w, r, "admin/not_found_log_settings.html", data)
}

// Clear handles POST /admin/config/redirects/not-found/clear: every path
// forgotten (the redirects and the settings stay).
func (c *Controller) Clear(w http.ResponseWriter, r *http.Request) {
	n, err := c.repo.Clear(r.Context())
	if err != nil {
		c.LogFailure(err, "clearing the 404 log")
		http.Redirect(w, r, manifest.SettingsPath+"?error=clear", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, manifest.SettingsPath+"?cleared="+strconv.Itoa(n), http.StatusSeeOther)
}
