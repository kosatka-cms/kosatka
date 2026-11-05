package database

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluelist"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluelistmulti"
)

// listStore - field_value_list: a list field's value, one per node.
var listStore = &fieldStore{
	table: listValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueList.Query().Where(fieldvaluelist.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluelist.NodeIDIn(nodeIDs...)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading list values: %w", err)
		}
		for _, row := range rows {
			sink.put(row.NodeID, row.FieldID, row.Value)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueList.Delete().
			Where(fieldvaluelist.NodeID(nodeID), fieldvaluelist.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		if s, ok := v.(string); ok && s != "" {
			return c.FieldValueList.Create().SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetValue(s).Exec(ctx)
		}
		return nil
	},
}

// listMultiStore - field_value_list_multi: a list field's several values
// ([]string) in the order saved, each once.
var listMultiStore = &fieldStore{
	table: listMultiValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueListMulti.Query().Where(fieldvaluelistmulti.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluelistmulti.NodeIDIn(nodeIDs...)).
			Order(generated.Asc(fieldvaluelistmulti.FieldID)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading multi-value list values: %w", err)
		}
		for _, row := range rows {
			sink.add(row.NodeID, row.FieldID, row.Value)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueListMulti.Delete().
			Where(fieldvaluelistmulti.NodeID(nodeID), fieldvaluelistmulti.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		vals, _ := v.([]string)
		seen := map[string]bool{}
		for _, val := range vals {
			if val == "" || seen[val] {
				continue
			}
			seen[val] = true
			if err := c.FieldValueListMulti.Create().SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetValue(val).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	registerFieldStorage(fieldStorage{single: listStore, multiple: listMultiStore, moveColumn: fieldvaluelist.FieldValue}, entity.FieldTypeList)
}
