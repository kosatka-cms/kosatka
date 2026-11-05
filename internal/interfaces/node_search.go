package interfaces

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// NodeSearchController serves GET /api/nodes/search - a generic title search
// across content types (see entity.NodesRepository.SearchByTitle), used by
// "link to a node" pickers in admin widgets.
type NodeSearchController struct {
	nodeRepo entity.NodesRepository
	logger   *slog.Logger
}

// NewNodeSearchController creates a NodeSearchController on nodeRepo.
func NewNodeSearchController(nodeRepo entity.NodesRepository, baseLogger *slog.Logger) *NodeSearchController {
	return &NodeSearchController{
		nodeRepo: nodeRepo,
		logger:   baseLogger.With("component", "node_search"),
	}
}

// nodeSearchResult - the JSON shape returned per match: enough for a
// picker to show a human-readable label and store the chosen ID.
type nodeSearchResult struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	Slug  string `json:"slug"`
	// URL - the node's public address (entity.Node.URL).
	URL string `json:"url"`
	// Match - the short text field value that matched when Title didn't
	// (e.g. a localized title), for the picker's label.
	Match string `json:"match,omitempty"`
}

// Search handles GET /api/nodes/search?q=... (signed-in users only, see
// RegisterNodeSearchRoutes).
func (c *NodeSearchController) Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := cutQuery(r.URL.Query().Get("q"))
	if query == "" {
		httputil.SendJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Missing search query parameter 'q'",
		})
		return
	}

	var types []string
	for _, t := range strings.Split(r.URL.Query().Get("types"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			types = append(types, t)
		}
	}
	hits, err := c.nodeRepo.SearchByTitle(ctx, query, types, 20)
	if err != nil {
		logFailure(c.logger, err, "error searching nodes", "query", query)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to search nodes",
		})
		return
	}

	results := make([]nodeSearchResult, len(hits))
	for i, h := range hits {
		results[i] = nodeSearchResult{
			ID:    h.Node.ID,
			Title: h.Node.Title,
			Type:  string(h.Node.Type),
			Slug:  h.Node.Slug,
			URL:   h.Node.URL(),
			Match: h.Match,
		}
	}

	httputil.SendJSON(w, http.StatusOK, map[string]any{
		"nodes": results,
	})
}
