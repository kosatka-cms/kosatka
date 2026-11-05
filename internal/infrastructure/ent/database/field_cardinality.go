package database

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldinstancedef"
)

// NodesWithSeveralFieldValues - the nodes holding more than one value of a
// multi-value field (they block making it single-valued), sorted by ID.
func (r ContentTypeRepository) NodesWithSeveralFieldValues(ctx context.Context, storage entity.FieldStorageDef) ([]int, error) {
	if !storage.FieldType.AllowsMultiple() {
		return nil, fmt.Errorf("field type %q has no multiple values", storage.FieldType)
	}
	t, _ := valueTableOf(storage.FieldType, true)
	nodeIDs, err := t.nodeIDs(ctx, GetClient(ctx, r.client), valueOfField(storage.ID))
	if err != nil {
		return nil, err
	}
	counts := map[int]int{}
	for _, id := range nodeIDs {
		counts[id]++
	}
	var several []int
	for id, n := range counts {
		if n > 1 {
			several = append(several, id)
		}
	}
	sort.Ints(several)
	return several, nil
}

// SetFieldMultiple switches a field storage between one and several values
// per node: its values move to the other value table, its config gets
// "multiple" (a single-valued reference loses "symmetric"), and each
// instance's widget that the new cardinality doesn't allow is replaced
// (cardinalityWidget). The caller makes sure no node has several values
// before making a field single-valued (NodesWithSeveralFieldValues), in the
// same transaction: it writes several tables.
func (r ContentTypeRepository) SetFieldMultiple(ctx context.Context, storageID int, multiple bool) error {
	client := GetClient(ctx, r.client)
	st, err := client.FieldStorageDef.Get(ctx, storageID)
	if err != nil {
		return notFoundAs(err, "field storage with id %d", storageID)
	}
	storage := entity.FieldStorageDef{ID: st.ID, FieldType: entity.ContentTypeFieldType(st.FieldType), Config: st.Config}
	if storage.Multiple() == multiple {
		return nil
	}
	// Values in a table of their own per cardinality move to the other
	// (fieldStorage.moveValues); a link's, a date's, a user field's stay.
	if err := fieldStorages[storage.FieldType].moveValues(ctx, client, storage.ID, multiple); err != nil {
		return fmt.Errorf("moving the values: %w", err)
	}

	config := map[string]any{}
	for k, v := range st.Config {
		config[k] = v
	}
	if multiple {
		config["multiple"] = true
	} else {
		delete(config, "multiple")
		delete(config, "symmetric")
	}
	now := time.Now()
	if err := client.FieldStorageDef.UpdateOneID(storageID).SetConfig(config).SetUpdatedAt(now).Exec(ctx); err != nil {
		return fmt.Errorf("ContentTypeRepository.SetFieldMultiple: %w", err)
	}

	instances, err := client.FieldInstanceDef.Query().Where(fieldinstancedef.FieldStorageID(storageID)).All(ctx)
	if err != nil {
		return fmt.Errorf("ContentTypeRepository.SetFieldMultiple: %w", err)
	}
	for _, in := range instances {
		widget, _ := in.Config["widget"].(string)
		replacement := cardinalityWidget(entity.FieldWidget(widget), multiple)
		if string(replacement) == widget {
			continue
		}
		widgetConfig := map[string]any{}
		for k, v := range in.Config {
			widgetConfig[k] = v
		}
		widgetConfig["widget"] = string(replacement)
		if err := client.FieldInstanceDef.UpdateOneID(in.ID).SetConfig(widgetConfig).SetUpdatedAt(now).Exec(ctx); err != nil {
			return fmt.Errorf("ContentTypeRepository.SetFieldMultiple: %w", err)
		}
	}
	return nil
}

// cardinalityWidget - the widget an instance keeps after its field's
// cardinality changes: radio buttons become checkboxes and back (one
// choice ↔ several), autocomplete (several terms only) becomes a select;
// the rest are valid for both.
func cardinalityWidget(w entity.FieldWidget, multiple bool) entity.FieldWidget {
	switch {
	case multiple && w == entity.WidgetRadio:
		return entity.WidgetCheckboxes
	case !multiple && w == entity.WidgetCheckboxes:
		return entity.WidgetRadio
	case !multiple && w == entity.WidgetAutocomplete:
		return entity.WidgetSelect
	}
	return w
}
