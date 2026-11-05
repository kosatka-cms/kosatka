package database

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetext"
)

// textStore - field_value_text: a text (with its text format), an
// e-mail address, a phone number; one per node.
var textStore = &fieldStore{
	table: textValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueText.Query().Where(fieldvaluetext.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluetext.NodeIDIn(nodeIDs...)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading text values: %w", err)
		}
		for _, row := range rows {
			sink.put(row.NodeID, row.FieldID, row.Value)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, textFormat int) error {
		if _, err := c.FieldValueText.Delete().
			Where(fieldvaluetext.NodeID(nodeID), fieldvaluetext.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		s, ok := v.(string)
		if !ok || s == "" {
			return nil
		}
		create := c.FieldValueText.Create().SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetValue(s)
		if textFormat > 0 {
			create = create.SetTextFormatID(textFormat)
		}
		if _, err := create.Save(ctx); err != nil {
			return fmt.Errorf("saving the text of field %s (node %d): %w", f.MachineName, nodeID, err)
		}
		return nil
	},
}

func init() {
	registerFieldStorage(fieldStorage{single: textStore}, entity.FieldTypeText, entity.FieldTypeEmail, entity.FieldTypePhone)
}
