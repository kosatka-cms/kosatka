package form

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestFieldTypesChecked - every field type is decided for the content
// form's checks (fieldChecks).
func TestFieldTypesChecked(t *testing.T) {
	for _, ft := range entity.FieldTypes {
		if _, ok := fieldChecks[ft]; !ok {
			t.Errorf("%s: not in fieldChecks - decide how the form checks it", ft)
		}
	}
}

// TestCheckIDs - one ID, or several comma-joined for a field with
// several values.
func TestCheckIDs(t *testing.T) {
	single := entity.FieldInstanceDef{FieldType: entity.FieldTypeReference}
	several := entity.FieldInstanceDef{FieldType: entity.FieldTypeReference, Config: map[string]any{"multiple": true}}
	for _, tc := range []struct {
		field entity.FieldInstanceDef
		raw   string
		ok    bool
	}{
		{single, "", true}, {single, "5", true}, {single, "5,6", false}, {single, "x", false},
		{several, "5, 6", true}, {several, "5,x", false},
	} {
		if got := checkIDs(t.Context(), nil, tc.field, tc.raw); (got == "") != tc.ok {
			t.Errorf("%q (several %v): %q", tc.raw, tc.field.Multiple(), got)
		}
	}
}
