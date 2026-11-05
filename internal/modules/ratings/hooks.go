package ratings

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/modules/ratings/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// typeForm - what the content type form's ratings section
// (content_type_settings_ratings) shows: whether the type is rated and
// its votes so far.
type typeForm struct {
	Rated bool
	Votes int
	Nodes int
}

// nodePage - what the node_rating block reads as .moduleNode.ratings: the
// node's average and count (the visitor's part loads apart, the page may
// be cached).
type nodePage struct {
	NodeID  int
	Summary Summary
}

// RegisterHooks adds the module's section of the content type form and
// its data of node pages (wiring, at startup).
func RegisterHooks(s *Service, logger *slog.Logger) {
	modulekit.AddContentTypeSettings(typeSettingsHook(s))
	modulekit.AddNodePage(manifest.Name, nodePageHook(s, logger))
}

// typeSettingsHook - the content type form's ratings section: whether
// content of the type is rated (the same switch as the settings page's).
func typeSettingsHook(s *Service) modulekit.ContentTypeSettings {
	return modulekit.ContentTypeSettings{
		Module: manifest.Name,
		Load: func(ctx context.Context, ct modulekit.ContentTypeDef) (any, error) {
			rated, err := s.Rated(ctx, ct.MachineName)
			if err != nil {
				return nil, err
			}
			votes, nodes, err := s.TypeTotals(ctx, ct.MachineName)
			if err != nil {
				return nil, err
			}
			return typeForm{Rated: rated, Votes: votes, Nodes: nodes}, nil
		},
		Save: func(ctx context.Context, ct modulekit.ContentTypeDef, r *http.Request) error {
			return s.SetTypeRated(ctx, ct.MachineName, r.FormValue("ratings_rated") == "1")
		},
	}
}

// nodePageHook - a rated node's average and count for its page. A failed
// load leaves the block out and is logged (the page itself renders).
func nodePageHook(s *Service, logger *slog.Logger) func(ctx context.Context, node modulekit.Node) modulekit.NodePageParts {
	return func(ctx context.Context, node modulekit.Node) modulekit.NodePageParts {
		page, ok, err := nodeRating(ctx, s, node)
		if err != nil {
			modulekit.LogFallback(logger, err, "ratings: the node's rating (the block left out)", "nodeID", node.ID)
		}
		if err != nil || !ok {
			return modulekit.NodePageParts{}
		}
		return modulekit.NodePageParts{Data: page}
	}
}

// nodeRating - node's rating for its page; false - its type isn't rated.
func nodeRating(ctx context.Context, s *Service, node modulekit.Node) (nodePage, bool, error) {
	rated, err := s.Rated(ctx, string(node.Type))
	if err != nil || !rated {
		return nodePage{}, false, err
	}
	sums, err := s.Summaries(ctx, []int{node.ID}, lookFull)
	if err != nil {
		return nodePage{}, false, err
	}
	sum, ok := sums[node.ID]
	if !ok {
		st, err := s.Settings(ctx)
		if err != nil {
			return nodePage{}, false, err
		}
		sum = Summary{Scale: st.Scale, Look: lookFull}
	}
	return nodePage{NodeID: node.ID, Summary: sum}, true, nil
}
