package feeds

import (
	"github.com/kosatka-cms/kosatka/modulekit"
)

// FeedController - the module's pages: the feeds at their paths
// (Fallback) and their admin (/admin/config/feeds).
type FeedController struct {
	*modulekit.BaseController
	feeds               *FeedService
	teasers             modulekit.TeaserBuilder
	siteInformationRepo modulekit.SiteInformationRepository
	contentTypeService  modulekit.ContentTypeService
	imageStyleService   modulekit.ImageStyleService
}

// NewFeedController creates the module's controller on base.
func NewFeedController(base *modulekit.BaseController, feeds *FeedService, teasers modulekit.TeaserBuilder, site modulekit.SiteInformationRepository, contentTypes modulekit.ContentTypeService, imageStyles modulekit.ImageStyleService) *FeedController {
	base.SetLogger(base.Logger().With("component", "feeds.handler"))
	return &FeedController{BaseController: base, feeds: feeds, teasers: teasers, siteInformationRepo: site, contentTypeService: contentTypes, imageStyleService: imageStyles}
}
