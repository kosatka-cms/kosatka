package actionlog

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/action_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// perPage - entries on a page of the log.
const perPage = 50

// keepChoices - the settings page's choice of how long entries are kept
// (days; 0 - forever).
var keepChoices = []int{30, 90, 180, 365, 0}

// Controller - the log and its settings.
type Controller struct {
	*modulekit.BaseController
	repo     *Repo
	recorder *Recorder
	types    modulekit.ContentTypeService
}

// NewController creates a Controller.
func NewController(base *modulekit.BaseController, repo *Repo, recorder *Recorder, types modulekit.ContentTypeService) *Controller {
	base.SetLogger(base.Logger().With("component", "action_log.handler"))
	return &Controller{BaseController: base, repo: repo, recorder: recorder, types: types}
}

// allowed - whether the request's user may see the log; answers 403 when
// not.
func (c *Controller) allowed(w http.ResponseWriter, r *http.Request) bool {
	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil || !c.AuthService().HasPermission(r.Context(), user, manifest.PermView) {
		c.RenderForbidden(w, r)
		return false
	}
	return true
}

// row - an entry as the log shows it.
type row struct {
	Entry
	Action string
	Tone   string
	Kind   string
}

// groupChoice - a group on the filter and the settings page.
type groupChoice struct {
	modulekit.ActionGroup
	On bool
}

// groups - the groups of the core and of the enabled modules.
func groups() []modulekit.ActionGroup {
	var out []modulekit.ActionGroup
	for _, g := range modulekit.ActionGroups() {
		if g.Module == "" || modulekit.ModuleEnabled(g.Module) {
			out = append(out, g)
		}
	}
	return out
}

// kindLabels - the core's kinds of objects as shown.
var kindLabels = map[string]string{
	"node": i18n.N("Content"), "comment": i18n.N("Comment"), "user": i18n.N("User"),
	"role": i18n.N("Role"), "module": i18n.N("Module"), "settings": i18n.N("Settings"),
}

// List renders GET /admin/reports/action-log: the entries the filter
// (who, the group, the object, the days; a user's or an object's own)
// shows, newest first.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	if !c.allowed(w, r) {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	f := Filter{Actor: q.Get("actor"), Group: q.Get("group"), Object: q.Get("object"), ObjectKind: q.Get("kind"), ObjectID: q.Get("id")}
	f.ActorID, _ = strconv.Atoi(q.Get("actor_id"))
	f.From, _ = time.ParseInLocation(time.DateOnly, q.Get("from"), time.Local)
	f.To, _ = time.ParseInLocation(time.DateOnly, q.Get("to"), time.Local)
	if f.ObjectKind == "" {
		f.ObjectID = ""
	}
	page := c.GetPageNumber(r)
	entries, total, err := c.repo.List(ctx, f, perPage, (page-1)*perPage)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "listing the action log")
		return
	}
	rows := make([]row, len(entries))
	var typeLabels map[string]string
	if c.types != nil {
		typeLabels = c.types.NodeTypeLabels(ctx)
	}
	for i, e := range entries {
		rows[i] = row{Entry: e, Action: e.Type, Kind: c.kindLabel(ctx, e.Object, typeLabels)}
		if t, ok := modulekit.ActionTypeOf(e.Type); ok {
			rows[i].Action, rows[i].Tone = i18n.T(ctx, t.Label), t.Tone
		}
	}
	data := c.AdminPageData(r)
	data["title"] = i18n.T(ctx, "Action log")
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["rows"] = rows
	data["total"] = total
	data["filter"] = f
	data["fromValue"], data["toValue"] = q.Get("from"), q.Get("to")
	data["groups"] = groups()
	data["keepDays"] = c.recorder.Settings(ctx).KeepDays
	data["pagination"] = pagination.New(page, total, perPage, manifest.AdminPath, q)
	c.RenderOrFail(w, r, "admin/action_log.html", data)
}

// kindLabel - what the object is, as shown: a content's type by its label
// (typeLabels - the site's and the modules' types; a type gone since - its
// machine name), the core's kind; "" - a module's own kind.
func (c *Controller) kindLabel(ctx context.Context, o modulekit.ActionObject, typeLabels map[string]string) string {
	if o.Kind == "node" && o.Subtype != "" {
		if l, ok := typeLabels[o.Subtype]; ok {
			return l
		}
		return o.Subtype
	}
	if l, ok := kindLabels[o.Kind]; ok {
		return i18n.T(ctx, l)
	}
	return ""
}

// Settings renders GET /admin/reports/action-log/settings and saves it
// (POST): how long entries are kept, the addresses, the groups recorded.
func (c *Controller) Settings(w http.ResponseWriter, r *http.Request) {
	if !c.allowed(w, r) {
		return
	}
	ctx := r.Context()
	if r.Method == http.MethodPost {
		c.saveSettings(w, r)
		return
	}
	st := c.recorder.Settings(ctx)
	var choices []groupChoice
	for _, g := range groups() {
		choices = append(choices, groupChoice{ActionGroup: g, On: !st.Off(g.Name)})
	}
	data := c.AdminPageData(r)
	data["title"] = i18n.T(ctx, "Action log settings")
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["settings"] = st
	data["groups"] = choices
	data["keepChoices"] = keepChoices
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error")
	c.RenderOrFail(w, r, "admin/action_log_settings.html", data)
}

// saveSettings handles the settings form: the groups not ticked are off
// (and those of disabled modules, not shown, stay as they were).
func (c *Controller) saveSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	old := c.recorder.Settings(ctx)
	st := Settings{LogIP: r.FormValue("log_ip") == "1"}
	st.KeepDays, _ = strconv.Atoi(r.FormValue("keep_days"))
	if !slices.Contains(keepChoices, st.KeepDays) {
		st.KeepDays = old.KeepDays
	}
	shown := map[string]bool{}
	for _, g := range groups() {
		shown[g.Name] = true
		if !slices.Contains(r.Form["groups"], g.Name) {
			st.GroupsOff = append(st.GroupsOff, g.Name)
		}
	}
	for _, g := range old.GroupsOff {
		if !shown[g] {
			st.GroupsOff = append(st.GroupsOff, g)
		}
	}
	if err := c.recorder.SaveSettings(ctx, st); err != nil {
		c.LogFailure(err, "saving the action log settings")
		http.Redirect(w, r, manifest.SettingsPath+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, manifest.SettingsPath+"?saved=1", http.StatusSeeOther)
}

// ObjectLink - the address of an object's own log (?kind=&id=).
func ObjectLink(kind, id string) string {
	return manifest.AdminPath + "?" + url.Values{"kind": {kind}, "id": {id}}.Encode()
}
