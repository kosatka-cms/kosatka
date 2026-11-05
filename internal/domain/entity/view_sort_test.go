package entity

import (
	"slices"
	"testing"
)

// TestViewSortDefsWithChoice - the exposed criteria are alternatives:
// the chosen one (else the first) takes the place of the first, the
// others keep theirs.
func TestViewSortDefsWithChoice(t *testing.T) {
	sticky := ViewSortDef{ID: 1, BuiltinField: ViewSortBuiltinSticky}
	cheap := ViewSortDef{ID: 2, ExposedKey: "cheap", ExposedLabel: "Cheapest first"}
	dear := ViewSortDef{ID: 3, ExposedKey: "dear", ExposedLabel: "Most expensive first"}
	title := ViewSortDef{ID: 4, BuiltinField: ViewSortBuiltinTitle}
	all := ViewSortDefs{sticky, cheap, dear, title}
	ids := func(ss ViewSortDefs) []int {
		var out []int
		for _, s := range ss {
			out = append(out, s.ID)
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		sorts ViewSortDefs
		key   string
		want  []int
	}{
		{"no choice - the first exposed", all, "", []int{1, 2, 4}},
		{"chosen", all, "dear", []int{1, 3, 4}},
		{"unknown key - the first exposed", all, "nonsense", []int{1, 2, 4}},
		{"none exposed - as is", ViewSortDefs{sticky, title}, "dear", []int{1, 4}},
		{"exposed first", ViewSortDefs{dear, cheap, title}, "cheap", []int{2, 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ids(tc.sorts.WithChoice(tc.key)); !slices.Equal(got, tc.want) {
				t.Errorf("WithChoice(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
	if got := ids(all.Exposed()); !slices.Equal(got, []int{2, 3}) {
		t.Errorf("Exposed() = %v, want [2 3]", got)
	}
}
