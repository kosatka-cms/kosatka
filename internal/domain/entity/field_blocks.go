package entity

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
)

// BlockItem - one item of a blocks field (FieldTypeBlocks): a node of a
// field set (a content type with Embedded) whose parent is the node
// holding the field. Type - the set's machine name; Fields, Values and
// TextFormats - its fields and their values by field instance ID, as a
// DynamicContent's. ID 0 - not saved yet. Snapshot - the item as a
// revision keeps it, when read from one (NodeSnapshot.Values): its values
// by machine name, resolved with the set's fields on reverting.
type BlockItem struct {
	ID          int
	Type        string
	Fields      FieldInstanceDefs
	Values      map[int]any
	TextFormats map[int]int
	Snapshot    *NodeSnapshot
}

// blockItemSnapshot - a BlockItem as a revision keeps it: its set and
// its values by machine name (no ID: a reverted item is a new node).
type blockItemSnapshot struct {
	Type        string                     `json:"type"`
	Fields      map[string]json.RawMessage `json:"fields,omitempty"`
	TextFormats map[string]int             `json:"text_formats,omitempty"`
}

// MarshalJSON - the item as a revision keeps it (blockItemSnapshot).
func (b BlockItem) MarshalJSON() ([]byte, error) {
	if b.Snapshot != nil {
		return json.Marshal(blockItemSnapshot{Type: b.Type, Fields: b.Snapshot.Fields, TextFormats: b.Snapshot.TextFormats})
	}
	fields, formats := snapshotValues(b.Fields, b.Values, b.TextFormats)
	return json.Marshal(blockItemSnapshot{Type: b.Type, Fields: fields, TextFormats: formats})
}

// UnmarshalJSON - an item from a revision: its set and Snapshot.
func (b *BlockItem) UnmarshalJSON(raw []byte) error {
	var s blockItemSnapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	*b = BlockItem{Type: s.Type, Snapshot: &NodeSnapshot{Fields: s.Fields, TextFormats: s.TextFormats}}
	return nil
}

// BlockItems - a blocks field's value: its items in order; the items of
// a list's batch (their IDs alone, []int) without their content.
func BlockItems(v any) []BlockItem {
	switch items := v.(type) {
	case []BlockItem:
		return items
	case []int:
		out := make([]BlockItem, len(items))
		for i, id := range items {
			out[i] = BlockItem{ID: id}
		}
		return out
	}
	return nil
}

// BlockItemIDs - the saved items of a blocks field's value, in order.
func BlockItemIDs(v any) []int {
	var ids []int
	for _, item := range BlockItems(v) {
		if item.ID > 0 {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

// blockSummaryLen - at most this many characters of an item's summary.
const blockSummaryLen = 120

// Summary - what an item is in a line: its first texts (an HTML one as
// text), "-" between them, up to blockSummaryLen characters; "" - no
// text. The item node's title, the heading of its folded card in the
// form.
func (b BlockItem) Summary() string {
	var parts []string
	for _, f := range b.Fields {
		switch f.FieldType {
		case FieldTypeText, FieldTypeEmail, FieldTypePhone:
		default:
			continue
		}
		s, _ := b.Values[f.ID].(string)
		if f.RichText() {
			s = htmlsanitize.StripToText(s)
		}
		if s = strings.Join(strings.Fields(s), " "); s != "" {
			parts = append(parts, s)
		}
		if len(parts) == 2 {
			break
		}
	}
	s := strings.Join(parts, " - ")
	if utf8.RuneCountInString(s) > blockSummaryLen {
		s = string([]rune(s)[:blockSummaryLen-1]) + "…"
	}
	return s
}

// BlockTypes - FieldTypeBlocks: the field sets its items may be of
// (their machine names, storage config "block_types").
func (f FieldInstanceDef) BlockTypes() []string {
	return blockTypesOf(f.Config)
}

// BlockTypes - the storage's BlockTypes (FieldInstanceDef.BlockTypes).
func (f FieldStorageDef) BlockTypes() []string {
	return blockTypesOf(f.Config)
}

func blockTypesOf(config map[string]any) []string {
	raw, _ := config["block_types"].([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
