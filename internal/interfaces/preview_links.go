package interfaces

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

type previewKey struct{}

// isPreview - whether ctx's page is shown through a preview link (Preview).
func isPreview(ctx context.Context) bool {
	p, _ := ctx.Value(previewKey{}).(bool)
	return p
}

// Preview handles GET /preview/{token}: the content item a preview link
// shows (service.PreviewLinks), its page as it would be, to whoever has
// the link - published or not, whatever its view roles - while the link
// works; not indexed, kept by no cache. An unknown or expired link, an
// item in the trash or of a module's type - not found.
func (c *DynamicContentController) Preview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if c.previewLinks == nil {
		c.RenderNotFound(w, r)
		return
	}
	nodeID, ok, err := c.previewLinks.Resolve(ctx, r.PathValue("token"))
	if err != nil {
		c.LogFailure(err, "resolving a preview link")
		httputil.InternalServerError(w)
		return
	}
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	node, err := c.nodeRepo.ByID(entity.WithAllNodeTypes(ctx), nodeID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading a preview link's node", "nodeID", nodeID)
		return
	}
	if node.InTrash() || slices.Contains(entity.ModuleNodeTypes(), node.Type.ToString()) {
		c.RenderNotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	ctx = context.WithValue(middleware.WithNode(ctx, node), previewKey{}, true)
	c.Detail(w, r.WithContext(ctx))
}

// PreviewLinkAction handles POST /node/{id}/preview-link (who may edit the
// node): action=create - a new link (an earlier one gone), JSON {"url",
// "expires" (RFC 3339)}; action=revoke - none left, JSON {}.
func (c *DynamicContentController) PreviewLinkAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, ok := middleware.GetNode(r)
	if !ok || c.previewLinks == nil {
		httputil.SendJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	switch r.FormValue("action") {
	case "create":
		user, _ := currentNodeUser(c.BaseController, r)
		by := 0
		if user != nil {
			by = user.ID
		}
		token, expires, err := c.previewLinks.Create(ctx, node.ID, by)
		if err != nil {
			c.LogFailure(err, "making a preview link", "nodeID", node.ID)
			httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{"error": i18n.T(ctx, "The link could not be made.")})
			return
		}
		httputil.SendJSON(w, http.StatusOK, map[string]any{
			"url":     publicBaseURL(r) + service.PreviewLinkPath + token,
			"expires": expires.UTC().Format(time.RFC3339),
		})
	case "revoke":
		if err := c.previewLinks.Revoke(ctx, node.ID); err != nil {
			c.LogFailure(err, "revoking a preview link", "nodeID", node.ID)
			httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{"error": i18n.T(ctx, "The link could not be revoked.")})
			return
		}
		httputil.SendJSON(w, http.StatusOK, map[string]any{})
	default:
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{"error": "unknown action"})
	}
}
