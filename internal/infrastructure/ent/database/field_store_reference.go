package database

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluereference"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluereferencemulti"
)

// referenceStore - field_value_reference: the referenced node's ID, one
// per node.
var referenceStore = &fieldStore{
	table: referenceValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueReference.Query().Where(fieldvaluereference.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluereference.NodeIDIn(nodeIDs...)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading reference values: %w", err)
		}
		for _, row := range rows {
			sink.put(row.NodeID, row.FieldID, row.TargetNodeID)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueReference.Delete().
			Where(fieldvaluereference.NodeID(nodeID), fieldvaluereference.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		if targetID, ok := v.(int); ok && targetID > 0 {
			return c.FieldValueReference.Create().SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetTargetNodeID(targetID).Exec(ctx)
		}
		return nil
	},
}

// referenceMultiStore - field_value_reference_multi: several referenced
// nodes' IDs ([]int) in the editor's order (delta), each once, never the
// node itself; a symmetric field (Symmetric) keeps the references back
// (syncSymmetricReferences).
var referenceMultiStore = &fieldStore{
	table: referenceMultiValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueReferenceMulti.Query().Where(fieldvaluereferencemulti.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluereferencemulti.NodeIDIn(nodeIDs...)).
			Order(generated.Asc(fieldvaluereferencemulti.FieldDelta), generated.Asc(fieldvaluereferencemulti.FieldID)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading multi-value reference values: %w", err)
		}
		for _, row := range rows {
			sink.add(row.NodeID, row.FieldID, row.TargetNodeID)
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		var oldTargets []int
		if f.Symmetric() {
			rows, err := c.FieldValueReferenceMulti.Query().
				Where(fieldvaluereferencemulti.NodeID(nodeID), fieldvaluereferencemulti.FieldIDEQ(f.FieldStorageID)).All(ctx)
			if err != nil {
				return fmt.Errorf("loading the references of field %s (node %d): %w", f.MachineName, nodeID, err)
			}
			for _, row := range rows {
				oldTargets = append(oldTargets, row.TargetNodeID)
			}
		}
		if _, err := c.FieldValueReferenceMulti.Delete().
			Where(fieldvaluereferencemulti.NodeID(nodeID), fieldvaluereferencemulti.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		targetIDs, _ := v.([]int)
		seen := make(map[int]bool, len(targetIDs))
		delta := 0
		for _, targetID := range targetIDs {
			if targetID <= 0 || targetID == nodeID || seen[targetID] {
				continue
			}
			seen[targetID] = true
			if err := c.FieldValueReferenceMulti.Create().
				SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetTargetNodeID(targetID).SetDelta(delta).Exec(ctx); err != nil {
				return err
			}
			delta++
		}
		if f.Symmetric() {
			return syncSymmetricReferences(ctx, c, f.FieldStorageID, nodeID, oldTargets, targetIDs)
		}
		return nil
	},
}

// syncSymmetricReferences keeps a symmetric (entity.FieldInstanceDef.
// Symmetric) multi-value reference field two-way after nodeID's own
// targets changed from oldTargets to newTargets: a removed target loses
// its reference back to nodeID, an added one gains it (appended after
// its existing targets).
func syncSymmetricReferences(ctx context.Context, client *generated.Client, fieldStorageID, nodeID int, oldTargets, newTargets []int) error {
	removed, added := symmetricReferenceDiff(nodeID, oldTargets, newTargets)
	for _, id := range removed {
		if _, err := client.FieldValueReferenceMulti.Delete().
			Where(
				fieldvaluereferencemulti.NodeID(id),
				fieldvaluereferencemulti.FieldIDEQ(fieldStorageID),
				fieldvaluereferencemulti.TargetNodeID(nodeID),
			).Exec(ctx); err != nil {
			return err
		}
	}
	for _, id := range added {
		exists, err := client.FieldValueReferenceMulti.Query().
			Where(
				fieldvaluereferencemulti.NodeID(id),
				fieldvaluereferencemulti.FieldIDEQ(fieldStorageID),
				fieldvaluereferencemulti.TargetNodeID(nodeID),
			).Exist(ctx)
		if err != nil {
			return fmt.Errorf("syncSymmetricReferences: %w", err)
		}
		if exists {
			continue
		}
		count, err := client.FieldValueReferenceMulti.Query().
			Where(fieldvaluereferencemulti.NodeID(id), fieldvaluereferencemulti.FieldIDEQ(fieldStorageID)).Count(ctx)
		if err != nil {
			return fmt.Errorf("syncSymmetricReferences: %w", err)
		}
		if _, err := client.FieldValueReferenceMulti.Create().
			SetNodeID(id).SetFieldID(fieldStorageID).SetTargetNodeID(nodeID).SetDelta(count).Save(ctx); err != nil {
			return err
		}
	}
	return nil
}

// symmetricReferenceDiff - which targets lost nodeID's reference
// (removed: their back-reference must go) and which gained it (added:
// they need one), in input order, ignoring self/invalid/duplicate IDs.
func symmetricReferenceDiff(nodeID int, oldTargets, newTargets []int) (removed, added []int) {
	newSet := make(map[int]bool, len(newTargets))
	for _, id := range newTargets {
		if id > 0 && id != nodeID {
			newSet[id] = true
		}
	}
	oldSet := make(map[int]bool, len(oldTargets))
	for _, id := range oldTargets {
		if oldSet[id] {
			continue
		}
		oldSet[id] = true
		if !newSet[id] {
			removed = append(removed, id)
		}
	}
	seen := make(map[int]bool, len(newTargets))
	for _, id := range newTargets {
		if newSet[id] && !oldSet[id] && !seen[id] {
			seen[id] = true
			added = append(added, id)
		}
	}
	return removed, added
}

func init() {
	registerFieldStorage(fieldStorage{single: referenceStore, multiple: referenceMultiStore, moveColumn: fieldvaluereference.FieldTargetNodeID}, entity.FieldTypeReference)
}
