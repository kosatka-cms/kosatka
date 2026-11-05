package complaints

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/modules/complaints/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// queuePerPage - the targets on a page of the queue.
const queuePerPage = 25

// hideAfterChoices - the settings' choices of how many complaints hide an
// item (0 - none hides).
var hideAfterChoices = []int{3, 5, 10, 0}

// Controller - the module's pages: the complaint's endpoint, the queue,
// the settings.
type Controller struct {
	*modulekit.BaseController
	service      *Service
	repo         *Repo
	auth         modulekit.AuthService
	users        modulekit.UserRepository
	nodes        modulekit.NodesRepository
	comments     modulekit.CommentsService
	contentTypes modulekit.ContentTypeService
}

// ControllerDeps - what NewController needs besides the BaseDeps.
type ControllerDeps struct {
	Service      *Service
	Repo         *Repo
	Auth         modulekit.AuthService
	Users        modulekit.UserRepository
	Nodes        modulekit.NodesRepository
	Comments     modulekit.CommentsService
	ContentTypes modulekit.ContentTypeService
}

// NewController - the module's controller.
func NewController(deps modulekit.BaseDeps, d ControllerDeps) *Controller {
	base := modulekit.NewBaseController(deps)
	base.SetLogger(deps.Logger.With("component", "complaints.handler"))
	return &Controller{BaseController: base, service: d.Service, repo: d.Repo, auth: d.Auth, users: d.Users,
		nodes: d.Nodes, comments: d.Comments, contentTypes: d.ContentTypes}
}

// may - whether the request's user has permission (full access has all).
func (c *Controller) may(r *http.Request, permission string) (*modulekit.User, bool) {
	user, _ := c.GetCurrentUser(r)
	if user == nil {
		return nil, false
	}
	return user, c.auth.HasPermission(r.Context(), user, permission)
}

// submitBody - a complaint as the page's script sends it.
type submitBody struct {
	Kind   string `json:"kind"`
	ID     int    `json:"id"`
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

// Submit handles POST /api/complaints (signed in, CSRF, rate limited): a
// complaint about a content item or a comment; the answer's message is
// for the visitor.
func (c *Controller) Submit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := c.may(r, manifest.PermComplain)
	if !ok {
		httputil.JSONError(w, i18n.T(ctx, "You may not report content."), http.StatusForbidden)
		return
	}
	var body submitBody
	if err := httputil.DecodeJSON(r, &body); err != nil {
		httputil.JSONError(w, "invalid json", http.StatusBadRequest)
		return
	}
	switch err := c.service.Complain(ctx, user, body.Kind, body.ID, body.Reason, body.Note); {
	case errors.Is(err, ErrNotFound):
		httputil.JSONError(w, i18n.ErrorText(ctx, err), http.StatusNotFound)
	case errors.Is(err, ErrNotAllowed), errors.Is(err, ErrReason):
		httputil.JSONError(w, i18n.ErrorText(ctx, err), http.StatusBadRequest)
	case err != nil:
		c.LogFailure(err, "taking a complaint", "kind", body.Kind, "id", body.ID)
		httputil.JSONError(w, i18n.T(ctx, "Could not send the complaint. Please try again later."), http.StatusInternalServerError)
	default:
		httputil.SendJSON(w, http.StatusOK, map[string]any{"ok": true, "message": i18n.T(ctx, "Thank you: the moderators will look at it.")})
	}
}

// nodeLink - the module's part of a content item's page (AddNodePage):
// the item when the viewer may report it.
type nodeLink struct {
	ID    int
	Title string
}

// NodePage - the "Report" link under a public content item of a type the
// settings let be reported, for a signed-in visitor who may report and
// isn't its author.
func (c *Controller) NodePage(ctx context.Context, node modulekit.Node) modulekit.NodePageParts {
	uid, ok := modulekit.ContextUserID(ctx)
	if !ok || uid <= 0 || !node.IsPublic() || node.CreatedBy.ID == uid {
		return modulekit.NodePageParts{}
	}
	settings, err := c.repo.Settings(ctx)
	if err != nil || !settings.NodeTypeAllowed(string(node.Type)) {
		modulekit.LogFallback(c.Logger(), err, "the complaints' settings (no Report link)")
		return modulekit.NodePageParts{}
	}
	user, err := c.users.ByID(ctx, uid)
	if err != nil || user == nil || !c.auth.HasPermission(ctx, user, manifest.PermComplain) {
		return modulekit.NodePageParts{}
	}
	return modulekit.NodePageParts{Data: nodeLink{ID: node.ID, Title: node.Title}}
}

// PageData - every page's data (AddPageData): the reasons of the
// complaint's window, data["complaintsReasons"], and whether comments may
// be reported, data["complaintsComments"], for a signed-in user who may
// report (no reasons - no window, and the comments' links go away).
func (c *Controller) PageData(r *http.Request, data map[string]any) {
	user, _ := data["user"].(*modulekit.User)
	if user == nil || !c.auth.HasPermission(r.Context(), user, manifest.PermComplain) {
		return
	}
	settings, err := c.repo.Settings(r.Context())
	if err != nil {
		modulekit.LogFallback(c.Logger(), err, "the complaints' settings (no Report window)")
		return
	}
	data["complaintsReasons"] = settings.ReasonList()
	data["complaintsOther"] = OtherReason
	data["complaintsComments"] = settings.Comments
}

// queueRow - a target as the queue shows it: what it is (the content
// item's title and address, a comment's excerpt), its author; a decided
// one - what was done, by whom.
type queueRow struct {
	Target
	Title          string
	URL            string
	Excerpt        string
	TypeLabel      string
	AuthorName     string
	AuthorURL      string
	Gone           bool
	Decision       string
	ResolvedByName string
}

// decisionLabels - what a moderator did, in words (Target.Resolution).
var decisionLabels = map[string]string{
	ResolutionKept:      i18n.N("Kept"),
	ResolutionSpam:      i18n.N("Marked as spam"),
	ResolutionUnpublish: i18n.N("Unpublished"),
	ResolutionDeleted:   i18n.N("Deleted"),
}

// Queue renders GET /admin/content/complaints (?tab=resolved - the
// decided ones): the content items and comments complained about, the
// newest complaint first - what it is, its author, how many complaints
// and why, the visitors' notes, whether they hid it - and the decisions.
func (c *Controller) Queue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if _, ok := c.may(r, manifest.PermHandle); !ok {
		c.RenderForbidden(w, r)
		return
	}
	status := StatusNew
	if r.URL.Query().Get("tab") == "resolved" {
		status = StatusResolved
	}
	page := c.GetPageNumber(r)
	targets, total, err := c.repo.List(ctx, status, queuePerPage, (page-1)*queuePerPage)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "listing the complaints")
		return
	}
	data := c.AdminPageData(r)
	data["title"] = i18n.T(ctx, "Complaints")
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["rows"] = c.queueRows(ctx, targets)
	data["tab"] = status
	query := url.Values{}
	if status == StatusResolved {
		query.Set("tab", "resolved")
	}
	data["pagination"] = pagination.New(page, total, queuePerPage, manifest.AdminQueuePath, query)
	data["done"] = r.URL.Query().Get("done")
	data["error"] = r.URL.Query().Get("error") == "1"
	c.RenderOrFail(w, r, "admin/complaints.html", data)
}

// queueRows - targets with what they are about.
func (c *Controller) queueRows(ctx context.Context, targets []Target) []queueRow {
	labels := c.contentTypes.NodeTypeLabels(ctx)
	var userIDs []int
	for _, t := range targets {
		for _, id := range []int{t.AuthorID, t.ResolvedBy} {
			if id > 0 && !slices.Contains(userIDs, id) {
				userIDs = append(userIDs, id)
			}
		}
	}
	users := map[int]modulekit.User{}
	if len(userIDs) > 0 {
		var err error
		if users, err = c.users.ByIDs(ctx, userIDs); err != nil {
			modulekit.LogFallback(c.Logger(), err, "the complained items' authors (unnamed)")
		}
	}
	rows := make([]queueRow, 0, len(targets))
	for _, t := range targets {
		row := queueRow{Target: t}
		if a, ok := users[t.AuthorID]; ok {
			row.AuthorName, row.AuthorURL = a.Username, "/"+a.Slug
		}
		if label, ok := decisionLabels[t.Resolution]; ok {
			row.Decision = i18n.T(ctx, label)
		}
		if m, ok := users[t.ResolvedBy]; ok {
			row.ResolvedByName = m.Username
		}
		n, err := c.nodes.ByID(ctx, t.NodeID)
		switch {
		case err != nil:
			row.Gone = true
		case t.Kind == KindNode:
			row.Title, row.URL, row.TypeLabel = n.Title, n.URL(), labels[string(n.Type)]
		default:
			row.Title, row.URL = n.Title, n.URL()+"#comment-"+strconv.Itoa(t.ObjectID)
			if cm, err := c.comments.ByID(ctx, t.ObjectID); err == nil && cm != nil {
				row.Excerpt = modulekit.PlainPreview(cm.Content, 160)
			} else {
				row.Gone = true
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// Decide handles POST /admin/content/complaints/{id}/{action}: a
// moderator's decision (Service.Decide), then back to the queue.
func (c *Controller) Decide(w http.ResponseWriter, r *http.Request) {
	user, ok := c.may(r, manifest.PermHandle)
	if !ok {
		c.RenderForbidden(w, r)
		return
	}
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	action := r.PathValue("action")
	if err := c.service.Decide(r.Context(), user, id, action); err != nil {
		if errors.Is(err, modulekit.ErrEntityNotFound) {
			c.RenderNotFound(w, r)
			return
		}
		c.LogFailure(err, "deciding on a complaint", "target_id", id, "action", action)
		http.Redirect(w, r, manifest.AdminQueuePath+"?error=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, manifest.AdminQueuePath+"?done="+action, http.StatusSeeOther)
}

// Settings renders and saves GET/POST /admin/config/complaints: the
// content types whose items may be reported, the reasons, how many
// complaints hide an item.
func (c *Controller) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if _, ok := c.may(r, manifest.PermHandle); !ok {
		c.RenderForbidden(w, r)
		return
	}
	labels := c.contentTypes.NodeTypeLabels(ctx)
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			c.RenderBadForm(w, r)
			return
		}
		s := Settings{Comments: r.FormValue("comments") == "1", Reasons: strings.TrimSpace(r.FormValue("reasons"))}
		for _, typ := range r.Form["node_types"] {
			if _, ok := labels[typ]; ok && !slices.Contains(s.NodeTypes, typ) {
				s.NodeTypes = append(s.NodeTypes, typ)
			}
		}
		n, err := strconv.Atoi(r.FormValue("hide_after"))
		if err != nil || !slices.Contains(hideAfterChoices, n) {
			http.Redirect(w, r, manifest.AdminSettingsPath+"?error=1", http.StatusSeeOther)
			return
		}
		s.HideAfter = n
		if err := c.repo.SaveSettings(ctx, s); err != nil {
			c.LogFailure(err, "saving the complaints' settings")
			http.Redirect(w, r, manifest.AdminSettingsPath+"?error=1", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, manifest.AdminSettingsPath+"?saved=1", http.StatusSeeOther)
		return
	}
	settings, err := c.repo.Settings(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the complaints' settings")
		return
	}
	data := c.AdminPageData(r)
	data["title"] = i18n.T(ctx, "Complaints")
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["settings"] = settings
	data["types"] = modulekit.NodeTypesByLabel(labels)
	data["typeLabels"] = labels
	defaults := make([]string, len(DefaultReasons))
	for i, reason := range DefaultReasons {
		defaults[i] = i18n.T(ctx, reason)
	}
	data["reasonsPlaceholder"] = strings.Join(defaults, "\n")
	data["mailURL"] = modulekit.MailTemplatePath(MailNewComplaint)
	data["hideAfterChoices"] = hideAfterChoices
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error") == "1"
	c.RenderOrFail(w, r, "admin/complaints_settings.html", data)
}
