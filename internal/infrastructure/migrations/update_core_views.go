package migrations

import (
	"context"
	"fmt"
)

func init() {
	RegisterUpdate(CoreSet, Update{Version: "20261111120000", Description: "views_published_everywhere", Run: keepLiveViewsLive})
}

// keepLiveViewsLive - visitors see a View display's published version now
// wherever it shows (blocks, embeds, the front page, term pages, profile
// tabs, feeds); only a content View's page did before, the rest showed
// the live version. A display published once whose published version
// nothing showed - not a content page display, or the front page's View
// - loses it, so visitors keep seeing what they saw (its live version)
// rather than an old one; it is published again in the admin.
func keepLiveViewsLive(ctx context.Context, env Env) error {
	if _, err := env.DB.ExecContext(ctx, env.Dialect.Rebind(`UPDATE view_displays SET published_snapshot = NULL, published_at = NULL
WHERE published_at IS NOT NULL
AND (display_type <> ? OR base_entity = ? OR machine_name IN (SELECT front_feed_view FROM site_information))`), "page", "comment"); err != nil {
		return fmt.Errorf("unpublishing the Views whose published versions showed nowhere: %w", err)
	}
	return nil
}
