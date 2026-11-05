package database

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluedate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluenumber"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomy"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomymulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetext"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// FieldValuesForNodes - the values of fields for each of nodeIDs (node →
// field instance ID → value), one query per value table over all the
// fields and nodes: text, number, date (a time.Time, or an
// entity.DateValue for a field with a time or an end), bool, image and file (the file ID -
// the caller resolves files in one batch), list and reference (one
// value or, Multiple(), a slice in the order saved), link ([]entity.Link
// in the order saved, whatever the cardinality), user ([]int user IDs,
// likewise), blocks ([]int item IDs, likewise). Taxonomy fields go
// through FieldTaxonomy(Multi)ValuesForNodes; other types are skipped.
func (r ContentTypeRepository) FieldValuesForNodes(ctx context.Context, nodeIDs []int, fields entity.FieldInstanceDefs) (map[int]map[int]any, error) {
	return r.fieldValues(ctx, nodeIDs, fields, false)
}

// fieldValues - FieldValuesForNodes, taxonomy fields too when
// withTaxonomy (FieldValues: a term ID, or the term IDs in the order
// saved). One query per value table the fields use: each through its
// store (fieldStoreOf).
func (r ContentTypeRepository) fieldValues(ctx context.Context, nodeIDs []int, fields entity.FieldInstanceDefs, withTaxonomy bool) (map[int]map[int]any, error) {
	client := GetClient(ctx, r.client)
	values := make(map[int]map[int]any, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		values[nodeID] = make(map[int]any)
	}
	if len(nodeIDs) == 0 {
		return values, nil
	}
	// store → field storage → the instances reading it (keyed by their
	// ID), and one of them (the storage's config: a date's parts).
	instances := map[*fieldStore]map[int][]int{}
	storageFields := map[*fieldStore]map[int]entity.FieldInstanceDef{}
	for _, f := range fields {
		store, ok := fieldStoreOf(f)
		if !ok || (!withTaxonomy && store.taxonomy) {
			continue
		}
		if instances[store] == nil {
			instances[store], storageFields[store] = map[int][]int{}, map[int]entity.FieldInstanceDef{}
		}
		instances[store][f.FieldStorageID] = append(instances[store][f.FieldStorageID], f.ID)
		storageFields[store][f.FieldStorageID] = f
	}
	for store, byStorage := range instances {
		sink := valueSink{
			put: func(nodeID, storageID int, v any) {
				if values[nodeID] == nil {
					values[nodeID] = make(map[int]any)
				}
				for _, id := range byStorage[storageID] {
					values[nodeID][id] = v
				}
			},
			add: func(nodeID, storageID int, v any) {
				if values[nodeID] == nil {
					values[nodeID] = make(map[int]any)
				}
				for _, id := range byStorage[storageID] {
					values[nodeID][id] = appendValue(values[nodeID][id], v)
				}
			},
		}
		if err := store.read(ctx, client, storageFields[store], nodeIDs, sink); err != nil {
			return nil, err
		}
	}
	return values, nil
}

// appendValue - v appended to list, a slice of v's type (nil - a new one).
func appendValue(list, v any) any {
	switch v := v.(type) {
	case string:
		l, _ := list.([]string)
		return append(l, v)
	case int:
		l, _ := list.([]int)
		return append(l, v)
	case entity.Link:
		l, _ := list.([]entity.Link)
		return append(l, v)
	case entity.DateValue:
		l, _ := list.([]entity.DateValue)
		return append(l, v)
	}
	return list
}

// FieldTextValuesForNodes - see
// entity.ContentTypeRepository.FieldTextValuesForNodes.
func (r ContentTypeRepository) FieldTextValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]string, error) {
	if len(nodeIDs) == 0 {
		return map[int]string{}, nil
	}
	client := GetClient(ctx, r.client)
	rows, err := client.FieldValueText.Query().
		Where(fieldvaluetext.FieldIDEQ(fieldStorageID), fieldvaluetext.NodeIDIn(nodeIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.FieldTextValuesForNodes: %w", err)
	}
	result := make(map[int]string, len(rows))
	for _, row := range rows {
		result[row.NodeID] = row.Value
	}
	return result, nil
}

// FieldNumberValuesForNodes - a number field's value per node, for the
// nodes among nodeIDs that have one (see
// entity.ContentTypeRepository.FieldTextValuesForNodes).
func (r ContentTypeRepository) FieldNumberValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]float64, error) {
	if len(nodeIDs) == 0 {
		return map[int]float64{}, nil
	}
	client := GetClient(ctx, r.client)
	rows, err := client.FieldValueNumber.Query().
		Where(fieldvaluenumber.FieldIDEQ(fieldStorageID), fieldvaluenumber.NodeIDIn(nodeIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.FieldNumberValuesForNodes: %w", err)
	}
	result := make(map[int]float64, len(rows))
	for _, row := range rows {
		if row.Value != nil {
			result[row.NodeID] = *row.Value
		}
	}
	return result, nil
}

// FieldTaxonomyValuesForNodes - see
// entity.ContentTypeRepository.FieldTaxonomyValuesForNodes.
func (r ContentTypeRepository) FieldTaxonomyValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]int, error) {
	if len(nodeIDs) == 0 {
		return map[int]int{}, nil
	}
	client := GetClient(ctx, r.client)
	rows, err := client.FieldValueTaxonomy.Query().
		Where(fieldvaluetaxonomy.FieldIDEQ(fieldStorageID), fieldvaluetaxonomy.NodeIDIn(nodeIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.FieldTaxonomyValuesForNodes: %w", err)
	}
	result := make(map[int]int, len(rows))
	for _, row := range rows {
		result[row.NodeID] = row.TermID
	}
	return result, nil
}

// FieldTaxonomyMultiValuesForNodes is FieldTaxonomyValuesForNodes for a
// Multiple() taxonomy field: node_id -> all its term IDs, read from
// field_value_taxonomy_multi. Used by hub routes that match a node against
// several term IDs and by detail-page rendering of multi-value fields.
func (r ContentTypeRepository) FieldTaxonomyMultiValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int][]int, error) {
	if len(nodeIDs) == 0 {
		return map[int][]int{}, nil
	}
	client := GetClient(ctx, r.client)
	rows, err := client.FieldValueTaxonomyMulti.Query().
		Where(fieldvaluetaxonomymulti.FieldIDEQ(fieldStorageID), fieldvaluetaxonomymulti.NodeIDIn(nodeIDs...)).
		Order(generated.Asc(fieldvaluetaxonomymulti.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.FieldTaxonomyMultiValuesForNodes: %w", err)
	}
	result := make(map[int][]int, len(nodeIDs))
	for _, row := range rows {
		result[row.NodeID] = append(result[row.NodeID], row.TermID)
	}
	return result, nil
}

// FieldDateValuesForNodes - a date field's start per node; a node with
// several dates - the nearest not ended yet (by now in the site's zone),
// all ended - the latest: what a View sorts by.
func (r ContentTypeRepository) FieldDateValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]time.Time, error) {
	if len(nodeIDs) == 0 {
		return map[int]time.Time{}, nil
	}
	client := GetClient(ctx, r.client)
	rows, err := client.FieldValueDate.Query().
		Where(fieldvaluedate.FieldIDEQ(fieldStorageID), fieldvaluedate.NodeIDIn(nodeIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.FieldDateValuesForNodes: %w", err)
	}
	now, zone := time.Now(), sitetime.Location()
	result := make(map[int]time.Time, len(rows))
	upcoming := make(map[int]bool, len(rows))
	for _, row := range rows {
		d := entity.DateValue{Start: row.Value, WithTime: row.WithTime}
		if row.EndValue != nil {
			d.End = *row.EndValue
		}
		open := !d.Ended(now, zone)
		cur, seen := result[row.NodeID]
		switch {
		case !seen,
			open && !upcoming[row.NodeID],
			open && d.Start.Before(cur),
			!open && !upcoming[row.NodeID] && d.Start.After(cur):
			result[row.NodeID] = d.Start
			upcoming[row.NodeID] = upcoming[row.NodeID] || open
		}
	}
	return result, nil
}
