package interfaces

import (
	"context"
	"net/http"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// NodeController handles node deletion, publishing and unpublishing.
type NodeController struct {
	*BaseController
	nodeService service.NodesService
	nodeRepo    entity.NodesRepository
	revisions   *service.NodeRevisionService
	deleter     *service.NodeDeleteService
}

// NodeDeps - what NewNodeController needs besides the BaseDeps.
type NodeDeps struct {
	NodeService service.NodesService
	// Revisions - the nodes' history: publishing recorded, its pages.
	Revisions *service.NodeRevisionService
	// Deleter - whether a node's deletion puts it in the trash (the
	// confirmation's wording).
	Deleter *service.NodeDeleteService
}

// NewNodeController creates a NodeController.
func NewNodeController(deps BaseDeps, d NodeDeps) *NodeController {
	base := NewBaseController(deps)
	base.logger = deps.Logger.With("component", "node.controller")

	return &NodeController{
		BaseController: base,
		nodeService:    d.NodeService,
		nodeRepo:       deps.Nodes,
		revisions:      d.Revisions,
		deleter:        d.Deleter,
	}
}

// DeleteConfirm displays the delete confirmation page.
func (c *NodeController) DeleteConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	nodeIDStr := r.PathValue("id")
	nodeID, err := strconv.Atoi(nodeIDStr)
	if err != nil {
		c.logger.Warn("invalid node ID", "id", nodeIDStr)
		c.RenderNotFound(w, r)
		return
	}

	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil {
		c.logger.Warn("user not authenticated")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	node, err := c.nodeService.ByID(ctx, nodeID)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.RenderNotFound(w, r)
			return
		}
		c.LogFailure(err, "error getting node by ID",
			"nodeID", nodeID,
		)
		httputil.InternalServerError(w)
		return
	}

	if !c.authService.CanDeleteNode(ctx, node, user) {
		c.logger.Warn("user does not have permission to delete this node",
			"userID", user.ID,
			"nodeID", nodeID,
		)
		c.RenderForbidden(w, r)
		return
	}

	data := c.GetCommonData(r)
	data["node"] = node
	// toTrash - the node goes to the trash (a content type's), for
	// trashDays days (0 - until emptied); else deleted for good.
	data["toTrash"] = c.deleter.Trashes(*node)
	data["trashDays"] = c.deleter.TrashDays()
	data["title"] = i18n.T(ctx, "Confirm deletion")
	c.setNodeSubpageTrail(ctx, data, *node, i18n.T(ctx, "Deletion"))

	c.RenderOrFail(w, r, "node/delete_confirm.html", data)
}

// Unpublish unpublishes the node (sets status to 0).
func (c *NodeController) Unpublish(w http.ResponseWriter, r *http.Request) {
	c.setPublished(w, r, false)
}

// Publish publishes the node (sets status to 1).
func (c *NodeController) Publish(w http.ResponseWriter, r *http.Request) {
	c.setPublished(w, r, true)
}

// setPublished publishes or unpublishes the node of POST
// /node/{id}/publish|unpublish (the route checks the right to publish;
// the user must also be allowed to edit the node) and goes back to its
// page.
func (c *NodeController) setPublished(w http.ResponseWriter, r *http.Request, publish bool) {
	ctx := r.Context()
	nodeID, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	node, err := c.nodeRepo.ByID(ctx, nodeID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error getting node by ID", "nodeID", nodeID)
		return
	}
	// A module publishing its type's items (an editorial workflow): its
	// transitions do, not this.
	if !c.authService.CanEditNode(ctx, node, user) || service.PublishingManaged(ctx, string(node.Type)) {
		c.RenderForbidden(w, r)
		return
	}
	set, action := c.nodeService.Unpublish, "unpublishing"
	if publish {
		set, action = c.nodeService.Publish, "publishing"
	}
	change := func(ctx context.Context) error { return set(ctx, nodeID) }
	if c.revisions != nil {
		change = func(ctx context.Context) error {
			return c.revisions.Track(ctx, nodeID, user, func(ctx context.Context) error { return set(ctx, nodeID) })
		}
	}
	if err := change(ctx); err != nil {
		c.LogFailure(err, "error "+action+" node", "nodeID", nodeID)
		httputil.InternalServerError(w)
		return
	}
	http.Redirect(w, r, node.URL(), http.StatusSeeOther)
}
