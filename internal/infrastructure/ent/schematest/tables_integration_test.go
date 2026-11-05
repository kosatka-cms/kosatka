//go:build dbintegration

package schematest

import (
	"context"
	"testing"

	entsql "entgo.io/ent/dialect/sql"
	entschema "entgo.io/ent/dialect/sql/schema"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
)

// allTables - every ent table: the core schema's and the modules' own
// (migrations.Sets).
func allTables() []*entschema.Table {
	manifests.Register()
	var out []*entschema.Table
	for _, s := range migrations.Sets() {
		out = append(out, s.Tables...)
	}
	return out
}

// createAll creates every ent table (allTables) on db in dialect d.
func createAll(t *testing.T, d string, db *entsql.Driver) int {
	t.Helper()
	m, err := entschema.NewMigrate(db)
	if err != nil {
		t.Fatal(err)
	}
	tables := allTables()
	if err := m.Create(context.Background(), tables...); err != nil {
		t.Fatalf("schema create (%s): %v", d, err)
	}
	return len(tables)
}
