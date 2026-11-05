package videoembed

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Wire - the module's text filter (the videos in texts, the editor's
// button) and ways of showing a link field (its videos), the background job asking the services about new links, the
// settings page and the editor's check of a link; uninstalling removes
// the previews' copies.
func Wire(d modulekit.Deps) modulekit.Parts {
	logger := d.Logger.With("component", "video_embed")
	service := NewService(NewRepo(d.DB), NewFetcher(), logger)
	filter := NewFilter(service, d.Templates, logger)
	modulekit.AddTextFilter(filter.TextFilter())
	for _, f := range filter.FieldFormatters() {
		modulekit.AddFieldFormatter(f)
	}
	modulekit.AddJob(manifest.Name, modulekit.Job{Name: "video_embed_fetch", Every: time.Minute, Run: service.FetchPending})
	c := NewController(modulekit.NewBaseController(d.Base), service)
	return modulekit.Parts{
		Routes: func(r modulekit.Routes) { RegisterRoutes(r.Mux, r.Store, c, r.AdminOnly) },
		Uninstall: func(ctx context.Context) error {
			if err := modulekit.PublicFiles().DeleteDir(ctx, manifest.Name); err != nil {
				return fmt.Errorf("removing the video previews: %w", err)
			}
			return nil
		},
	}
}
