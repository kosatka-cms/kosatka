package admin

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

// blockFilterOption - a field of a blocks field's sets in a View's filter
// picker: "<blocks field> › <field>", its target "block:<blocks
// field>:<field>" (entity.ViewFilterDef.ViaFieldStorageID).
type blockFilterOption struct {
	Target      string
	Label       string
	MachineName string
	FieldID     int
	FieldType   entity.ContentTypeFieldType
	Operators   string
}

// blockFilterOptions - the filter picker's fields of the blocks fields
// among storages (labels - the storages' labels), by label, and the
// storages only field sets have: a node of its own never has them, so
// the pickers of the nodes' own fields leave them out.
func (c *Controller) blockFilterOptions(ctx context.Context, storages entity.FieldStorageDefs, labels map[int]string) ([]blockFilterOption, map[int]bool, error) {
	types, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		return nil, nil, err
	}
	sets, err := c.contentTypeService.FieldSets(ctx)
	if err != nil {
		return nil, nil, err
	}
	own := map[int]bool{}
	for _, t := range types {
		fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, t.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, f := range fields {
			own[f.FieldStorageID] = true
		}
	}
	setFields := map[string]entity.FieldInstanceDefs{}
	setOnly := map[int]bool{}
	for _, set := range sets {
		fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, set.ID)
		if err != nil {
			return nil, nil, err
		}
		setFields[set.MachineName] = fields.SortBySortOrder()
		for _, f := range fields {
			if !own[f.FieldStorageID] {
				setOnly[f.FieldStorageID] = true
			}
		}
	}
	var out []blockFilterOption
	for _, bs := range storages {
		if bs.FieldType != entity.FieldTypeBlocks {
			continue
		}
		for _, name := range bs.BlockTypes() {
			for _, f := range setFields[name] {
				target := fmt.Sprintf("block:%d:%d", bs.ID, f.FieldStorageID)
				if !views.FilterableFieldType(f.FieldType) || slices.ContainsFunc(out, func(o blockFilterOption) bool { return o.Target == target }) {
					continue
				}
				out = append(out, blockFilterOption{
					Target:      target,
					Label:       labels[bs.ID] + " › " + f.Label,
					MachineName: f.MachineName,
					FieldID:     f.FieldStorageID,
					FieldType:   f.FieldType,
					Operators:   operatorsAttr(views.FilterOperators(f.FieldType)),
				})
			}
		}
	}
	slices.SortFunc(out, func(a, b blockFilterOption) int {
		return strings.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label))
	})
	return out, setOnly, nil
}
