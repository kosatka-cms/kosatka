//go:build dbintegration

package schematest

import (
	"os"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestDumpSchema writes the MySQL schema ent creates - the core's and the
// modules' own (columns, indexes, foreign keys) - to $SCHEMA_DUMP, for
// scripts/schema_drift.sh to compare with a live database. Skipped
// without SCHEMA_DUMP.
func TestDumpSchema(t *testing.T) {
	out := os.Getenv("SCHEMA_DUMP")
	if out == "" {
		t.Skip()
	}
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		if db.Driver != "mysql" {
			return
		}
		createAll(t, dialect.MySQL, entsql.OpenDB(dialect.MySQL, db.DB))
		f, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		dump(t, db, f)
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	})
}
