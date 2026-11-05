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

// FieldValues - a node's value per field (fieldValues: one query per
// value table, taxonomy fields too). A field never set for this node is
// simply absent from the map - not an error.
func (r ContentTypeRepository) FieldValues(ctx context.Context, nodeID int, fields entity.FieldInstanceDefs) (map[int]any, error) {
	values, err := r.fieldValues(ctx, []int{nodeID}, fields, true)
	if err != nil {
		return nil, err
	}
	return values[nodeID], nil
}

// SetFieldValues replaces each field's values of the node with its
// value in values (absent - none) and, a text, its text format: through
// the store of the field's type (fieldStoreOf), its rows keyed by the
// shared FieldStorageID (not the per-type instance ID). The search index
// gets the fields' words again.
func (r ContentTypeRepository) SetFieldValues(ctx context.Context, nodeID int, fields entity.FieldInstanceDefs, values map[int]any, textFormats map[int]int) error {
	client := GetClient(ctx, r.client)
	storageIDs := make([]int, 0, len(fields))
	for _, f := range fields {
		store, ok := fieldStoreOf(f)
		if !ok {
			continue
		}
		if err := store.write(ctx, client, nodeID, f, values[f.ID], textFormats[f.ID]); err != nil {
			return fmt.Errorf("saving field %s (node %d): %w", f.MachineName, nodeID, err)
		}
		storageIDs = append(storageIDs, f.FieldStorageID)
	}
	return indexFields(ctx, client, nodeID, storageIDs)
}

// TextFormatIDsByNode returns, for fields with non-empty
// AllowedTextFormatIDs(), the TextFormatDef each existing value was saved
// with, so the edit form can preselect it: one query for all of them, none
// if the content type doesn't use formats.
func (r ContentTypeRepository) TextFormatIDsByNode(ctx context.Context, nodeID int, fields entity.FieldInstanceDefs) (map[int]int, error) {
	byNode, err := textFormatIDsForNodes(ctx, GetClient(ctx, r.client), []int{nodeID}, fields)
	if err != nil {
		return nil, err
	}
	if formats := byNode[nodeID]; formats != nil {
		return formats, nil
	}
	return map[int]int{}, nil
}

// textFormatIDsForNodes - TextFormatIDsByNode of many nodes in one query:
// node → field instance → its value's text format.
func textFormatIDsForNodes(ctx context.Context, client *generated.Client, nodeIDs []int, fields entity.FieldInstanceDefs) (map[int]map[int]int, error) {
	out := map[int]map[int]int{}
	instances := map[int][]int{}
	for _, f := range fields {
		if f.FieldType == entity.FieldTypeText && len(f.AllowedTextFormatIDs()) > 0 {
			instances[f.FieldStorageID] = append(instances[f.FieldStorageID], f.ID)
		}
	}
	if len(instances) == 0 || len(nodeIDs) == 0 {
		return out, nil
	}
	// The format only, not the text.
	rows, err := client.FieldValueText.Query().
		Where(fieldvaluetext.NodeIDIn(nodeIDs...), fieldvaluetext.FieldIDIn(slices.Collect(maps.Keys(instances))...), fieldvaluetext.TextFormatIDNotNil()).
		Select(fieldvaluetext.FieldNodeID, fieldvaluetext.FieldFieldID, fieldvaluetext.FieldTextFormatID).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the text formats of nodes: %w", err)
	}
	for _, row := range rows {
		if out[row.NodeID] == nil {
			out[row.NodeID] = map[int]int{}
		}
		for _, id := range instances[row.FieldID] {
			out[row.NodeID][id] = *row.TextFormatID
		}
	}
	return out, nil
}
