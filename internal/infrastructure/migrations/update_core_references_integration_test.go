//go:build dbintegration

package migrations_test

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestKeepNodePageReferences - a content type's reference field shown on
// its page loses "Show on the page" (the page didn't show it before); a
// field set's (a block's items: shown there always), a field of another
// type and one not shown stay as they were; a second run changes
// nothing.
func TestKeepNodePageReferences(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		if err := r.EnsureTable(ctx); err != nil {
			t.Fatal(err)
		}
		files, err := migrations.Files(db.Dialect, migrations.CoreSet)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if err := r.Apply(ctx, migrations.CoreSet, f); err != nil {
				t.Fatal(err)
			}
		}
		now := time.Now().UTC().Truncate(time.Second)
		exec := func(query string, args ...any) {
			t.Helper()
			if _, err := db.DB.ExecContext(ctx, db.Dialect.Rebind(query), args...); err != nil {
				t.Fatal(err)
			}
		}
		exec("INSERT INTO content_types (id, machine_name, label, embedded, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)", 1, "project", "Project", false, 1, now, now)
		exec("INSERT INTO content_types (id, machine_name, label, embedded, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)", 2, "role", "Role", true, 1, now, now)
		exec("INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?)", 1, "field_products", "reference", now, now)
		exec("INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?)", 2, "field_client", "text", now, now)
		rows := []struct {
			typ, storage int
			shown, want  bool
		}{{1, 1, true, false}, {2, 1, true, true}, {1, 2, true, true}}
		for i, row := range rows {
			exec("INSERT INTO field_instances (id, content_type_id, field_storage_id, label, show_on_detail, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)", i+1, row.typ, row.storage, "F", row.shown, now, now)
		}
		env := migrations.Env{DB: db.DB, Dialect: db.Dialect}
		for run := 1; run <= 2; run++ {
			if err := migrations.KeepNodePageReferences(ctx, env); err != nil {
				t.Fatalf("run %d: %v", run, err)
			}
			for i, row := range rows {
				var shown bool
				if err := db.DB.QueryRowContext(ctx, db.Dialect.Rebind("SELECT show_on_detail FROM field_instances WHERE id = ?"), i+1).Scan(&shown); err != nil {
					t.Fatal(err)
				}
				if shown != row.want {
					t.Errorf("run %d, field %d: shown %v, want %v", run, i+1, shown, row.want)
				}
			}
		}
	})
}
