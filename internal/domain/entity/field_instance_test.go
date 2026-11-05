package entity

import (
	"testing"
	"time"
)

func TestFieldInstanceDefsSortBySortOrder(t *testing.T) {
	fields := FieldInstanceDefs{
		{ID: 3, SortOrder: 10},
		{ID: 1, SortOrder: 0},
		{ID: 2, SortOrder: 0},
		{ID: 4, SortOrder: 5},
	}

	sorted := fields.SortBySortOrder()

	want := []int{1, 2, 4, 3}
	if len(sorted) != len(want) {
		t.Fatalf("got %d fields, want %d", len(sorted), len(want))
	}
	for i, id := range want {
		if sorted[i].ID != id {
			t.Errorf("position %d: got ID %d, want %d", i, sorted[i].ID, id)
		}
	}

	// Original slice must be untouched.
	if fields[0].ID != 3 {
		t.Errorf("SortBySortOrder mutated the original slice")
	}
}

func TestFieldInstanceDefConfigAccessors(t *testing.T) {
	f := FieldInstanceDef{
		Config: map[string]any{
			"vocabulary_id":       float64(12), // JSON-decoded numbers are float64
			"target_content_type": "events",
			"max_len":             float64(500),
		},
	}
	if got := f.VocabularyID(); got != 12 {
		t.Errorf("VocabularyID() = %d, want 12", got)
	}
	if got := f.TargetContentType(); got != "events" {
		t.Errorf("TargetContentType() = %q, want %q", got, "events")
	}
	if got := f.MaxLen(); got != 500 {
		t.Errorf("MaxLen() = %d, want 500", got)
	}

	empty := FieldInstanceDef{}
	if got := empty.VocabularyID(); got != 0 {
		t.Errorf("VocabularyID() on empty config = %d, want 0", got)
	}
	if got := empty.TargetContentType(); got != "" {
		t.Errorf("TargetContentType() on empty config = %q, want empty", got)
	}
	if got := empty.MaxLen(); got != 255 {
		t.Errorf("MaxLen() on empty config = %d, want default 255", got)
	}
}

func TestApplyFieldFallbacks(t *testing.T) {
	t.Run("empty value with a configured fallback is substituted", func(t *testing.T) {
		fields := FieldInstanceDefs{
			{ID: 1, MachineName: "title_alt", FallbackFieldKey: "title"},
			{ID: 2, MachineName: "title"},
		}
		values := map[int]any{1: "", 2: "Foo: Bar"}
		ApplyFieldFallbacks(fields, values)
		if values[1] != "Foo: Bar" {
			t.Fatalf("values[1] = %v, want fallback to apply", values[1])
		}
	})

	t.Run("non-empty value is left untouched even with a configured fallback", func(t *testing.T) {
		fields := FieldInstanceDefs{
			{ID: 1, MachineName: "title_alt", FallbackFieldKey: "title"},
			{ID: 2, MachineName: "title"},
		}
		values := map[int]any{1: "Baz: Qux", 2: "Foo: Bar"}
		ApplyFieldFallbacks(fields, values)
		if values[1] != "Baz: Qux" {
			t.Fatalf("values[1] = %v, want unchanged original value", values[1])
		}
	})

	t.Run("no FallbackFieldKey configured is a no-op", func(t *testing.T) {
		fields := FieldInstanceDefs{
			{ID: 1, MachineName: "title_alt"},
			{ID: 2, MachineName: "title"},
		}
		values := map[int]any{1: "", 2: "Foo: Bar"}
		ApplyFieldFallbacks(fields, values)
		if values[1] != "" {
			t.Fatalf("values[1] = %v, want still empty (no fallback configured)", values[1])
		}
	})

	t.Run("fallback key with no matching field never panics", func(t *testing.T) {
		fields := FieldInstanceDefs{
			{ID: 1, MachineName: "title_alt", FallbackFieldKey: "does_not_exist"},
		}
		values := map[int]any{1: ""}
		ApplyFieldFallbacks(fields, values)
		if values[1] != "" {
			t.Fatalf("values[1] = %v, want still empty (no such field)", values[1])
		}
	})

	t.Run("fallback target that is itself empty leaves the field empty", func(t *testing.T) {
		fields := FieldInstanceDefs{
			{ID: 1, MachineName: "title_alt", FallbackFieldKey: "title"},
			{ID: 2, MachineName: "title"},
		}
		values := map[int]any{1: "", 2: ""}
		ApplyFieldFallbacks(fields, values)
		if values[1] != "" {
			t.Fatalf("values[1] = %v, want still empty (fallback target also empty)", values[1])
		}
	})

	t.Run("works for a builtin target key, not just generic machine names", func(t *testing.T) {
		fields := FieldInstanceDefs{
			{ID: 1, MachineName: "title_alt", FallbackFieldKey: string(ViewFieldBuiltinTitle)},
			{ID: 2, MachineName: string(ViewFieldBuiltinTitle)},
		}
		values := map[int]any{1: "  ", 2: "VisionQuest"}
		ApplyFieldFallbacks(fields, values)
		if values[1] != "VisionQuest" {
			t.Fatalf("values[1] = %v, want fallback to the builtin title field", values[1])
		}
	})
}

func TestIsEmptyFieldValue(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"whitespace-only string", "   ", true},
		{"non-empty string", "x", false},
		{"zero int", 0, true},
		{"non-zero int", 5, false},
		{"zero float64", float64(0), true},
		{"non-zero float64", 1.5, false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"zero time.Time", time.Time{}, true},
		{"non-zero time.Time", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyFieldValue(tt.v); got != tt.want {
				t.Errorf("isEmptyFieldValue(%#v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

// TestListItems - a list field's chosen options: their labels and places
// among the options, a value no longer among them as it is (place 0);
// printed - the labels joined.
func TestListItems(t *testing.T) {
	f := FieldInstanceDef{FieldType: FieldTypeList, Config: map[string]any{"allowed_values": []any{
		map[string]any{"value": "great", "label": "✓ Great"}, map[string]any{"value": "miss", "label": "✕ Miss"},
	}}}
	got := f.ListItems([]string{"miss", "great", "gone"})
	want := ListItems{{"miss", "✕ Miss", 2}, {"great", "✓ Great", 1}, {"gone", "gone", 0}}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %+v, want %+v", i, got[i], want[i])
		}
	}
	if s := got.String(); s != "✕ Miss, ✓ Great, gone" {
		t.Errorf("printed: %q", s)
	}
	if one := f.ListItems("great"); len(one) != 1 || one[0].Index != 1 {
		t.Errorf("one value: %v", one)
	}
	if len(f.ListItems("")) != 0 || len(f.ListItems(nil)) != 0 {
		t.Error("no value: items")
	}
}
