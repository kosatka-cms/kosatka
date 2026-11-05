package interfaces

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// RevisionsPerPage - the history's page size.
const RevisionsPerPage = 25

// MayViewRevisions - user may see the history of node: "view content
// revisions" and editing the node.
func (c *BaseController) MayViewRevisions(ctx context.Context, node *entity.Node, user *entity.User) bool {
	return user != nil && c.authService.HasPermission(ctx, user, service.PermViewRevisions) && c.authService.CanEditNode(ctx, node, user)
}

// mayRevert - user may bring back a revision of node: "revert content
// revisions" and editing the node.
func (c *BaseController) mayRevert(ctx context.Context, node *entity.Node, user *entity.User) bool {
	return user != nil && c.authService.HasPermission(ctx, user, service.PermRevertRevisions) && c.authService.CanEditNode(ctx, node, user)
}

// revisionsNode - the node of a history page (the route's edit access
// put it in the request) and its viewer, when they may see its history;
// otherwise the answer is written and ok is false.
func (c *NodeController) revisionsNode(w http.ResponseWriter, r *http.Request) (*entity.Node, *entity.User, bool) {
	node, ok := middleware.GetNode(r)
	if !ok || c.revisions == nil {
		c.RenderNotFound(w, r)
		return nil, nil, false
	}
	user, _ := currentNodeUser(c.BaseController, r)
	if !c.MayViewRevisions(r.Context(), node, user) {
		c.RenderForbidden(w, r)
		return nil, nil, false
	}
	return node, user, true
}

// revisionsPageData - what every history page shows: the admin's page
// under "Content", its trail through the node and the history.
func (c *NodeController) revisionsPageData(r *http.Request, node *entity.Node, title string, trail ...entity.BreadcrumbItem) map[string]any {
	ctx := r.Context()
	data := c.AdminPageData(r)
	if menu, ok := data["adminMenu"].([]entity.AdminMenuSection); ok {
		if section, item, ok := entity.AdminMenuCurrent(menu, "/admin/content"); ok {
			data["adminSection"], data["adminItem"] = section, item
		}
	}
	data["adminTrail"] = append([]entity.BreadcrumbItem{{URL: node.URL(), Label: node.Title}}, trail...)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title+": "+node.Title)}
	data["node"] = node
	if def, ok, err := c.revisions.Type(ctx, node.Type.ToString()); err != nil {
		c.LogFailure(err, "error loading the node's type for its history", "nodeID", node.ID)
	} else if ok {
		data["contentType"] = def
	}
	return data
}

// Revisions renders GET /node/{id}/revisions: the node's history, newest
// first, a page at a time.
func (c *NodeController) Revisions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, user, ok := c.revisionsNode(w, r)
	if !ok {
		return
	}
	page := c.GetPageNumber(r)
	rows, total, err := c.revisions.List(ctx, *node, (page-1)*RevisionsPerPage, RevisionsPerPage)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the node's history", "nodeID", node.ID)
		return
	}
	data := c.revisionsPageData(r, node, i18n.T(ctx, "History of changes"))
	data["revisions"] = rows
	data["totalCount"] = total
	data["pagination"] = pagination.New(page, total, RevisionsPerPage, fmt.Sprintf("/node/%d/revisions", node.ID), r.URL.Query())
	data["canRevert"] = c.mayRevert(ctx, node, user)
	data["reverted"] = r.URL.Query().Get("reverted") == "1"
	data["dropped"], _ = strconv.Atoi(r.URL.Query().Get("dropped"))
	data["error"] = r.URL.Query().Get("error")
	c.RenderOrFail(w, r, "admin/revisions/list.html", data)
}

// RevisionsCompare renders GET /node/{id}/revisions/compare?from=&to=:
// two revisions side by side, the older one first.
func (c *NodeController) RevisionsCompare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, user, ok := c.revisionsNode(w, r)
	if !ok {
		return
	}
	from, errFrom := strconv.Atoi(r.URL.Query().Get("from"))
	to, errTo := strconv.Atoi(r.URL.Query().Get("to"))
	historyURL := fmt.Sprintf("/node/%d/revisions", node.ID)
	if errFrom != nil || errTo != nil || from == to {
		http.Redirect(w, r, historyURL+"?error=compare", http.StatusSeeOther)
		return
	}
	if from > to {
		from, to = to, from
	}
	cmp, err := c.revisions.Compare(ctx, *node, from, to)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "comparing the node's revisions", "nodeID", node.ID, "from", from, "to", to)
		return
	}
	data := c.revisionsPageData(r, node, i18n.T(ctx, "Comparing versions"), entity.BreadcrumbItem{URL: historyURL, Label: i18n.T(ctx, "History of changes")})
	data["comparison"] = cmp
	data["canRevert"] = c.mayRevert(ctx, node, user)
	c.RenderOrFail(w, r, "admin/revisions/compare.html", data)
}

// RevisionRevert handles POST /node/{id}/revisions/{rev}/revert: the
// revision brought back as a new one; to the history.
func (c *NodeController) RevisionRevert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, user, ok := c.revisionsNode(w, r)
	if !ok {
		return
	}
	if !c.mayRevert(ctx, node, user) {
		c.RenderForbidden(w, r)
		return
	}
	revID, err := strconv.Atoi(r.PathValue("rev"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	historyURL := fmt.Sprintf("/node/%d/revisions", node.ID)
	dropped, err := c.revisions.Revert(ctx, node.ID, revID, user)
	if err != nil {
		if service.IsNotFound(err) {
			c.RenderNotFound(w, r)
			return
		}
		c.LogFailure(err, "error reverting the node", "nodeID", node.ID, "revision", revID)
		http.Redirect(w, r, historyURL+"?error=revert", http.StatusSeeOther)
		return
	}
	c.logger.Info("node reverted", "nodeID", node.ID, "revision", revID, "dropped", dropped)
	http.Redirect(w, r, fmt.Sprintf("%s?reverted=1&dropped=%d", historyURL, dropped), http.StatusSeeOther)
}
