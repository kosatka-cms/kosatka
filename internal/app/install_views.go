package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/viewquery"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// publishInstalledViews publishes every View display a new site starts
// with - the starter's, the demo's, the modules' (their Default displays
// have no version of their own): visitors see the published versions, and
// the administrator's changes stay drafts until published, as for a
// display published in the admin.
func publishInstalledViews(ctx context.Context, client *generated.Client, logger *slog.Logger) error {
	svc := views.NewService(database.NewViewRepository(client), viewquery.New(client), database.NewContentTypeRepository(client),
		database.NewUnitOfWork(client), nil, logger)
	displays, err := svc.Views(ctx)
	if err != nil {
		return err
	}
	for _, d := range displays {
		if d.IsDefault || d.PublishedAt != nil {
			continue
		}
		if _, err := svc.PublishDisplay(ctx, d.ID); err != nil {
			return fmt.Errorf("publishing the View display %q: %w", d.MachineName, err)
		}
	}
	return nil
}
