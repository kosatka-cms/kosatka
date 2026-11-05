package database

import (
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func TestSymmetricReferenceDiff(t *testing.T) {
	removed, added := symmetricReferenceDiff(10, []int{1, 2, 3}, []int{3, 4, 10, 4, 0, 5})
	if !slices.Equal(removed, []int{1, 2}) {
		t.Fatalf("removed = %v, want [1 2]", removed)
	}
	if !slices.Equal(added, []int{4, 5}) {
		t.Fatalf("added = %v, want [4 5] (self, zero and duplicates ignored)", added)
	}

	removed, added = symmetricReferenceDiff(10, nil, nil)
	if removed != nil || added != nil {
		t.Fatalf("empty diff expected, got %v %v", removed, added)
	}
}

// TestFieldTypesHaveValueTables - every field type with a value (all but
// group) has its storage (registerFieldStorage), for one value and,
// where it allows them, several: a store reading and writing its values;
// and two stores - the column moved between them (moveValues).
func TestFieldTypesHaveValueTables(t *testing.T) {
	for _, ft := range entity.FieldTypes {
		s, ok := fieldStorages[ft]
		if ft == entity.FieldTypeGroup {
			if ok {
				t.Errorf("group: has a storage")
			}
			continue
		}
		if !ok || s.single == nil || s.single.read == nil || s.single.write == nil {
			t.Errorf("%s: no storage reading and writing its values", ft)
			continue
		}
		if s.multiple != nil && (!ft.AllowsMultiple() || s.moveColumn == "" || s.multiple.read == nil || s.multiple.write == nil) {
			t.Errorf("%s: a store for several values, but not one it takes, reads, writes and moves to", ft)
		}
	}
}
