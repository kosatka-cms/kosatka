package migrations

import (
	"context"
	"fmt"
)

func init() {
	RegisterUpdate(CoreSet, Update{Version: "20261108233000", Description: "node_page_references", Run: keepNodePageReferences})
}

// keepNodePageReferences - a content item's page shows its reference
// fields now (their content, the public only), as a block's always did;
// it left them out before, Views attached that content instead. A
// reference field of a content type (not of a field set: a block's
// items) shown on the page loses "Show on the page", so a site looks as
// it did and doesn't list the content twice - beside its View. A site
// wanting the field on the page ticks it again.
func keepNodePageReferences(ctx context.Context, env Env) error {
	if _, err := env.DB.ExecContext(ctx, env.Dialect.Rebind(`UPDATE field_instances SET show_on_detail = ?
WHERE show_on_detail = ?
AND field_storage_id IN (SELECT id FROM field_storages WHERE field_type = ?)
AND content_type_id IN (SELECT id FROM content_types WHERE embedded = ?)`), false, true, "reference", false); err != nil {
		return fmt.Errorf("keeping the reference fields off the content pages: %w", err)
	}
	return nil
}
