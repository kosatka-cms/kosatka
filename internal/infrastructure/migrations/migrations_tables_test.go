package migrations

import (
	"testing"

	entschema "entgo.io/ent/dialect/sql/schema"
)

// TestOwnTablesStandIns - a table of a module's own schema named as a
// core one is its stand-in: left out of the module's tables (so never
// created or dropped with the module), the foreign keys to it pointed at
// the core's table and its columns.
func TestOwnTablesStandIns(t *testing.T) {
	coreID := &entschema.Column{Name: "id", Type: 0, SchemaType: map[string]string{"mysql": "int unsigned"}}
	coreNodes := &entschema.Table{Name: "nodes", Columns: []*entschema.Column{coreID}, PrimaryKey: []*entschema.Column{coreID}}
	standInID := &entschema.Column{Name: "id"}
	standIn := &entschema.Table{Name: "nodes", Columns: []*entschema.Column{standInID}, PrimaryKey: []*entschema.Column{standInID}}
	nodeID := &entschema.Column{Name: "node_id", Nullable: true}
	authors := &entschema.Table{Name: "quotes_authors", Columns: []*entschema.Column{nodeID}}
	authors.ForeignKeys = []*entschema.ForeignKey{{Symbol: "quotes_authors_nodes_node", Columns: []*entschema.Column{nodeID}, RefTable: standIn, RefColumns: []*entschema.Column{standInID}, OnDelete: entschema.SetNull}}
	own := &entschema.Table{Name: "quotes", ForeignKeys: []*entschema.ForeignKey{{RefTable: authors, RefColumns: authors.Columns}}}

	for range 2 { // Sets() runs it every time: the same result
		got := ownTables([]*entschema.Table{standIn, authors, own}, map[string]*entschema.Table{"nodes": coreNodes})
		if len(got) != 2 || got[0] != authors || got[1] != own {
			t.Fatalf("tables: %v", got)
		}
		fk := authors.ForeignKeys[0]
		if fk.RefTable != coreNodes || fk.RefColumns[0] != coreID || fk.OnDelete != entschema.SetNull || fk.Columns[0] != nodeID {
			t.Errorf("the key to the stand-in: %+v", fk)
		}
		if own.ForeignKeys[0].RefTable != authors {
			t.Error("a key between the module's own tables changed")
		}
	}
	if len(coreNodes.Columns) != 1 || len(coreNodes.ForeignKeys) != 0 {
		t.Error("the core's table changed")
	}
}
