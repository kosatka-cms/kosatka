//go:build dbintegration

package database_test

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/modules/manifests"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// entClient - an ent client over the test database.
func entClient(db testdb.DB) *generated.Client {
	d := dialect.MySQL
	if db.Dialect == sqldialect.Postgres {
		d = dialect.Postgres
	}
	return generated.NewClient(generated.Driver(entsql.OpenDB(d, db.DB)))
}

// seedNode inserts a published node of the type by user 1.
func seedNode(t *testing.T, db testdb.DB, id int, typ, title, slug string) {
	t.Helper()
	if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO nodes (id, type, title, slug, status, created_by) VALUES (?, ?, ?, ?, 1, 1)`), id, typ, title, slug); err != nil {
		t.Fatal(err)
	}
}

// seedUser inserts the user with id 1.
func seedUser(t *testing.T, db testdb.DB) {
	seedRow(t, db, `INSERT INTO users (id, username, slug, password_hash) VALUES (1, 'alice', 'alice', '')`)
}

// schemaDB - a fresh database of every server with the whole schema
// (every set's migrations) and the settings repositories speaking its
// dialect.
func schemaDB(t *testing.T, fn func(t *testing.T, db testdb.DB)) {
	manifests.Register() // the modules' tables make their own sets
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		for _, s := range migrations.Sets() {
			if _, err := r.Install(ctx, s.Name); err != nil {
				t.Fatal(err)
			}
		}
		d := dialect.MySQL
		if db.Dialect == sqldialect.Postgres {
			d = dialect.Postgres
		}
		database.SetDialect(d)
		t.Cleanup(func() { database.SetDialect(dialect.MySQL) })
		fn(t, db)
	})
}

// seedRow runs a statement that prepares the test's data.
func seedRow(t *testing.T, db testdb.DB, insert string) {
	t.Helper()
	if _, err := db.DB.Exec(insert); err != nil {
		t.Fatal(err)
	}
}
