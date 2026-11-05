package database

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluelink"
)

// linkStore - field_value_link, whatever the cardinality: the links
// ([]entity.Link) in the editor's order (delta); a field with one value
// keeps the first.
var linkStore = &fieldStore{
	table: linkValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueLink.Query().Where(fieldvaluelink.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluelink.NodeIDIn(nodeIDs...)).
			Order(generated.Asc(fieldvaluelink.FieldDelta), generated.Asc(fieldvaluelink.FieldID)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading link values: %w", err)
		}
		for _, row := range rows {
			sink.add(row.NodeID, row.FieldID, entity.Link{URL: row.URL, Title: row.Title})
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueLink.Delete().
			Where(fieldvaluelink.NodeID(nodeID), fieldvaluelink.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		links := entity.Links(v)
		if !f.Multiple() && len(links) > 1 {
			links = links[:1]
		}
		delta := 0
		for _, l := range links {
			if l.URL == "" {
				continue
			}
			if _, err := c.FieldValueLink.Create().
				SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetDelta(delta).SetURL(l.URL).SetTitle(l.Title).Save(ctx); err != nil {
				return fmt.Errorf("saving the links of field %s (node %d): %w", f.MachineName, nodeID, err)
			}
			delta++
		}
		return nil
	},
}

func init() {
	registerFieldStorage(fieldStorage{single: linkStore}, entity.FieldTypeLink)
}
