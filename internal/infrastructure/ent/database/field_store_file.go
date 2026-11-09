package database

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluefile"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluefilemulti"
)

// fileStore - field_value_file: an image's or a file's file ID, one per
// node.
var fileStore = &fieldStore{
	table: fileValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueFile.Query().Where(fieldvaluefile.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluefile.NodeIDIn(nodeIDs...)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading file values: %w", err)
		}
		for _, row := range rows {
			sink.put(row.NodeID, row.FieldID, row.FileID)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueFile.Delete().
			Where(fieldvaluefile.NodeID(nodeID), fieldvaluefile.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		if fileID, ok := v.(int); ok && fileID > 0 {
			return c.FieldValueFile.Create().SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetFileID(fileID).Exec(ctx)
		}
		return nil
	},
}

// fileMultiStore - field_value_file_multi: an image or a file field's
// several files' IDs ([]int) in the editor's order (delta), each once.
var fileMultiStore = &fieldStore{
	table: fileMultiValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueFileMulti.Query().Where(fieldvaluefilemulti.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluefilemulti.NodeIDIn(nodeIDs...)).
			Order(generated.Asc(fieldvaluefilemulti.FieldDelta), generated.Asc(fieldvaluefilemulti.FieldID)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading multi-value file values: %w", err)
		}
		for _, row := range rows {
			sink.add(row.NodeID, row.FieldID, row.FileID)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueFileMulti.Delete().
			Where(fieldvaluefilemulti.NodeID(nodeID), fieldvaluefilemulti.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		fileIDs, _ := v.([]int)
		seen := make(map[int]bool, len(fileIDs))
		delta := 0
		for _, fileID := range fileIDs {
			if fileID <= 0 || seen[fileID] {
				continue
			}
			seen[fileID] = true
			if err := c.FieldValueFileMulti.Create().
				SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetFileID(fileID).SetDelta(delta).Exec(ctx); err != nil {
				return err
			}
			delta++
		}
		return nil
	},
}

func init() {
	registerFieldStorage(fieldStorage{single: fileStore, multiple: fileMultiStore, moveColumn: fieldvaluefile.FieldFileID}, entity.FieldTypeImage)
	registerFieldStorage(fieldStorage{single: fileStore, multiple: fileMultiStore, moveColumn: fieldvaluefile.FieldFileID}, entity.FieldTypeFile)
}
