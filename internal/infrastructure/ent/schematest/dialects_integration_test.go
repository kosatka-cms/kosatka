//go:build dbintegration

package schematest

import (
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestEntSchemaCreates - every ent schema (the core's, the modules' own)
// creates its tables on MySQL and PostgreSQL (types given only for MySQL fall back to ent's defaults on
// PostgreSQL).
func TestEntSchemaCreates(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		d := dialect.MySQL
		if db.Dialect == sqldialect.Postgres {
			d = dialect.Postgres
		}
		want := createAll(t, d, entsql.OpenDB(d, db.DB))
		q := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()"
		if db.Dialect == sqldialect.Postgres {
			q = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'"
		}
		var n int
		if err := db.DB.QueryRow(q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("%d tables created, ent describes %d", n, want)
		}
	})
}
