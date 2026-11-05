package galleries

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated/gallery"
	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated/galleryitem"
	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// NodeRefs and FileRefs - the core's nodes and files a gallery and its
// items reference, by ID (NodesRepository.ByIDs, FileRepository.ByIDs).
type (
	NodeRefs interface {
		ByIDs(ctx context.Context, ids []int) (map[int]modulekit.Node, error)
	}
	// FileRefs - the core's files of the gallery items, by ID.
	FileRefs interface {
		ByIDs(ctx context.Context, ids []int) (map[int]modulekit.File, error)
	}
	// StyleRefs - the core's image styles (their sizes).
	StyleRefs interface {
		ImageStyles(ctx context.Context) (modulekit.ImageStyleDefs, error)
	}
)

// NewGalleryRepository - the galleries on db, on the module's own ent
// schema (ent/); their nodes, items' files and preview styles come from
// the core.
func NewGalleryRepository(db *sql.DB, nodes NodeRefs, files FileRefs, styles StyleRefs) *EntGalleryRepository {
	return &EntGalleryRepository{db: db, nodes: nodes, files: files, styles: styles}
}

// EntGalleryRepository provides database access to galleries.
type EntGalleryRepository struct {
	db     *sql.DB
	nodes  NodeRefs
	files  FileRefs
	styles StyleRefs
}

// client - the module's client for ctx: in its transaction, if any.
func (r *EntGalleryRepository) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// refs - the core's rows the galleries (their nodes, with withNodes) and
// items (their files) reference, each kind in one query.
func (r *EntGalleryRepository) refs(ctx context.Context, galleries []*generated.Gallery, items []*generated.GalleryItem, withNodes bool) (coreRefs, error) {
	var nodeIDs, fileIDs []int
	for _, g := range galleries {
		if withNodes {
			nodeIDs = append(nodeIDs, g.Nid)
		}
		items = append(items, g.Edges.Items...)
	}
	for _, it := range items {
		if it.Fid > 0 {
			fileIDs = append(fileIDs, it.Fid)
		}
	}
	refs := coreRefs{styles: r.styleSizes(ctx)}
	var err error
	if refs.nodes, err = r.nodes.ByIDs(ctx, nodeIDs); err != nil {
		return coreRefs{}, fmt.Errorf("the galleries' nodes: %w", err)
	}
	if refs.files, err = r.files.ByIDs(ctx, fileIDs); err != nil {
		return coreRefs{}, fmt.Errorf("the gallery items' files: %w", err)
	}
	return refs, nil
}

// galleries - the rows as galleries, with their nodes when withNodes.
func (r *EntGalleryRepository) galleries(ctx context.Context, rows []*generated.Gallery, withNodes bool) (Galleries, error) {
	refs, err := r.refs(ctx, rows, nil, withNodes)
	if err != nil {
		return nil, err
	}
	return ToGalleriesEntity(rows, refs), nil
}

// items - the rows as gallery items, with their files.
func (r *EntGalleryRepository) items(ctx context.Context, rows []*generated.GalleryItem) (GalleryItems, error) {
	refs, err := r.refs(ctx, nil, rows, false)
	if err != nil {
		return nil, err
	}
	return ToGalleryItemsEntity(rows, refs), nil
}

// styleSizes - the image styles by name, for the galleries' preview
// sizes, once per repository call. A failure leaves the sizes unknown
// (the plain grid) rather than breaking gallery reads.
func (r *EntGalleryRepository) styleSizes(ctx context.Context) map[string]modulekit.ImageStyleDef {
	if r.styles == nil {
		return nil
	}
	list, err := r.styles.ImageStyles(ctx)
	if err != nil {
		return nil
	}
	out := make(map[string]modulekit.ImageStyleDef, len(list))
	for _, st := range list {
		out[st.Name] = st
	}
	return out
}

// Find returns galleries matching the filter.
func (r *EntGalleryRepository) Find(ctx context.Context, filter GalleriesFilter) (Galleries, error) {
	query := r.client(ctx).Gallery.Query().Where(galleriesWhere(filter)...)

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	query = query.
		Order(generated.Asc(gallery.FieldWeight), generated.Desc(gallery.FieldCreatedAt)).
		WithItems(func(iq *generated.GalleryItemQuery) {
			iq.Select(
				galleryitem.FieldID,
				galleryitem.FieldGalleryID,
				galleryitem.FieldFid,
				galleryitem.FieldTitle,
				galleryitem.FieldDescription,
				galleryitem.FieldWeight,
				galleryitem.FieldIsCover,
				galleryitem.FieldCreatedAt,
			).
				Order(generated.Asc(galleryitem.FieldWeight))
		})

	res, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("error finding galleries: %w", err)
	}

	return r.galleries(ctx, res, true)
}

// TotalCount returns the number of galleries matching the filter.
func (r *EntGalleryRepository) TotalCount(ctx context.Context, filter GalleriesFilter) (int, error) {
	totalCount, err := r.client(ctx).Gallery.Query().Where(galleriesWhere(filter)...).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("error counting galleries: %w", err)
	}

	return totalCount, nil
}

// galleriesWhere - filter's conditions (its node, status, search), the
// same for Find and TotalCount.
func galleriesWhere(filter GalleriesFilter) []predicate.Gallery {
	var where []predicate.Gallery
	if filter.NodeID > 0 {
		where = append(where, gallery.Nid(filter.NodeID))
	}
	if filter.Status != nil {
		where = append(where, gallery.Status(*filter.Status))
	}
	if filter.Query != "" {
		match := []predicate.Gallery{gallery.TitleContainsFold(filter.Query)}
		if len(filter.NodeIDs) > 0 {
			match = append(match, gallery.NidIn(filter.NodeIDs...))
		}
		where = append(where, gallery.Or(match...))
	}
	return where
}

// ByID returns a gallery by ID with all its items.
func (r *EntGalleryRepository) ByID(ctx context.Context, id int) (*Gallery, error) {
	client := r.client(ctx)
	res, err := client.Gallery.Query().
		Where(gallery.ID(id)).
		WithItems(func(iq *generated.GalleryItemQuery) {
			iq.Select(
				galleryitem.FieldID,
				galleryitem.FieldGalleryID,
				galleryitem.FieldFid,
				galleryitem.FieldTitle,
				galleryitem.FieldDescription,
				galleryitem.FieldWeight,
				galleryitem.FieldIsCover,
				galleryitem.FieldCreatedAt,
			).
				Order(generated.Asc(galleryitem.FieldWeight))
		}).
		Only(ctx)

	if err != nil {
		return nil, fmt.Errorf("error finding gallery by ID: %w", err)
	}

	list, err := r.galleries(ctx, []*generated.Gallery{res}, true)
	if err != nil {
		return nil, err
	}
	return list[0], nil
}

// ByNodeID returns all galleries of a node.
func (r *EntGalleryRepository) ByNodeID(ctx context.Context, nodeID int) (Galleries, error) {
	client := r.client(ctx)
	res, err := client.Gallery.Query().
		Where(
			gallery.Nid(nodeID),
			gallery.Status(true),
		).
		Order(generated.Asc(gallery.FieldWeight), generated.Desc(gallery.FieldCreatedAt)).
		WithItems(func(iq *generated.GalleryItemQuery) {
			iq.Select(
				galleryitem.FieldID,
				galleryitem.FieldGalleryID,
				galleryitem.FieldFid,
				galleryitem.FieldTitle,
				galleryitem.FieldDescription,
				galleryitem.FieldWeight,
				galleryitem.FieldIsCover,
				galleryitem.FieldCreatedAt,
			).
				Order(generated.Asc(galleryitem.FieldWeight))
		}).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("error finding galleries by node ID: %w", err)
	}

	return r.galleries(ctx, res, true)
}

// CountsByNodeIDs - per node of ids, its galleries and their images.
func (r *EntGalleryRepository) CountsByNodeIDs(ctx context.Context, ids []int) (map[int]NodeGalleryCounts, error) {
	out := map[int]NodeGalleryCounts{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.client(ctx).Gallery.Query().
		Where(gallery.NidIn(ids...)).
		Select(gallery.FieldID, gallery.FieldNid).
		WithItems(func(q *generated.GalleryItemQuery) { q.Select(galleryitem.FieldGalleryID) }).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, g := range rows {
		c := out[g.Nid]
		c.Galleries++
		c.Images += len(g.Edges.Items)
		out[g.Nid] = c
	}
	return out, nil
}

// SummaryByNodeID returns galleries for a node without loading item files/previews -
// cheap enough to call synchronously (e.g. for a "gallery name + link" list under the main image).
func (r *EntGalleryRepository) SummaryByNodeID(ctx context.Context, nodeID int) (Galleries, error) {
	client := r.client(ctx)
	res, err := client.Gallery.Query().
		Where(
			gallery.Nid(nodeID),
			gallery.Status(true),
		).
		Order(generated.Asc(gallery.FieldWeight), generated.Desc(gallery.FieldCreatedAt)).
		WithItems(func(iq *generated.GalleryItemQuery) {
			iq.Select(galleryitem.FieldID)
		}).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("error finding galleries summary by node ID: %w", err)
	}

	return ToGalleriesEntity(res, coreRefs{styles: r.styleSizes(ctx)}), nil
}

// Create creates a gallery.
func (r *EntGalleryRepository) Create(ctx context.Context, input GalleryCreateInput) (*Gallery, error) {
	client := r.client(ctx)
	builder := client.Gallery.Create().
		SetNid(input.NodeID).
		SetPreviewSmallStyle(input.PreviewSmallStyle).
		SetPreviewMediumStyle(input.PreviewMediumStyle).
		SetWeight(input.Weight).
		SetStatus(input.Status)

	if input.Title != nil {
		builder = builder.SetTitle(*input.Title)
	}

	if input.Description != nil {
		builder = builder.SetDescription(*input.Description)
	}

	res, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating gallery: %w", err)
	}

	return ToGalleryEntity(res, coreRefs{styles: r.styleSizes(ctx)}), nil
}

// Update updates a gallery.
func (r *EntGalleryRepository) Update(ctx context.Context, id int, input GalleryUpdateInput) (*Gallery, error) {
	client := r.client(ctx)
	builder := client.Gallery.UpdateOneID(id)

	if input.PreviewSmallStyle != nil {
		builder = builder.SetPreviewSmallStyle(*input.PreviewSmallStyle)
	}
	if input.PreviewMediumStyle != nil {
		builder = builder.SetPreviewMediumStyle(*input.PreviewMediumStyle)
	}

	if input.Title != nil {
		builder = builder.SetTitle(*input.Title)
	}

	if input.Description != nil {
		builder = builder.SetDescription(*input.Description)
	}

	if input.Weight != nil {
		builder = builder.SetWeight(*input.Weight)
	}

	if input.Status != nil {
		builder = builder.SetStatus(*input.Status)
	}

	res, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error updating gallery: %w", err)
	}

	return ToGalleryEntity(res, coreRefs{styles: r.styleSizes(ctx)}), nil
}

// Delete deletes a gallery (items are removed by cascade).
func (r *EntGalleryRepository) Delete(ctx context.Context, id int) error {
	client := r.client(ctx)
	err := client.Gallery.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error deleting gallery: %w", err)
	}

	return nil
}

// DeleteWithItems deletes a gallery together with all its items.
func (r *EntGalleryRepository) DeleteWithItems(ctx context.Context, id int) error {
	client := r.client(ctx)
	// Delete the items first.
	_, err := client.GalleryItem.Delete().
		Where(galleryitem.GalleryID(id)).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("error deleting gallery items: %w", err)
	}

	// Then the gallery itself.
	err = client.Gallery.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error deleting gallery: %w", err)
	}

	return nil
}

// DeleteByNodeIDWithItems deletes all galleries of a node with their items.
func (r *EntGalleryRepository) DeleteByNodeIDWithItems(ctx context.Context, nodeID int) error {
	client := r.client(ctx)
	galleries, err := client.Gallery.Query().
		Where(gallery.Nid(nodeID)).
		Select(gallery.FieldID).
		All(ctx)

	if err != nil {
		return fmt.Errorf("error getting galleries by node ID: %w", err)
	}

	if len(galleries) == 0 {
		return nil
	}

	galleryIDs := make([]int, len(galleries))
	for i, g := range galleries {
		galleryIDs[i] = g.ID
	}

	// Delete all their items in one query.
	_, err = client.GalleryItem.Delete().
		Where(galleryitem.GalleryIDIn(galleryIDs...)).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("error deleting gallery items by node ID: %w", err)
	}

	// Then the galleries in one query.
	_, err = client.Gallery.Delete().
		Where(gallery.Nid(nodeID)).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("error deleting galleries by node ID: %w", err)
	}

	return nil
}

// RemoveItem removes an item from a gallery.
func (r *EntGalleryRepository) RemoveItem(ctx context.Context, id int) error {
	client := r.client(ctx)
	err := client.GalleryItem.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error removing gallery item: %w", err)
	}

	return nil
}

// GetItemsByGalleryID returns all items of a gallery.
func (r *EntGalleryRepository) GetItemsByGalleryID(ctx context.Context, galleryID int) (GalleryItems, error) {
	client := r.client(ctx)
	res, err := client.GalleryItem.Query().
		Where(galleryitem.GalleryID(galleryID)).
		Order(generated.Asc(galleryitem.FieldWeight)).
		WithGallery().
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("error getting gallery items: %w", err)
	}

	return r.items(ctx, res)
}

// GetItemByID returns a gallery item by ID with its file and its previews
// in its gallery's styles.
func (r *EntGalleryRepository) GetItemByID(ctx context.Context, itemID int) (*GalleryItem, error) {
	client := r.client(ctx)
	res, err := client.GalleryItem.Query().
		Where(galleryitem.ID(itemID)).
		WithGallery().
		Only(ctx)

	if err != nil {
		if generated.IsNotFound(err) {
			return nil, fmt.Errorf("gallery item not found: %w", err)
		}
		return nil, fmt.Errorf("error getting gallery item: %w", err)
	}

	list, err := r.items(ctx, []*generated.GalleryItem{res})
	if err != nil {
		return nil, err
	}
	return list[0], nil
}

// ReorderItems updates the weight of several items at once.
func (r *EntGalleryRepository) ReorderItems(ctx context.Context, items []GalleryItemReorder) error {
	client := r.client(ctx)
	for _, item := range items {
		err := client.GalleryItem.UpdateOneID(item.ID).
			SetWeight(item.Weight).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("error reordering gallery item %d: %w", item.ID, err)
		}
	}

	return nil
}

// SetCover marks an item as the gallery cover.
func (r *EntGalleryRepository) SetCover(ctx context.Context, galleryID, itemID int) error {
	client := r.client(ctx)
	// Reset existing covers in this gallery first.
	err := client.GalleryItem.Update().
		Where(
			galleryitem.GalleryID(galleryID),
			galleryitem.IsCover(true),
		).
		SetIsCover(false).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("error resetting covers: %w", err)
	}

	err = client.GalleryItem.UpdateOneID(itemID).
		SetIsCover(true).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("error setting cover: %w", err)
	}

	return nil
}

// GetCover returns the gallery cover.
func (r *EntGalleryRepository) GetCover(ctx context.Context, galleryID int) (*GalleryItem, error) {
	client := r.client(ctx)
	item, err := client.GalleryItem.Query().
		Where(
			galleryitem.GalleryID(galleryID),
			galleryitem.IsCover(true),
		).
		First(ctx)

	if err != nil && !generated.IsNotFound(err) {
		return nil, fmt.Errorf("finding gallery %d's cover: %w", galleryID, err)
	}
	if err != nil {
		// No item is marked as cover - fall back to the first image.
		item, err = client.GalleryItem.Query().
			Where(galleryitem.GalleryID(galleryID)).
			Order(generated.Asc(galleryitem.FieldWeight)).
			First(ctx)

		if err != nil {
			return nil, fmt.Errorf("error finding cover: %w", err)
		}
	}

	list, err := r.items(ctx, []*generated.GalleryItem{item})
	if err != nil {
		return nil, err
	}
	return list[0], nil
}

// AddItem adds an item to a gallery.
func (r *EntGalleryRepository) AddItem(ctx context.Context, input GalleryItemCreateInput) (*GalleryItem, error) {
	client := r.client(ctx)
	builder := client.GalleryItem.Create().
		SetGalleryID(input.GalleryID).
		SetFid(input.FileID).
		SetWeight(input.Weight).
		SetIsCover(input.IsCover)

	if input.Title != nil {
		builder = builder.SetTitle(*input.Title)
	}

	if input.Description != nil {
		builder = builder.SetDescription(*input.Description)
	}

	res, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error adding gallery item: %w", err)
	}

	return ToGalleryItemEntity(res, coreRefs{styles: r.styleSizes(ctx)}), nil
}

// UpdateItem updates a gallery item.
func (r *EntGalleryRepository) UpdateItem(ctx context.Context, id int, input GalleryItemUpdateInput) (*GalleryItem, error) {
	client := r.client(ctx)
	builder := client.GalleryItem.UpdateOneID(id)

	if input.Title != nil {
		builder = builder.SetTitle(*input.Title)
	}

	if input.Description != nil {
		builder = builder.SetDescription(*input.Description)
	}

	if input.Weight != nil {
		builder = builder.SetWeight(*input.Weight)
	}

	if input.IsCover != nil {
		builder = builder.SetIsCover(*input.IsCover)
	}

	res, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error updating gallery item: %w", err)
	}

	return ToGalleryItemEntity(res, coreRefs{styles: r.styleSizes(ctx)}), nil
}
