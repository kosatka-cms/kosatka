package galleries

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// Main types

// Galleries - a list of galleries (a node's, a page of a search).
type Galleries []*Gallery

// Gallery - a gallery of images attached to a node: its title, its
// previews' image styles, its place among the node's galleries and its
// items.
type Gallery struct {
	ID     int
	NodeID int
	// PreviewSmallStyle/PreviewMediumStyle - the image styles of the
	// grid previews and of the larger previews (image_styles.name).
	PreviewSmallStyle  string
	PreviewMediumStyle string
	Title              *string
	Description        *string
	Weight             int
	Status             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Node               *modulekit.Node
	Cover              *modulekit.File
	Items              GalleryItems

	// PreviewWidth/PreviewHeight - PreviewSmallStyle's size, read with
	// the gallery (0 - unknown); IsPortrait - the style is taller than
	// wide: the portrait grid, cells of its proportions.
	PreviewWidth  int
	PreviewHeight int
	IsPortrait    bool
	// NodePreviewCount - the previews on the content's page the block
	// asks for (0 - the default, PreviewLimit).
	NodePreviewCount int
}

// maxNodePreviews - the most previews the block may ask a gallery for.
const maxNodePreviews = 50

// Default previews on the content's page: a row of the grid (the
// portrait grid's cells are narrower).
const (
	defaultNodePreviews         = 5
	defaultPortraitNodePreviews = 6
)

// PreviewLimit - how many previews the gallery shows on its content's
// page: the block's count, else the default for its grid. With more
// images the last preview leads to the whole gallery.
func (g *Gallery) PreviewLimit() int {
	switch {
	case g.NodePreviewCount > 0:
		return g.NodePreviewCount
	case g.IsPortrait:
		return defaultPortraitNodePreviews
	}
	return defaultNodePreviews
}

// GalleryItems - a gallery's images, in their order.
type GalleryItems []*GalleryItem

// GalleryItem - an image of a gallery: its file, caption and place in the
// gallery, with its previews in the gallery's styles.
type GalleryItem struct {
	ID            int
	GalleryID     int
	FileID        int
	Title         *string
	Description   *string
	Weight        int
	IsCover       bool
	CreatedAt     time.Time
	Gallery       *Gallery
	File          *modulekit.File
	PreviewSmall  *modulekit.File
	PreviewMedium *modulekit.File
	// Picture - the grid's preview (PreviewSmall) for the template
	// function img: with its style's other widths and its size.
	Picture modulekit.Picture
}

// Filter types

// GalleriesFilter - which galleries Find returns: of a node (0 - any),
// shown or hidden (Status nil - both), a page (Offset, Limit 0 - all).
type GalleriesFilter struct {
	NodeID int
	Status *bool
	Offset int
	Limit  int
	// Query - galleries whose title has it, or (NodeIDs) of these nodes:
	// a search ("" - any).
	Query   string
	NodeIDs []int
}

// NodeGalleryCounts - a node's galleries and their images (the admin's
// galleries overview).
type NodeGalleryCounts struct {
	Galleries int
	Images    int
}

// Input types

// GalleryCreateInput - a new gallery of a node (empty styles - the
// defaults; GalleriesRepository.Create).
type GalleryCreateInput struct {
	NodeID             int
	PreviewSmallStyle  string
	PreviewMediumStyle string
	Title              *string
	Description        *string
	Weight             int
	Status             bool
}

// GalleryUpdateInput - changes to a gallery (nil - unchanged;
// GalleriesRepository.Update).
type GalleryUpdateInput struct {
	PreviewSmallStyle  *string
	PreviewMediumStyle *string
	Title              *string
	Description        *string
	Weight             *int
	Status             *bool
}

// GalleryItemCreateInput - a new image of a gallery: its file and caption
// (GalleriesRepository.AddItem).
type GalleryItemCreateInput struct {
	GalleryID   int
	FileID      int
	Title       *string
	Description *string
	Weight      int
	IsCover     bool
}

// GalleryItemUpdateInput - changes to a gallery's image (nil - unchanged;
// GalleriesRepository.UpdateItem).
type GalleryItemUpdateInput struct {
	Title       *string
	Description *string
	Weight      *int
	IsCover     *bool
}

// GalleryItemReorder - an item's new weight in its gallery
// (GalleriesRepository.ReorderItems).
type GalleryItemReorder struct {
	ID     int
	Weight int
}

// Repository interface

// GalleriesRepository - the store of galleries and their items.
type GalleriesRepository interface {
	Find(ctx context.Context, filter GalleriesFilter) (Galleries, error)
	TotalCount(ctx context.Context, filter GalleriesFilter) (int, error)
	ByID(ctx context.Context, id int) (*Gallery, error)
	ByNodeID(ctx context.Context, nodeID int) (Galleries, error)
	// SummaryByNodeID returns galleries for a node without loading item files/previews.
	SummaryByNodeID(ctx context.Context, nodeID int) (Galleries, error)
	// CountsByNodeIDs - the galleries and images of the nodes among ids
	// that have any.
	CountsByNodeIDs(ctx context.Context, ids []int) (map[int]NodeGalleryCounts, error)
	Create(ctx context.Context, input GalleryCreateInput) (*Gallery, error)
	Update(ctx context.Context, id int, input GalleryUpdateInput) (*Gallery, error)
	Delete(ctx context.Context, id int) error
	DeleteWithItems(ctx context.Context, id int) error
	DeleteByNodeIDWithItems(ctx context.Context, nodeID int) error
	AddItem(ctx context.Context, input GalleryItemCreateInput) (*GalleryItem, error)
	UpdateItem(ctx context.Context, id int, input GalleryItemUpdateInput) (*GalleryItem, error)
	RemoveItem(ctx context.Context, id int) error
	GetItemsByGalleryID(ctx context.Context, galleryID int) (GalleryItems, error)
	GetItemByID(ctx context.Context, itemID int) (*GalleryItem, error)
	ReorderItems(ctx context.Context, items []GalleryItemReorder) error
	SetCover(ctx context.Context, galleryID, itemID int) error
	GetCover(ctx context.Context, galleryID int) (*GalleryItem, error)
}

// Helper methods for Galleries

// Len - the number of galleries.
func (galleries Galleries) Len() int {
	return len(galleries)
}

// IsEmpty reports whether the list has no galleries.
func (galleries Galleries) IsEmpty() bool {
	return len(galleries) == 0
}

// FilterByStatus - the galleries whose Status is status (true - shown).
func (galleries Galleries) FilterByStatus(status bool) Galleries {
	var filtered Galleries
	for _, gallery := range galleries {
		if gallery.Status == status {
			filtered = append(filtered, gallery)
		}
	}
	return filtered
}

// Helper methods for Gallery

// GetTitle - the gallery's title ("" - none).
func (g *Gallery) GetTitle() string {
	if g.Title != nil {
		return *g.Title
	}
	return ""
}

// GetDescription - the gallery's description ("" - none).
func (g *Gallery) GetDescription() string {
	if g.Description != nil {
		return *g.Description
	}
	return ""
}

// HasItems reports whether the gallery has images (of those loaded).
func (g *Gallery) HasItems() bool {
	return len(g.Items) > 0
}

// ItemsCount - the number of the gallery's loaded images.
func (g *Gallery) ItemsCount() int {
	return len(g.Items)
}

// GetFirstItem - the gallery's first image (nil - none).
func (g *Gallery) GetFirstItem() *GalleryItem {
	if len(g.Items) > 0 {
		return g.Items[0]
	}
	return nil
}

// GetCoverImage - the file of the gallery's cover: the item marked as
// cover, else the first image (nil - none).
func (g *Gallery) GetCoverImage() *modulekit.File {
	// Prefer the item marked as cover.
	for _, item := range g.Items {
		if item.IsCover && item.File != nil {
			return item.File
		}
	}
	// Fallback: the first image.
	if len(g.Items) > 0 && g.Items[0].File != nil {
		return g.Items[0].File
	}
	return nil
}

// GetCoverItem - the gallery's cover item: the one marked as cover, else
// the first (nil - no items).
func (g *Gallery) GetCoverItem() *GalleryItem {
	for _, item := range g.Items {
		if item.IsCover {
			return item
		}
	}
	// Fallback: the first item.
	if len(g.Items) > 0 {
		return g.Items[0]
	}
	return nil
}

// GetPreviewItems returns first N items for preview
func (g *Gallery) GetPreviewItems(limit int) GalleryItems {
	if limit <= 0 || limit >= len(g.Items) {
		return g.Items
	}
	return g.Items[:limit]
}

// GetRandomItems returns N random items for preview
func (g *Gallery) GetRandomItems(limit int) GalleryItems {
	if limit <= 0 || limit >= len(g.Items) {
		return g.Items
	}
	shuffled := make(GalleryItems, len(g.Items))
	copy(shuffled, g.Items)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled[:limit]
}

// Helper methods for GalleryItems

// Len - the number of items.
func (items GalleryItems) Len() int {
	return len(items)
}

// IsEmpty reports whether the list has no items.
func (items GalleryItems) IsEmpty() bool {
	return len(items) == 0
}

// GetFiles - the items' files, skipping items without one.
func (items GalleryItems) GetFiles() []*modulekit.File {
	files := make([]*modulekit.File, 0, len(items))
	for _, item := range items {
		if item.File != nil {
			files = append(files, item.File)
		}
	}
	return files
}

// GetURIs - the URIs of the items' files, skipping items without one.
func (items GalleryItems) GetURIs() []string {
	uris := make([]string, 0, len(items))
	for _, item := range items {
		if item.File != nil {
			uris = append(uris, item.File.URI)
		}
	}
	return uris
}

// Helper methods for GalleryItem

// GetTitle - the image's title ("" - none).
func (item *GalleryItem) GetTitle() string {
	if item.Title != nil {
		return *item.Title
	}
	return ""
}

// GetDescription - the image's description ("" - none).
func (item *GalleryItem) GetDescription() string {
	if item.Description != nil {
		return *item.Description
	}
	return ""
}

// HasFile reports whether the item has its image file (found among the
// core's files).
func (item *GalleryItem) HasFile() bool {
	return item.File != nil
}

// GetFileURI - the URI of the item's file ("" - no file).
func (item *GalleryItem) GetFileURI() string {
	if item.File != nil {
		return item.File.URI
	}
	return ""
}

// GetCover - the file of the cover among the items: the one marked as
// cover, else the first image (nil - none).
func (items *GalleryItems) GetCover() *modulekit.File {
	for _, item := range *items {
		if item.IsCover && item.File != nil {
			return item.File
		}
	}

	// Fallback: the first image.
	if len(*items) > 0 && (*items)[0].File != nil {
		return (*items)[0].File
	}

	return nil
}

// CoverListStyle - the image style of a gallery's cover in a node's
// galleries list (/node/{id}/galleries).
const CoverListStyle = "gallery_cover"

// coverStyle - CoverListStyle among styles (nil - the site has none): the
// galleries list's covers take its proportions, a style taller than wide
// gives the portrait grid.
func coverStyle(styles modulekit.ImageStyleDefs) *modulekit.ImageStyleDef {
	for i := range styles {
		if styles[i].Name == CoverListStyle {
			return &styles[i]
		}
	}
	return nil
}

// CoverListURI - the gallery's cover in CoverListStyle ("" - no cover).
func (g *Gallery) CoverListURI() string {
	if g.Cover == nil || strings.TrimSpace(g.Cover.URI) == "" {
		return ""
	}
	rel := strings.TrimPrefix(strings.TrimSpace(g.Cover.URI), "/")
	return modulekit.StyledImage(modulekit.File{URI: rel}, CoverListStyle).URI
}
