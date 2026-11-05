package database

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluedate"
)

// dateStore - field_value_date, whatever the cardinality: a date
// (time.Time), one with a time or an end (entity.DateValue, a field with
// DateRich) or several ([]entity.DateValue, DateList, in the editor's
// order - delta).
var dateStore = &fieldStore{
	table: dateValues,
	read: func(ctx context.Context, c *generated.Client, fields map[int]entity.FieldInstanceDef, nodeIDs []int, sink valueSink) error {
		rows, err := c.FieldValueDate.Query().Where(fieldvaluedate.FieldIDIn(slices.Collect(maps.Keys(fields))...), fieldvaluedate.NodeIDIn(nodeIDs...)).
			Order(generated.Asc(fieldvaluedate.FieldDelta), generated.Asc(fieldvaluedate.FieldID)).All(ctx)
		if err != nil {
			return fmt.Errorf("loading date values: %w", err)
		}
		for _, row := range rows {
			d := entity.DateValue{Start: row.Value, WithTime: row.WithTime}
			if row.EndValue != nil {
				d.End = *row.EndValue
			}
			switch f := fields[row.FieldID]; {
			case f.DateList():
				sink.add(row.NodeID, row.FieldID, d)
			case f.DateRich():
				sink.put(row.NodeID, row.FieldID, d)
			default:
				sink.put(row.NodeID, row.FieldID, row.Value)
			}
		}
		return nil
	},
	write: func(ctx context.Context, c *generated.Client, nodeID int, f entity.FieldInstanceDef, v any, _ int) error {
		if _, err := c.FieldValueDate.Delete().
			Where(fieldvaluedate.NodeID(nodeID), fieldvaluedate.FieldIDEQ(f.FieldStorageID)).Exec(ctx); err != nil {
			return err
		}
		// A field with one value keeps the first.
		dates := entity.DateValues(v)
		if !f.Multiple() && len(dates) > 1 {
			dates = dates[:1]
		}
		delta := 0
		for _, d := range dates {
			if d.Start.IsZero() {
				continue
			}
			create := c.FieldValueDate.Create().
				SetNodeID(nodeID).SetFieldID(f.FieldStorageID).SetValue(d.Start).SetWithTime(d.WithTime).SetDelta(delta)
			if d.HasEnd() {
				create = create.SetEndValue(d.End)
			}
			if _, err := create.Save(ctx); err != nil {
				return err
			}
			delta++
		}
		return nil
	},
}

func init() {
	registerFieldStorage(fieldStorage{single: dateStore}, entity.FieldTypeDate)
}
