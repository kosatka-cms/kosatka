package form

import (
	"context"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestUserFieldValue - a user field's hidden input: its IDs in order,
// each once, the rest left out; more users than the field holds, or no
// ID at all, refused.
func TestUserFieldValue(t *testing.T) {
	if got := ParseIDList(" 5, 3,x,5,0,7 "); !slices.Equal(got, []int{5, 3, 7}) {
		t.Errorf("ParseIDList = %v", got)
	}
	ctx := context.Background()
	one := entity.FieldInstanceDef{Label: "Lead", FieldType: entity.FieldTypeUser}
	several := entity.FieldInstanceDef{Label: "Members", FieldType: entity.FieldTypeUser,
		Config: map[string]any{"multiple": true}, WidgetConfig: map[string]any{"max_values": float64(2)}}
	for name, tc := range map[string]struct {
		field entity.FieldInstanceDef
		raw   string
		want  string
	}{
		"empty":        {one, "", ""},
		"one":          {one, "4", ""},
		"two for one":  {one, "4,5", "Lead: at most 1 person."},
		"two":          {several, "4,5", ""},
		"three of two": {several, "4,5,6", "Members: at most 2 people."},
		"not ids":      {several, "x,y", "Illegal value"},
	} {
		if got := validateUserIDs(ctx, tc.field, tc.raw); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}
