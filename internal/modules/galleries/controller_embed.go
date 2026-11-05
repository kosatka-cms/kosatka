package galleries

import (
	"net/http"
	"strconv"
)

// GalleriesEmbedHTML returns the node page's galleries block (lazy-loaded on
// scroll by block_node_galleries); ?previews=N - the previews of each.
func (c *GalleryController) GalleriesEmbedHTML(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodeIDStr := r.PathValue("id")
	nodeID, err := strconv.Atoi(nodeIDStr)
	if err != nil || nodeID <= 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	node, err := c.nodeService.ByID(ctx, nodeID)
	if err != nil {
		c.LogFailure(err, "loading the content", "nodeID", nodeID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Defense in depth for direct API calls: a disabled node yields an empty
	// body (like "no galleries"), since this is a lazy-load mount, not a page.
	if !c.nodeEnabled(ctx, node) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	}

	currentUser, _ := c.GetCurrentUser(r)
	if !c.AuthService().CanViewNode(ctx, node, currentUser) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	galleries, err := c.galleryService.ByNodeID(ctx, nodeID)
	if err != nil {
		c.Logger().Warn("galleries embed load failed", "error", err, "nodeID", nodeID)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	}

	hasAny := false
	for _, g := range galleries {
		if g.ItemsCount() > 0 {
			hasAny = true
			break
		}
	}
	if !hasAny {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return
	}

	// The block's "previews" setting: how many previews each gallery
	// shows (otherwise its grid's row, Gallery.PreviewLimit).
	if n, err := strconv.Atoi(r.URL.Query().Get("previews")); err == nil && n > 0 && n <= maxNodePreviews {
		for _, g := range galleries {
			g.NodePreviewCount = n
		}
	}

	data := map[string]any{
		"item": map[string]any{
			"Node":      node,
			"Galleries": galleries,
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=120")
	c.RenderOrFail(w, r, "node/galleries_embed.html", data)
}
