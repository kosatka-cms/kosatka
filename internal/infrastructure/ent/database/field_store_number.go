package database

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluebool"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluenumber"
)

// numberStore - field_value_number: a number (float64), one per node.
var numberStore = &fieldStore{
	table: numberValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueNumber.Query().Where(fieldvaluenumber.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluenumber.NodeIDIn(nodeIDs...)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading number values: %w", err)
		}
		for _, row := range rows {
			if row.Value != nil {
				sink.put(row.NodeID, row.FieldID, *row.Value)
			}
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueNumber.Delete().
			Where(fieldvaluenumber.NodeID(nodeID), fieldvaluenumber.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		if n, ok := v.(float64); ok {
			return c.FieldValueNumber.Create().SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetValue(n).Exec(ctx)
		}
		return nil
	},
}

// boolStore - field_value_bool: yes or no, one per node.
var boolStore = &fieldStore{
	table: boolValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueBool.Query().Where(fieldvaluebool.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluebool.NodeIDIn(nodeIDs...)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading bool values: %w", err)
		}
		for _, row := range rows {
			sink.put(row.NodeID, row.FieldID, row.Value)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueBool.Delete().
			Where(fieldvaluebool.NodeID(nodeID), fieldvaluebool.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		if b, ok := v.(bool); ok {
			return c.FieldValueBool.Create().SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetValue(b).Exec(ctx)
		}
		return nil
	},
}

func init() {
	registerFieldStorage(fieldStorage{single: numberStore}, entity.FieldTypeNumber)
	registerFieldStorage(fieldStorage{single: boolStore}, entity.FieldTypeBool)
}
