//go:build dbintegration

package migrations_test

import (
	"context"
	"strings"
	"testing"

	"ariga.io/atlas/sql/migrate"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestDropRemovedTables - a table the set's files created and no ent
// schema has any more gets a DROP TABLE migration; a table ent still has
// doesn't, and once the drop is applied there is nothing more to write.
func TestDropRemovedTables(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		for _, q := range []string{`CREATE TABLE kept_table (id INT PRIMARY KEY)`, `CREATE TABLE removed_table (id INT PRIMARY KEY)`} {
			if _, err := db.DB.ExecContext(ctx, q); err != nil {
				t.Fatal(err)
			}
		}
		drv, err := migrations.AtlasDriver(db.DB, db.Dialect)
		if err != nil {
			t.Fatal(err)
		}
		dir, err := migrate.NewLocalDir(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		owned := []string{"kept_table", "removed_table"}
		ent := map[string]bool{"kept_table": true}

		dropped, err := migrations.DropRemovedTables(ctx, drv, dir, "drop_removed", owned, ent)
		if err != nil || !dropped {
			t.Fatalf("dropped %v, %v; want a migration", dropped, err)
		}
		files, err := dir.Files()
		if err != nil || len(files) != 1 {
			t.Fatalf("files %v, %v; want one", files, err)
		}
		if !strings.HasSuffix(files[0].Name(), "_drop_removed.sql") {
			t.Errorf("file name %q", files[0].Name())
		}
		sql := string(files[0].Bytes())
		if !strings.Contains(sql, "DROP TABLE") || !strings.Contains(sql, "removed_table") || strings.Contains(sql, "kept_table") {
			t.Fatalf("migration:\n%s", sql)
		}
		if _, err := dir.Checksum(); err != nil {
			t.Fatalf("atlas.sum: %v", err)
		}

		stmts, err := files[0].Stmts()
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range stmts {
			if _, err := db.DB.ExecContext(ctx, s); err != nil {
				t.Fatal(err)
			}
		}
		if dropped, err := migrations.DropRemovedTables(ctx, drv, dir, "drop_removed", owned, ent); err != nil || dropped {
			t.Fatalf("after the drop: dropped %v, %v; want nothing", dropped, err)
		}
	})
}
