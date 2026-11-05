package database

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvalueblock"
)

// blockStore - field_value_block: a blocks field's items' IDs ([]int)
// in the editor's order (delta); the items' nodes are
// DynamicContentRepository's (saveBlockItems, loadBlockItems).
var blockStore = &fieldStore{
	table: blockValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueBlock.Query().Where(fieldvalueblock.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvalueblock.NodeIDIn(nodeIDs...)).
			Order(generated.Asc(fieldvalueblock.FieldDelta), generated.Asc(fieldvalueblock.FieldID)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading block values: %w", err)
		}
		for _, row := range rows {
			sink.add(row.NodeID, row.FieldID, row.ItemID)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueBlock.Delete().
			Where(fieldvalueblock.NodeID(nodeID), fieldvalueblock.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		for delta, itemID := range entity.BlockItemIDs(v) {
			if _, err := c.FieldValueBlock.Create().
				SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetItemID(itemID).SetDelta(delta).Save(ctx); err != nil {
				return fmt.Errorf("saving the block %d of node %d: %w", itemID, nodeID, err)
			}
		}
		return nil
	},
}

func init() {
	registerFieldStorage(fieldStorage{single: blockStore}, entity.FieldTypeBlocks)
}
