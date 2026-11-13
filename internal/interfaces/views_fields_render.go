package interfaces

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

// contentListSources - what views.BuildContentList reads to build a
// content View's list (its page, its preview, a contextual fragment).
func (c *DynamicContentController) contentListSources() views.ContentListSources {
	return views.ContentListSources{Types: c.contentTypeRepo, Terms: c.taxonomyService,
		Files: c.fileRepo, Comments: c.commentsService, Users: c.userService, Logger: c.logger}
}

// resolveCarouselLinkURLs - views.CarouselLinkURLs; on failure the slides
// go without their links.
func (c *DynamicContentController) resolveCarouselLinkURLs(ctx context.Context, fieldStorageID int, nodes entity.Nodes) map[int]string {
	urls, err := views.CarouselLinkURLs(ctx, c.contentTypeRepo, fieldStorageID, nodes)
	if err != nil {
		c.LogFailure(err, "error resolving carousel link field values", "fieldStorageID", fieldStorageID)
	}
	return urls
}
