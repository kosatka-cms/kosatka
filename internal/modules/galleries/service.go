package galleries

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/schema"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Column limits of galleries/gallery_items - checked up front so an
// over-long value is a clear 400, not a database error (the session
// runs in STRICT_TRANS_TABLES, see config.DBConfig.SQLMode).
const galleryTitleMaxLen = 255

// ErrGalleryFieldTooLong - a gallery/item title exceeds its column;
// wrapped with the field name and limit.
var ErrGalleryFieldTooLong = errors.New("gallery field too long")

func checkGalleryLen(field string, v *string, maxLen int) error {
	if v != nil && utf8.RuneCountInString(*v) > maxLen {
		return fmt.Errorf("%w: %s (max %d)", ErrGalleryFieldTooLong, field, maxLen)
	}
	return nil
}

// GalleryService - the galleries of nodes and their images, the input
// checked (title lengths, IDs, preview styles) before the store.
type GalleryService interface {
	Find(ctx context.Context, filter GalleriesFilter) (Galleries, error)
	TotalCount(ctx context.Context, filter GalleriesFilter) (int, error)
	ByID(ctx context.Context, id int) (*Gallery, error)
	ByNodeID(ctx context.Context, nodeID int) (Galleries, error)
	SummaryByNodeID(ctx context.Context, nodeID int) (Galleries, error)
	CountsByNodeIDs(ctx context.Context, ids []int) (map[int]NodeGalleryCounts, error)
	Create(ctx context.Context, input GalleryCreateInput) (*Gallery, error)
	Update(ctx context.Context, id int, input GalleryUpdateInput) (*Gallery, error)
	Delete(ctx context.Context, id int) error
	DeleteWithItems(ctx context.Context, id int) error
	DeleteByNodeIDWithItems(ctx context.Context, nodeID int) error
	AddItem(ctx context.Context, input GalleryItemCreateInput) (*GalleryItem, error)
	UpdateItem(ctx context.Context, id int, input GalleryItemUpdateInput) (*GalleryItem, error)
	RemoveItem(ctx context.Context, id int) error
	GetItems(ctx context.Context, galleryID int) (GalleryItems, error)
	GetItemByID(ctx context.Context, itemID int) (*GalleryItem, error)
	ReorderItems(ctx context.Context, items []GalleryItemReorder) error
	SetCover(ctx context.Context, galleryID, itemID int) error
	GetCover(ctx context.Context, galleryID int) (*GalleryItem, error)
}

// ErrGalleryStyle - a gallery's preview style the site has no image
// style of.
var ErrGalleryStyle = errors.New("no such image style")

// StyleNames - the core's image styles by name (a gallery's preview
// styles must exist).
type StyleNames interface {
	ImageStyleByName(ctx context.Context, name string) (*modulekit.ImageStyleDef, error)
}

// DefaultGalleriesService - GalleryService on a GalleriesRepository.
type DefaultGalleriesService struct {
	repository GalleriesRepository
	styles     StyleNames
	logger     *slog.Logger
}

// NewGalleryService - the galleries service on r; a gallery's preview
// styles are checked against styles (the core's image styles).
func NewGalleryService(r GalleriesRepository, styles StyleNames, baseLogger *slog.Logger) GalleryService {
	return &DefaultGalleriesService{
		repository: r,
		styles:     styles,
		logger:     baseLogger.With("component", "galleries.service"),
	}
}

// checkStyles reports a preview style the site has none of
// (ErrGalleryStyle); nil - not changed.
func (s *DefaultGalleriesService) checkStyles(ctx context.Context, names ...*string) error {
	for _, n := range names {
		if n == nil {
			continue
		}
		if _, err := s.styles.ImageStyleByName(ctx, *n); err != nil {
			if modulekit.IsNotFound(err) {
				return fmt.Errorf("%w: %q", ErrGalleryStyle, *n)
			}
			return fmt.Errorf("the image style %q: %w", *n, err)
		}
	}
	return nil
}

// Find - the galleries matching filter, with their items and nodes.
func (s *DefaultGalleriesService) Find(ctx context.Context, filter GalleriesFilter) (Galleries, error) {
	galleries, err := s.repository.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("finding galleries: %w", err)
	}

	s.logger.Debug("galleries found",
		"count", len(galleries),
		"filter", filter,
	)

	return galleries, nil
}

// TotalCount - the number of galleries of filter's node and status.
func (s *DefaultGalleriesService) TotalCount(ctx context.Context, filter GalleriesFilter) (int, error) {
	total, err := s.repository.TotalCount(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("getting galleries total count: %w", err)
	}

	s.logger.Debug("galleries total count",
		"total", total,
		"filter", filter,
	)

	return total, nil
}

// ByID - the gallery with its items and its node.
func (s *DefaultGalleriesService) ByID(ctx context.Context, id int) (*Gallery, error) {
	gallery, err := s.repository.ByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting gallery by ID (id %v): %w", id, err)
	}

	s.logger.Debug("gallery found by ID",
		"id", id,
		"items_count", len(gallery.Items),
	)

	return gallery, nil
}

// ByNodeID - the node's shown galleries with their items.
func (s *DefaultGalleriesService) ByNodeID(ctx context.Context, nodeID int) (Galleries, error) {
	galleries, err := s.repository.ByNodeID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("getting galleries by node ID (node_id %v): %w", nodeID, err)
	}

	return galleries, nil
}

// CountsByNodeIDs - the galleries and images of the nodes among ids that
// have any (the admin's galleries overview).
func (s *DefaultGalleriesService) CountsByNodeIDs(ctx context.Context, ids []int) (map[int]NodeGalleryCounts, error) {
	counts, err := s.repository.CountsByNodeIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("counting the galleries of nodes %v: %w", ids, err)
	}
	return counts, nil
}

// SummaryByNodeID - the node's shown galleries without their images'
// files and previews (their titles and image counts).
func (s *DefaultGalleriesService) SummaryByNodeID(ctx context.Context, nodeID int) (Galleries, error) {
	galleries, err := s.repository.SummaryByNodeID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("getting galleries summary by node ID (node_id %v): %w", nodeID, err)
	}

	return galleries, nil
}

// Create creates a gallery of the node: empty preview styles get the
// defaults, a style the site lacks is ErrGalleryStyle, an over-long
// title ErrGalleryFieldTooLong.
func (s *DefaultGalleriesService) Create(ctx context.Context, input GalleryCreateInput) (*Gallery, error) {
	if err := checkGalleryLen("title", input.Title, galleryTitleMaxLen); err != nil {
		return nil, err
	}
	// Validation
	if input.NodeID <= 0 {
		return nil, fmt.Errorf("invalid node ID: %d", input.NodeID)
	}
	if input.PreviewSmallStyle == "" {
		input.PreviewSmallStyle = schema.DefaultPreviewSmallStyle
	}
	if input.PreviewMediumStyle == "" {
		input.PreviewMediumStyle = schema.DefaultPreviewMediumStyle
	}
	if err := s.checkStyles(ctx, &input.PreviewSmallStyle, &input.PreviewMediumStyle); err != nil {
		return nil, err
	}

	gallery, err := s.repository.Create(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("creating gallery (node %v): %w", input.NodeID, err)
	}

	s.logger.Info("gallery created",
		"id", gallery.ID,
		"node_id", gallery.NodeID,
	)

	return gallery, nil
}

// Update changes the gallery's fields set in input (a style the site
// lacks - ErrGalleryStyle, an over-long title - ErrGalleryFieldTooLong).
func (s *DefaultGalleriesService) Update(ctx context.Context, id int, input GalleryUpdateInput) (*Gallery, error) {
	if err := checkGalleryLen("title", input.Title, galleryTitleMaxLen); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, fmt.Errorf("invalid gallery ID: %d", id)
	}
	if err := s.checkStyles(ctx, input.PreviewSmallStyle, input.PreviewMediumStyle); err != nil {
		return nil, err
	}

	gallery, err := s.repository.Update(ctx, id, input)
	if err != nil {
		return nil, fmt.Errorf("updating gallery (id %v, input %v): %w", id, input, err)
	}

	s.logger.Info("gallery updated", "id", gallery.ID)

	return gallery, nil
}

// Delete deletes the gallery (its items go by cascade).
func (s *DefaultGalleriesService) Delete(ctx context.Context, id int) error {
	err := s.repository.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting gallery (id %v): %w", id, err)
	}

	return nil
}

// DeleteWithItems deletes the gallery together with its items.
func (s *DefaultGalleriesService) DeleteWithItems(ctx context.Context, id int) error {
	if err := s.repository.DeleteWithItems(ctx, id); err != nil {
		return fmt.Errorf("deleting gallery with items (id %v): %w", id, err)
	}
	return nil
}

// DeleteByNodeIDWithItems deletes all the node's galleries with their
// items.
func (s *DefaultGalleriesService) DeleteByNodeIDWithItems(ctx context.Context, nodeID int) error {
	err := s.repository.DeleteByNodeIDWithItems(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("deleting galleries by node ID with items (node_id %v): %w", nodeID, err)
	}

	return nil
}

// AddItem adds an image (a file) to a gallery (an over-long title -
// ErrGalleryFieldTooLong).
func (s *DefaultGalleriesService) AddItem(ctx context.Context, input GalleryItemCreateInput) (*GalleryItem, error) {
	if err := checkGalleryLen("title", input.Title, galleryTitleMaxLen); err != nil {
		return nil, err
	}
	// Validation
	if input.GalleryID <= 0 {
		return nil, fmt.Errorf("invalid gallery ID: %d", input.GalleryID)
	}

	if input.FileID <= 0 {
		return nil, fmt.Errorf("invalid file ID: %d", input.FileID)
	}

	item, err := s.repository.AddItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("adding gallery item (gallery %v, file %v): %w", input.GalleryID, input.FileID, err)
	}

	s.logger.Info("gallery item added",
		"id", item.ID,
		"gallery_id", item.GalleryID,
		"file_id", item.FileID,
	)

	return item, nil
}

// UpdateItem changes the item's fields set in input (an over-long title -
// ErrGalleryFieldTooLong).
func (s *DefaultGalleriesService) UpdateItem(ctx context.Context, id int, input GalleryItemUpdateInput) (*GalleryItem, error) {
	if err := checkGalleryLen("title", input.Title, galleryTitleMaxLen); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, fmt.Errorf("invalid item ID: %d", id)
	}

	item, err := s.repository.UpdateItem(ctx, id, input)
	if err != nil {
		return nil, fmt.Errorf("updating gallery item (id %v, input %v): %w", id, input, err)
	}

	s.logger.Info("gallery item updated",
		"id", item.ID,
		"gallery_id", item.GalleryID,
	)

	return item, nil
}

// RemoveItem removes the item from its gallery.
func (s *DefaultGalleriesService) RemoveItem(ctx context.Context, id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid item ID: %d", id)
	}

	err := s.repository.RemoveItem(ctx, id)
	if err != nil {
		return fmt.Errorf("removing gallery item (id %v): %w", id, err)
	}

	s.logger.Info("gallery item removed",
		"id", id,
	)

	return nil
}

// GetItems - the gallery's items in their order, with their files and
// previews.
func (s *DefaultGalleriesService) GetItems(ctx context.Context, galleryID int) (GalleryItems, error) {
	if galleryID <= 0 {
		return nil, fmt.Errorf("invalid gallery ID: %d", galleryID)
	}

	items, err := s.repository.GetItemsByGalleryID(ctx, galleryID)
	if err != nil {
		return nil, fmt.Errorf("getting gallery items (gallery_id %v): %w", galleryID, err)
	}

	s.logger.Debug("gallery items retrieved",
		"gallery_id", galleryID,
		"count", len(items),
	)

	return items, nil
}

// GetItemByID - the gallery item with its file and previews.
func (s *DefaultGalleriesService) GetItemByID(ctx context.Context, itemID int) (*GalleryItem, error) {
	if itemID <= 0 {
		return nil, fmt.Errorf("invalid item ID: %d", itemID)
	}

	item, err := s.repository.GetItemByID(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("getting gallery item by ID (item_id %v): %w", itemID, err)
	}

	s.logger.Debug("gallery item retrieved",
		"item_id", itemID,
	)

	return item, nil
}

// ReorderItems sets the items' weights (none to set, a bad ID or a
// negative weight - an error).
func (s *DefaultGalleriesService) ReorderItems(ctx context.Context, items []GalleryItemReorder) error {
	if len(items) == 0 {
		return fmt.Errorf("no items to reorder")
	}

	// Validation
	for _, item := range items {
		if item.ID <= 0 {
			return fmt.Errorf("invalid item ID: %d", item.ID)
		}
		if item.Weight < 0 {
			return fmt.Errorf("invalid weight for item %d: %d", item.ID, item.Weight)
		}
	}

	err := s.repository.ReorderItems(ctx, items)
	if err != nil {
		return fmt.Errorf("reordering gallery items (items_count %v): %w", len(items), err)
	}

	s.logger.Info("gallery items reordered",
		"items_count", len(items),
	)

	return nil
}

// SetCover makes the item the gallery's only cover.
func (s *DefaultGalleriesService) SetCover(ctx context.Context, galleryID, itemID int) error {
	if galleryID <= 0 {
		return fmt.Errorf("invalid gallery ID: %d", galleryID)
	}

	if itemID <= 0 {
		return fmt.Errorf("invalid item ID: %d", itemID)
	}

	err := s.repository.SetCover(ctx, galleryID, itemID)
	if err != nil {
		return fmt.Errorf("setting gallery cover (gallery_id %v, item_id %v): %w", galleryID, itemID, err)
	}

	s.logger.Info("gallery cover updated",
		"gallery_id", galleryID,
		"item_id", itemID,
	)

	return nil
}

// GetCover - the gallery's cover: the item marked as cover, else its
// first item by weight.
func (s *DefaultGalleriesService) GetCover(ctx context.Context, galleryID int) (*GalleryItem, error) {
	if galleryID <= 0 {
		return nil, fmt.Errorf("invalid gallery ID: %d", galleryID)
	}

	cover, err := s.repository.GetCover(ctx, galleryID)
	if err != nil {
		return nil, fmt.Errorf("getting gallery cover (gallery_id %v): %w", galleryID, err)
	}

	s.logger.Debug("gallery cover retrieved",
		"gallery_id", galleryID,
		"item_id", cover.ID,
	)

	return cover, nil
}
