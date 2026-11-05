package galleries

import (
	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// coreRefs - the core's rows a gallery and its items reference: nodes and
// files by ID, the image styles by name (the repository loads them; none -
// left unset).
type coreRefs struct {
	nodes  map[int]modulekit.Node
	files  map[int]modulekit.File
	styles map[string]modulekit.ImageStyleDef
}

// ToGalleryItemsEntity converts generated gallery items to domain entities;
// an item loaded with its gallery gets its previews in the gallery's
// styles (else the original image).
func ToGalleryItemsEntity(items []*generated.GalleryItem, refs coreRefs) GalleryItems {
	result := make(GalleryItems, len(items))
	for i, item := range items {
		result[i] = ToGalleryItemEntity(item, refs)
	}
	return result
}

// ToGalleryEntity converts a generated gallery to its domain entity.
func ToGalleryEntity(g *generated.Gallery, refs coreRefs) *Gallery {
	if g == nil {
		return nil
	}
	gallery := toGallery(g, refs)
	if node, ok := refs.nodes[g.Nid]; ok {
		gallery.Node = &node
	}
	if len(g.Edges.Items) > 0 {
		gallery.Items = make(GalleryItems, len(g.Edges.Items))
		for i, item := range g.Edges.Items {
			gallery.Items[i] = toGalleryItemEntity(item, g.PreviewSmallStyle, g.PreviewMediumStyle, refs)
		}
		// Set Cover from items if exists
		for _, item := range gallery.Items {
			if item.IsCover && item.File != nil {
				gallery.Cover = item.File
				break
			}
		}
	}
	return gallery
}

// toGallery - the gallery's own fields, its preview size from its grid
// style.
func toGallery(g *generated.Gallery, refs coreRefs) *Gallery {
	gallery := &Gallery{
		ID:                 g.ID,
		NodeID:             g.Nid,
		PreviewSmallStyle:  g.PreviewSmallStyle,
		PreviewMediumStyle: g.PreviewMediumStyle,
		Weight:             g.Weight,
		Status:             g.Status,
		CreatedAt:          g.CreatedAt,
		UpdatedAt:          g.UpdatedAt,
	}
	if g.Title != "" {
		gallery.Title = &g.Title
	}
	if g.Description != "" {
		gallery.Description = &g.Description
	}
	if st, ok := refs.styles[g.PreviewSmallStyle]; ok {
		gallery.PreviewWidth, gallery.PreviewHeight = st.Width, st.Height
		// A style that keeps the aspect ratio (height 0) is not portrait.
		gallery.IsPortrait = st.Height > 0 && st.Height > st.Width
	}
	return gallery
}

// ToGalleriesEntity converts generated galleries to domain entities
// (ToGalleryEntity each).
func ToGalleriesEntity(galleries []*generated.Gallery, refs coreRefs) Galleries {
	result := make(Galleries, len(galleries))
	for i, g := range galleries {
		result[i] = ToGalleryEntity(g, refs)
	}
	return result
}

// ToGalleryItemEntity converts a generated gallery item to its domain entity.
func ToGalleryItemEntity(item *generated.GalleryItem, refs coreRefs) *GalleryItem {
	small, medium := "", ""
	if item != nil && item.Edges.Gallery != nil {
		small, medium = item.Edges.Gallery.PreviewSmallStyle, item.Edges.Gallery.PreviewMediumStyle
	}
	return toGalleryItemEntity(item, small, medium, refs)
}

// toGalleryItemEntity - the item with its previews in the gallery's styles
// small and medium ("" - the original image).
func toGalleryItemEntity(item *generated.GalleryItem, small, medium string, refs coreRefs) *GalleryItem {
	if item == nil {
		return nil
	}

	galleryItem := &GalleryItem{
		ID:        item.ID,
		GalleryID: item.GalleryID,
		FileID:    item.Fid,
		Weight:    item.Weight,
		IsCover:   item.IsCover,
		CreatedAt: item.CreatedAt,
	}

	if item.Title != "" {
		galleryItem.Title = &item.Title
	}

	if item.Description != "" {
		galleryItem.Description = &item.Description
	}

	if item.Edges.Gallery != nil {
		galleryItem.Gallery = toGallery(item.Edges.Gallery, refs)
	}

	if file, ok := refs.files[item.Fid]; ok {
		original := modulekit.OriginalImage(file)
		filePreviewSmall, filePreviewMedium := original, original
		galleryItem.Picture = modulekit.OriginalPicture(file)
		if small != "" && medium != "" {
			filePreviewSmall = modulekit.StyledImage(file, small)
			filePreviewMedium = modulekit.StyledImage(file, medium)
			galleryItem.Picture = modulekit.StyledPicture(file, small)
		}
		file = original

		galleryItem.File = &file
		galleryItem.PreviewSmall = &filePreviewSmall
		galleryItem.PreviewMedium = &filePreviewMedium
	}

	return galleryItem
}
