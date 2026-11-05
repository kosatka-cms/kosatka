package interfaces

import (
	"context"
	"fmt"
)

// tagTermsUpdater is the subset of service.TaxonomyService that
// saveContentTaxonomy needs.
type tagTermsUpdater interface {
	UpdateNodeTerms(ctx context.Context, nodeID int, termIDs []int) error
	FieldTermIDsForNode(ctx context.Context, nodeID int) ([]int, error)
}

// saveContentTaxonomy writes the node's taxonomy index: every taxonomy
// field value. The field values failing to load, the index is left as it
// is: written without them it would lose the node's terms.
func saveContentTaxonomy(ctx context.Context, taxonomyService tagTermsUpdater, nID int) error {
	// UpdateNodeTerms replaces the node's whole index, so it must include every
	// taxonomy field value too, or field-based terms would vanish from term
	// pages, Views and filters. Field values are already saved at this point.
	fieldTermIDs, err := taxonomyService.FieldTermIDsForNode(ctx, nID)
	if err != nil {
		return fmt.Errorf("loading the node's field terms (node %v): %w", nID, err)
	}
	if err := taxonomyService.UpdateNodeTerms(ctx, nID, fieldTermIDs); err != nil {
		return fmt.Errorf("updating the node's terms (node %v): %w", nID, err)
	}
	return nil
}
