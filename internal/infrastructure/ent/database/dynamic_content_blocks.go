package database

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvalueblock"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/schema"
)

// blockSet - a field set as its items are read and saved: the type and
// its fields.
type blockSet struct {
	def    entity.ContentTypeDef
	fields entity.FieldInstanceDefs
}

// blockSets - the field sets named, read once per call; a name that
// isn't a field set is left out.
func (r DynamicContentRepository) blockSets(ctx context.Context, names []string) (map[string]blockSet, error) {
	out := map[string]blockSet{}
	for _, name := range names {
		if _, done := out[name]; done {
			continue
		}
		def, err := r.types.ContentTypeByMachineName(ctx, name)
		if generated.IsNotFound(err) || errors.Is(err, entity.ErrEntityNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("loading the field set %q: %w", name, err)
		}
		if !def.Embedded {
			continue
		}
		fields, err := r.types.FieldsByContentTypeID(ctx, def.ID)
		if err != nil {
			return nil, fmt.Errorf("loading the fields of the field set %q: %w", name, err)
		}
		out[name] = blockSet{def: *def, fields: fields.SortBySortOrder()}
	}
	return out, nil
}

// loadBlockItems puts in values, for each blocks field, its items (the
// item IDs read) with their content: their set's fields, their values
// and text formats - read for all the items of a set at once. An item
// whose set is gone is left out.
func (r DynamicContentRepository) loadBlockItems(ctx context.Context, client *generated.Client, values map[int]any) error {
	ctx = schema.WithBlocks(ctx)
	for _, f := range r.fields {
		if f.FieldType != entity.FieldTypeBlocks {
			continue
		}
		ids := entity.BlockItemIDs(values[f.ID])
		if len(ids) == 0 {
			continue
		}
		rows, err := client.Node.Query().Where(node.IDIn(ids...)).Select(node.FieldID, node.FieldType).All(ctx)
		if err != nil {
			return fmt.Errorf("loading the blocks of field %s: %w", f.MachineName, err)
		}
		typeOf := make(map[int]string, len(rows))
		for _, row := range rows {
			typeOf[row.ID] = row.Type
		}
		sets, err := r.blockSets(ctx, slices.Collect(maps.Values(typeOf)))
		if err != nil {
			return err
		}
		bySet := map[string][]int{}
		for _, id := range ids {
			bySet[typeOf[id]] = append(bySet[typeOf[id]], id)
		}
		setValues := map[int]map[int]any{}
		setFormats := map[int]map[int]int{}
		for name, setIDs := range bySet {
			set, ok := sets[name]
			if !ok {
				continue
			}
			vals, err := ContentTypeRepository{client: r.client}.fieldValues(ctx, setIDs, set.fields, true)
			if err != nil {
				return fmt.Errorf("loading the values of the blocks of %s: %w", name, err)
			}
			formats, err := textFormatIDsForNodes(ctx, client, setIDs, set.fields)
			if err != nil {
				return fmt.Errorf("loading the text formats of the blocks of %s: %w", name, err)
			}
			maps.Copy(setValues, vals)
			maps.Copy(setFormats, formats)
		}
		items := make([]entity.BlockItem, 0, len(ids))
		for _, id := range ids {
			set, ok := sets[typeOf[id]]
			if !ok {
				continue
			}
			formats := setFormats[id]
			if formats == nil {
				formats = map[int]int{}
			}
			items = append(items, entity.BlockItem{ID: id, Type: set.def.MachineName, Fields: set.fields, Values: setValues[id], TextFormats: formats})
		}
		values[f.ID] = items
	}
	return nil
}

// saveBlockItems saves the items of the blocks fields in values with
// their node parent (its ID set): an item of the field's own (its ID and
// set those of one of the parent's items of that field) updated, any
// other created, the field's items left out deleted; a new item of a set
// the field doesn't take (any more) is dropped - an own one is kept. The values returned have each blocks field's item IDs
// in order (SetFieldValues writes them). An item's values are its set's
// fields' - by their instance IDs, or its Snapshot's (a reverted
// revision).
func (r DynamicContentRepository) saveBlockItems(ctx context.Context, client *generated.Client, parent entity.Node, values map[int]any) (map[int]any, error) {
	ctx = schema.WithBlocks(ctx)
	out := values
	cloned := false
	for _, f := range r.fields {
		if f.FieldType != entity.FieldTypeBlocks {
			continue
		}
		if !cloned {
			out, cloned = maps.Clone(values), true
			if out == nil {
				out = map[int]any{}
			}
		}
		own, err := client.FieldValueBlock.Query().
			Where(fieldvalueblock.NodeID(parent.ID), fieldvalueblock.FieldIDEQ(f.FieldStorageID)).
			Select(fieldvalueblock.FieldItemID).Ints(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading the blocks of field %s (node %d): %w", f.MachineName, parent.ID, err)
		}
		ownTypes := map[int]string{}
		if len(own) > 0 {
			rows, err := client.Node.Query().Where(node.IDIn(own...)).Select(node.FieldID, node.FieldType).All(ctx)
			if err != nil {
				return nil, fmt.Errorf("loading the blocks of field %s (node %d): %w", f.MachineName, parent.ID, err)
			}
			for _, row := range rows {
				ownTypes[row.ID] = row.Type
			}
		}
		sets, err := r.blockSets(ctx, append(f.BlockTypes(), slices.Collect(maps.Values(ownTypes))...))
		if err != nil {
			return nil, err
		}
		var ids []int
		for _, item := range entity.BlockItems(values[f.ID]) {
			isOwn := item.ID > 0 && ownTypes[item.ID] == item.Type
			set, ok := sets[item.Type]
			if !ok || (!isOwn && !slices.Contains(f.BlockTypes(), item.Type)) {
				continue
			}
			id, err := r.saveBlockItem(ctx, client, parent, set, item, isOwn)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		var stale []int
		for _, id := range own {
			if !slices.Contains(ids, id) {
				stale = append(stale, id)
			}
		}
		if len(stale) > 0 {
			// Their values, their row in the field go with them (cascade).
			if _, err := client.Node.Delete().Where(node.IDIn(stale...), node.ParentID(parent.ID)).Exec(ctx); err != nil {
				return nil, fmt.Errorf("deleting the blocks %v of node %d: %w", stale, parent.ID, err)
			}
		}
		out[f.ID] = ids
	}
	return out, nil
}

// saveBlockItem saves item of set with parent: its node updated (own -
// one of the parent's) or created, then its values; its ID.
func (r DynamicContentRepository) saveBlockItem(ctx context.Context, client *generated.Client, parent entity.Node, set blockSet, item entity.BlockItem, own bool) (int, error) {
	values, formats := item.Values, item.TextFormats
	if item.Snapshot != nil {
		values, formats = item.Snapshot.Values(set.fields)
	}
	title := entity.BlockItem{Fields: set.fields, Values: values}.Summary()
	if title == "" {
		title = set.def.Label
	}
	now := time.Now()
	id := item.ID
	if own {
		if err := client.Node.UpdateOneID(id).SetTitle(title).SetUpdatedAt(now).Exec(ctx); err != nil {
			return 0, fmt.Errorf("updating block %d: %w", id, err)
		}
	} else {
		// No author: the item is its parent's (a user's content is
		// counted, reassigned and deleted by its parents).
		row, err := client.Node.Create().
			SetType(set.def.MachineName).
			SetTitle(title).
			SetStatus(1).
			SetCommentMode(string(entity.CommentModeClosed)).
			SetParentID(parent.ID).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			Save(ctx)
		if err != nil {
			return 0, fmt.Errorf("creating a block of %s (node %d): %w", set.def.MachineName, parent.ID, err)
		}
		id = row.ID
	}
	if err := r.types.SetFieldValues(ctx, id, set.fields, values, formats); err != nil {
		return 0, fmt.Errorf("saving the values of block %d: %w", id, err)
	}
	return id, nil
}
