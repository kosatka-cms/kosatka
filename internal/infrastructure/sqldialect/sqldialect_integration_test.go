//go:build dbintegration

package sqldialect_test

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestHelpersOnBothDatabases - the helpers' SQL runs on MySQL and
// PostgreSQL alike.
func TestHelpersOnBothDatabases(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		d := db.Dialect
		ddl := "CREATE TABLE items (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(64) NOT NULL UNIQUE, enabled BOOLEAN NOT NULL)"
		if d == sqldialect.Postgres {
			ddl = "CREATE TABLE items (id SERIAL PRIMARY KEY, name VARCHAR(64) NOT NULL UNIQUE, enabled BOOLEAN NOT NULL)"
		}
		if _, err := db.DB.ExecContext(ctx, ddl); err != nil {
			t.Fatal(err)
		}

		id, err := d.InsertID(ctx, db.DB, "INSERT INTO items (name, enabled) VALUES (?, ?)", "first", true)
		if err != nil || id != 1 {
			t.Fatalf("InsertID: %d %v", id, err)
		}

		upsert := d.Rebind(d.Upsert("items", []string{"name", "enabled"}, []string{"name"}, []string{"enabled"}))
		for _, enabled := range []bool{false, true, false} {
			if _, err := db.DB.ExecContext(ctx, upsert, "second", enabled); err != nil {
				t.Fatalf("Upsert: %v", err)
			}
		}
		keep := d.Rebind(d.Upsert("items", []string{"name", "enabled"}, []string{"name"}, nil))
		if _, err := db.DB.ExecContext(ctx, keep, "first", false); err != nil {
			t.Fatalf("Upsert without updates: %v", err)
		}

		var count int
		var secondEnabled, firstEnabled bool
		if err := db.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&count); err != nil || count != 2 {
			t.Fatalf("rows: %d %v", count, err)
		}
		if err := db.DB.QueryRowContext(ctx, d.Rebind("SELECT enabled FROM items WHERE name = ?"), "second").Scan(&secondEnabled); err != nil || secondEnabled {
			t.Errorf("upsert didn't update: %v %v", secondEnabled, err)
		}
		if err := db.DB.QueryRowContext(ctx, d.Rebind("SELECT enabled FROM items WHERE name = ?"), "first").Scan(&firstEnabled); err != nil || !firstEnabled {
			t.Errorf("upsert without updates changed the row: %v %v", firstEnabled, err)
		}

		var name string
		if err := db.DB.QueryRowContext(ctx, "SELECT name FROM items ORDER BY "+d.Random()+" LIMIT 1").Scan(&name); err != nil {
			t.Errorf("Random: %v", err)
		}
		var today string
		if err := db.DB.QueryRowContext(ctx, "SELECT CAST("+d.CurrentDate()+" AS CHAR(10))").Scan(&today); err != nil {
			t.Errorf("CurrentDate: %v", err)
		}
	})
}
