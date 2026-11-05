package galleries

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/metadescription"
)

// pageSEOForNodeGalleriesList - the node's galleries list (/node/{id}/galleries).
// nodeTitle is the node's display title.
func pageSEOForNodeGalleriesList(ctx context.Context, node modulekit.Node, nodeTitle string, galleriesCount int, path string) modulekit.PageSEO {
	fullURL := fmt.Sprintf("%s%s", modulekit.SiteDomain(), path)
	title := i18n.T(ctx, "Galleries - @title", "@title", nodeTitle)
	var desc string
	if galleriesCount > 0 {
		desc = i18n.Plural(ctx, galleriesCount, "Image galleries for \"@title\": @count collection.", "Image galleries for \"@title\": @count collections.", "@title", nodeTitle)
	} else {
		desc = i18n.T(ctx, "Image galleries for \"@title\".", "@title", nodeTitle)
	}
	desc = metadescription.Generate(desc, modulekit.MaxMetaDesc)
	return modulekit.PageSEO{
		Title:       title,
		Description: desc,
		Canonical:   fullURL,
		OGURL:       fullURL,
		OGImage:     "",
	}
}

// pageSEOForGalleryView - the public gallery page (/galleries/{id}).
// nodeTitle is the node's display title.
func pageSEOForGalleryView(ctx context.Context, gallery Gallery, node *modulekit.Node, nodeTitle string, path string) modulekit.PageSEO {
	fullURL := fmt.Sprintf("%s%s", modulekit.SiteDomain(), path)
	gt := gallery.GetTitle()
	if gt == "" {
		gt = i18n.T(ctx, "Image gallery")
	}
	title := gt
	if node != nil {
		title = fmt.Sprintf("%s - %s", gt, nodeTitle)
	}
	desc := metadescription.Generate(gallery.GetDescription(), modulekit.MaxMetaDesc)
	if desc == "" && node != nil {
		desc = metadescription.Generate(i18n.T(ctx, "Gallery \"@gallery\" for \"@title\".", "@gallery", gt, "@title", nodeTitle), modulekit.MaxMetaDesc)
	} else if desc == "" {
		desc = metadescription.Generate(gt, modulekit.MaxMetaDesc)
	}
	ogImage := ""
	if cover := gallery.GetCoverImage(); cover != nil {
		if img := modulekit.OriginalImage(*cover); img.URI != "" {
			ogImage = modulekit.AbsoluteAssetURL(modulekit.SiteDomain(), img.URI)
		}
	}
	return modulekit.PageSEO{
		Title:       title,
		Description: desc,
		Canonical:   fullURL,
		OGURL:       fullURL,
		OGImage:     ogImage,
	}
}
