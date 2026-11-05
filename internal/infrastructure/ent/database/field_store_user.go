package database

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvalueuser"
)

// userStore - field_value_user, whatever the cardinality: the users'
// IDs ([]int) in the editor's order (delta), each once; a field with one
// value keeps the first.
var userStore = &fieldStore{
	table: userValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueUser.Query().Where(fieldvalueuser.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvalueuser.NodeIDIn(nodeIDs...)).
			Order(generated.Asc(fieldvalueuser.FieldDelta), generated.Asc(fieldvalueuser.FieldID)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading user values: %w", err)
		}
		for _, row := range rows {
			sink.add(row.NodeID, row.FieldID, row.UserID)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueUser.Delete().
			Where(fieldvalueuser.NodeID(nodeID), fieldvalueuser.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		seen := map[int]bool{}
		delta := 0
		for _, uid := range entity.UserIDs(v) {
			if uid <= 0 || seen[uid] || (!f.Multiple() && delta > 0) {
				continue
			}
			seen[uid] = true
			if _, err := c.FieldValueUser.Create().
				SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetUserID(uid).SetDelta(delta).Save(ctx); err != nil {
				return fmt.Errorf("saving the users of field %s (node %d): %w", f.MachineName, nodeID, err)
			}
			delta++
		}
		return nil
	},
}

func init() {
	registerFieldStorage(fieldStorage{single: userStore}, entity.FieldTypeUser)
}
