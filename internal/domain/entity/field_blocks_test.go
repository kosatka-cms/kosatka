package entity

import (
	"encoding/json"
	"strings"
	"testing"
)

// A block in a revision: its set and its values by machine name, no ID;
// read back - its Snapshot, its values by the set's fields.
func TestBlockItemSnapshot(t *testing.T) {
	fields := FieldInstanceDefs{{ID: 3, MachineName: "field_character", FieldType: FieldTypeText}}
	raw, err := json.Marshal([]BlockItem{{ID: 9, Type: "role", Fields: fields, Values: map[int]any{3: "Stalker"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); got != `[{"type":"role","fields":{"field_character":"Stalker"}}]` {
		t.Errorf("snapshot %s", got)
	}
	cast := FieldInstanceDef{ID: 1, MachineName: "field_cast", FieldType: FieldTypeBlocks}
	values, _ := NodeSnapshot{Fields: map[string]json.RawMessage{"field_cast": raw}}.Values(FieldInstanceDefs{cast})
	items := BlockItems(values[1])
	if len(items) != 1 || items[0].ID != 0 || items[0].Type != "role" || items[0].Snapshot == nil {
		t.Fatalf("read back %+v", items)
	}
	if v, _ := items[0].Snapshot.Values(fields); v[3] != "Stalker" {
		t.Errorf("its values %v", v)
	}
}

// A block's summary: its first two texts, an HTML one as text, cut.
func TestBlockItemSummary(t *testing.T) {
	fields := FieldInstanceDefs{
		{ID: 1, FieldType: FieldTypeNumber},
		{ID: 2, FieldType: FieldTypeText},
		{ID: 3, FieldType: FieldTypeText, WidgetConfig: map[string]any{"widget": "safe_html"}},
		{ID: 4, FieldType: FieldTypeText},
	}
	b := BlockItem{Fields: fields, Values: map[int]any{1: 5.0, 2: " Stalker ", 3: "<p>The <b>guide</b></p>", 4: "more"}}
	if got := b.Summary(); got != "Stalker - The guide" {
		t.Errorf("summary %q", got)
	}
	b.Values = map[int]any{2: strings.Repeat("я", 200)}
	if got := []rune(b.Summary()); len(got) != blockSummaryLen || got[len(got)-1] != '…' {
		t.Errorf("long summary of %d runes", len(got))
	}
}
