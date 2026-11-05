package entity

import "testing"

// GetAltTitle reads the field the type names in AltTitleField, not a
// fixed machine name.
func TestDynamicContentGetAltTitle(t *testing.T) {
	dc := DynamicContent{
		Fields: FieldInstanceDefs{{ID: 1, MachineName: "title_alt"}, {ID: 2, MachineName: "original_name"}},
		Values: map[int]any{1: "Alt Foo", 2: "Foo"},
	}
	if got := dc.GetAltTitle(); got != "" {
		t.Fatalf("no AltTitleField: got %q", got)
	}
	dc.ContentType.AltTitleField = "original_name"
	if got := dc.GetAltTitle(); got != "Foo" {
		t.Fatalf("AltTitleField=original_name: got %q", got)
	}
	dc.ContentType.AltTitleField = "missing"
	if got := dc.GetAltTitle(); got != "" {
		t.Fatalf("missing field: got %q", got)
	}
}
