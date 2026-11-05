package entity

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

// revisionFields - one field of every value type a snapshot keeps.
func revisionFields() FieldInstanceDefs {
	multi := map[string]any{"multiple": true}
	return FieldInstanceDefs{
		{ID: 1, MachineName: "body", FieldType: FieldTypeText},
		{ID: 2, MachineName: "rating", FieldType: FieldTypeNumber},
		{ID: 3, MachineName: "released", FieldType: FieldTypeDate},
		{ID: 4, MachineName: "featured", FieldType: FieldTypeBool},
		{ID: 5, MachineName: "kind", FieldType: FieldTypeList},
		{ID: 6, MachineName: "colours", FieldType: FieldTypeList, Config: multi},
		{ID: 7, MachineName: "cover", FieldType: FieldTypeImage},
		{ID: 8, MachineName: "tags", FieldType: FieldTypeTaxonomy, Config: multi},
		{ID: 9, MachineName: "related", FieldType: FieldTypeReference},
	}
}

// TestNodeSnapshotValues - a node's values come back from its snapshot
// as DynamicContentRepository reads them, through the stored form; an
// empty value is left out.
func TestNodeSnapshotValues(t *testing.T) {
	fields := revisionFields()
	if !fields[5].Multiple() || !fields[7].Multiple() {
		t.Fatal("the fixture's multi-value fields aren't multiple")
	}
	released := time.Date(2026, 10, 24, 12, 30, 0, 0, time.UTC)
	values := map[int]any{
		1: "<p>Text</p>", 2: 4.5, 3: released, 4: true, 5: "film",
		6: []string{"red", "blue"}, 7: 12, 8: []int{3, 1, 2}, 9: 40,
	}
	dc := DynamicContent{
		Node:        Node{Title: "Title", Status: 1, CreatedBy: User{ID: 7}, PageSEO: PageSEO{Title: "Meta"}},
		Fields:      fields,
		Values:      values,
		TextFormats: map[int]int{1: 2},
	}
	stored, err := SnapshotOf(dc).Encode()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := DecodeNodeSnapshot(stored)
	if err != nil {
		t.Fatal(err)
	}
	gotValues, gotFormats := snap.Values(fields)
	if !reflect.DeepEqual(gotValues, values) {
		t.Errorf("values %#v, want %#v", gotValues, values)
	}
	if !reflect.DeepEqual(gotFormats, map[int]int{1: 2}) {
		t.Errorf("text formats %v", gotFormats)
	}
	if snap.Title != "Title" || snap.Status != 1 || snap.AuthorID != 7 || snap.MetaTitle != "Meta" {
		t.Errorf("the node's own parts: %+v", snap)
	}

	dc.Values = map[int]any{1: "", 6: []string{}, 8: []int(nil)}
	if got := SnapshotOf(dc).Fields; len(got) != 0 {
		t.Errorf("empty values kept: %v", got)
	}
}

// TestNodeSnapshotDiff - what differs: the node's own parts first, then
// the fields by name; a date in another location, or a text differing in
// white space only, is the same.
func TestNodeSnapshotDiff(t *testing.T) {
	fields := revisionFields()
	at := time.Date(2026, 10, 24, 12, 30, 0, 0, time.UTC)
	base := DynamicContent{
		Node:   Node{Title: "A", Status: 1},
		Fields: fields,
		Values: map[int]any{1: "<p>One</p><p>Two</p>", 3: at, 8: []int{1, 2}},
	}
	old := SnapshotOf(base)

	same := base
	same.Values = map[int]any{1: "<p>One</p>\n<p>Two</p>\n", 3: at.In(time.FixedZone("MSK", 3*3600)), 8: []int{1, 2}}
	if got := old.Diff(SnapshotOf(same)); len(got) != 0 {
		t.Errorf("white space and location: %v", got)
	}

	changed := base
	changed.Node.Title, changed.Node.Status = "B", 0
	changed.Values = map[int]any{1: "<p>One</p><p>Three</p>", 8: []int{2, 1}, 9: 5}
	want := []string{SnapshotTitle, SnapshotStatus, SnapshotFieldKey("body"), SnapshotFieldKey("related"), SnapshotFieldKey("released"), SnapshotFieldKey("tags")}
	if got := old.Diff(SnapshotOf(changed)); !slices.Equal(got, want) {
		t.Errorf("diff %v, want %v", got, want)
	}
}
