package migrations

import (
	"testing"

	entschema "entgo.io/ent/dialect/sql/schema"
)

// TestDropOrder - a referenced table is dropped after the tables
// referencing it; self references and tables outside don't matter.
func TestDropOrder(t *testing.T) {
	core := &entschema.Table{Name: "nodes"}
	authors := &entschema.Table{Name: "authors"}
	items := &entschema.Table{Name: "items"}
	items.ForeignKeys = []*entschema.ForeignKey{{RefTable: authors}, {RefTable: core}, {RefTable: items}}
	votes := &entschema.Table{Name: "votes", ForeignKeys: []*entschema.ForeignKey{{RefTable: items}}}
	var got []string
	for _, tb := range dropOrder([]*entschema.Table{authors, items, votes}) {
		got = append(got, tb.Name)
	}
	if len(got) != 3 || got[0] != "votes" || got[1] != "items" || got[2] != "authors" {
		t.Errorf("order: %v", got)
	}
}
