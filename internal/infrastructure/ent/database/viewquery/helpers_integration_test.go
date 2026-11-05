//go:build dbintegration

package viewquery_test

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// entClient - an ent client over the test database.
func entClient(db testdb.DB) *generated.Client {
	return generated.NewClient(generated.Driver(entsql.OpenDB(entDialect(db), db.DB)))
}

func entDialect(db testdb.DB) string {
	if db.Dialect == sqldialect.Postgres {
		return dialect.Postgres
	}
	return dialect.MySQL
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
	t.Helper()
	if _, err := db.DB.Exec(`INSERT INTO users (id, username, slug, password_hash) VALUES (1, 'alice', 'alice', '')`); err != nil {
		t.Fatal(err)
	}
}

// schemaDB - a fresh database of every server with the whole schema
// (every set's migrations), the repositories speaking its dialect.
func schemaDB(t *testing.T, fn func(t *testing.T, db testdb.DB)) {
	manifests.Register() // the modules' tables make their own sets
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		for _, s := range migrations.Sets() {
			if _, err := r.Install(context.Background(), s.Name); err != nil {
				t.Fatal(err)
			}
		}
		database.SetDialect(entDialect(db))
		t.Cleanup(func() { database.SetDialect(dialect.MySQL) })
		fn(t, db)
	})
}
