package database

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// A field type's values are kept by its fieldStorage: the fieldStore of
// its value table (field_value_<…>) - or, a type allowing several values
// (entity.ContentTypeFieldType.AllowsMultiple) in a table of their own,
// one per cardinality. Each store lives in its file (field_store_<…>.go)
// and registers the types it keeps (registerFieldStorage, in init):
// reading, writing, deleting and counting values go through the registry,
// with no switch over the field types. A new field type registers its
// storage here, or the completeness tests of this layer fail.

// valueSink - where a store's read puts a row's value: put - the
// field's one value, add - appended to its slice (several values, in
// order).
type valueSink struct {
	put, add func(nodeID, storageID int, v any)
}

// fieldStore - one value table and how values are read from and written
// to it.
type fieldStore struct {
	table valueTable
	// read reads the values of the field storages (fields: storage ID →
	// one of its instances, its config) for nodeIDs, handing each to sink.
	read func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error
	// write replaces the node's values of f with v (nil - none; its text
	// format - textFormat, 0 none).
	write func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, textFormat int) error
	// taxonomy - a store of terms: FieldValuesForNodes leaves them to the
	// terms' own queries.
	taxonomy bool
}

// fieldStorage - a field type's stores: single (and the one for several
// values, multiple, nil - the same) and, with two tables, moveColumn -
// the value column a storage's values are carried over in to the other
// table on a cardinality change (SetFieldMultiple, moveValues).
type fieldStorage struct {
	single, multiple *fieldStore
	moveColumn       string
}

// moveValues carries the values of field storage storageID from s's
// single table to its multiple one (toMultiple) or back: node, field and
// moveColumn copied over (a new row's other columns - their defaults),
// then deleted where they were; none to move - nothing done.
func (s fieldStorage) moveValues(ctx context.Context, c *generated.Client, storageID int, toMultiple bool) error {
	if s.moveColumn == "" || s.multiple == nil {
		return nil
	}
	from, to := s.single.table, s.multiple.table
	if !toMultiple {
		from, to = to, from
	}
	query := fmt.Sprintf("INSERT INTO %s (node_id, field_id, %s) SELECT node_id, field_id, %s FROM %s WHERE field_id = ?",
		to.name, s.moveColumn, s.moveColumn, from.name)
	if _, err := c.ExecContext(ctx, rebind(query), storageID); err != nil {
		return fmt.Errorf("copying the values of field %d to %s: %w", storageID, to.name, err)
	}
	if _, err := from.delete(ctx, c, valueOfField(storageID)); err != nil {
		return fmt.Errorf("deleting the moved values of field %d from %s: %w", storageID, from.name, err)
	}
	return nil
}

// fieldStorages - the registry: each field type's storage.
var fieldStorages = map[entity.ContentTypeFieldType]fieldStorage{}

// registerFieldStorage - s keeps the values of types.
func registerFieldStorage(s fieldStorage, types ...entity.ContentTypeFieldType) {
	for _, t := range types {
		if _, dup := fieldStorages[t]; dup {
			panic(fmt.Sprintf("field type %q: storage registered twice", t))
		}
		fieldStorages[t] = s
	}
}

// storeOf - the store of a field type's values, for one value or
// several; false - the type keeps none (a group).
func storeOf(ft entity.ContentTypeFieldType, multiple bool) (*fieldStore, bool) {
	s, ok := fieldStorages[ft]
	if !ok {
		return nil, false
	}
	if multiple && s.multiple != nil {
		return s.multiple, true
	}
	return s.single, true
}

// fieldStoreOf - storeOf of a field instance.
func fieldStoreOf(f entity.FieldInstanceDef) (*fieldStore, bool) {
	return storeOf(f.FieldType, f.Multiple())
}

// valueTableOf - the table holding a field's values: by its type and, for
// types that allow it, whether it holds several values; false - the type
// stores no values (a group).
func valueTableOf(ft entity.ContentTypeFieldType, multiple bool) (valueTable, bool) {
	s, ok := storeOf(ft, multiple)
	if !ok {
		return valueTable{}, false
	}
	return s.table, true
}
