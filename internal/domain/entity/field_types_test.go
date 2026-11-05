package entity

import (
	"reflect"
	"testing"
	"time"
)

// fieldTypeSamples - a field of each type with a value of it, as
// DynamicContentRepository reads it; the completeness tests of other
// layers keep their own. A type without one fails TestFieldTypesSnapshot:
// give it one and make the snapshot keep it.
var fieldTypeSamples = map[ContentTypeFieldType]struct {
	config map[string]any
	value  any
}{
	FieldTypeText:      {nil, "Whale"},
	FieldTypeNumber:    {nil, 2.5},
	FieldTypeDate:      {nil, time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)},
	FieldTypeBool:      {nil, true},
	FieldTypeTaxonomy:  {map[string]any{"multiple": true}, []int{3, 5}},
	FieldTypeReference: {nil, 7},
	FieldTypeImage:     {map[string]any{"multiple": true}, []int{11, 12}},
	FieldTypeFile:      {nil, 13},
	FieldTypeList:      {nil, "a"},
	FieldTypeLink:      {nil, []Link{{URL: "https://example.com", Title: "Example"}}},
	FieldTypeUser:      {nil, []int{4}},
	FieldTypeEmail:     {nil, "info@example.com"},
	FieldTypePhone:     {nil, "+7 495 123-45-67"},
	FieldTypeBlocks: {nil, []BlockItem{{Type: "role", Fields: FieldInstanceDefs{{ID: 9, MachineName: "field_character", FieldType: FieldTypeText}},
		Values: map[int]any{9: "Stalker"}}}},
}

// TestFieldTypesSnapshot - every field type with a value (all but group)
// is kept by a revision and read back as it was (a blocks field's items -
// their set and values, Snapshot).
func TestFieldTypesSnapshot(t *testing.T) {
	for _, ft := range FieldTypes {
		if _, ok := fieldTypeDefs[ft]; !ok {
			t.Errorf("%s: not in fieldTypeDefs", ft)
		}
		if ft == FieldTypeGroup {
			continue
		}
		sample, ok := fieldTypeSamples[ft]
		if !ok {
			t.Errorf("%s: no sample value - add one to fieldTypeSamples", ft)
			continue
		}
		f := FieldInstanceDef{ID: 1, MachineName: "field_x", FieldType: ft, Config: sample.config}
		snap := SnapshotOf(DynamicContent{Fields: FieldInstanceDefs{f}, Values: map[int]any{1: sample.value}})
		if _, ok := snap.Fields["field_x"]; !ok {
			t.Errorf("%s: the snapshot leaves the value out", ft)
			continue
		}
		got, _ := snap.Values(FieldInstanceDefs{f})
		if ft == FieldTypeBlocks {
			items := BlockItems(got[1])
			if len(items) != 1 || items[0].Type != "role" || items[0].Snapshot == nil {
				t.Errorf("%s: read back %#v", ft, got[1])
			}
			continue
		}
		if !reflect.DeepEqual(got[1], sample.value) {
			t.Errorf("%s: read back %#v, want %#v", ft, got[1], sample.value)
		}
	}
}
