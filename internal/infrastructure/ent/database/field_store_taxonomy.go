package database

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomy"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomymulti"
)

// taxonomyStore - field_value_taxonomy: a term's ID, one per node.
var taxonomyStore = &fieldStore{
	table:    taxonomyValues,
	taxonomy: true,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueTaxonomy.Query().Where(fieldvaluetaxonomy.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluetaxonomy.NodeIDIn(nodeIDs...)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading taxonomy values: %w", err)
		}
		for _, row := range rows {
			sink.put(row.NodeID, row.FieldID, row.TermID)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueTaxonomy.Delete().
			Where(fieldvaluetaxonomy.NodeID(nodeID), fieldvaluetaxonomy.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		if termID, ok := v.(int); ok && termID > 0 {
			return c.FieldValueTaxonomy.Create().SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetTermID(termID).Exec(ctx)
		}
		return nil
	},
}

// taxonomyMultiStore - field_value_taxonomy_multi: several terms' IDs
// ([]int) in the order saved, each once (a repeat - "5,new:Foo" where Foo
// is 5 - is dropped: the unique index refuses it).
var taxonomyMultiStore = &fieldStore{
	table:    taxonomyMultiValues,
	taxonomy: true,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueTaxonomyMulti.Query().Where(fieldvaluetaxonomymulti.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluetaxonomymulti.NodeIDIn(nodeIDs...)).
			Order(generated.Asc(fieldvaluetaxonomymulti.FieldID)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading multi-value taxonomy values: %w", err)
		}
		for _, row := range rows {
			sink.add(row.NodeID, row.FieldID, row.TermID)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueTaxonomyMulti.Delete().
			Where(fieldvaluetaxonomymulti.NodeID(nodeID), fieldvaluetaxonomymulti.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		termIDs, _ := v.([]int)
		seen := map[int]bool{}
		for _, termID := range termIDs {
			if termID <= 0 || seen[termID] {
				continue
			}
			seen[termID] = true
			if err := c.FieldValueTaxonomyMulti.Create().SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetTermID(termID).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	registerFieldStorage(fieldStorage{single: taxonomyStore, multiple: taxonomyMultiStore, moveColumn: fieldvaluetaxonomy.FieldTermID}, entity.FieldTypeTaxonomy)
}
