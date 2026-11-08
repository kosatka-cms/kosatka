//go:build dbintegration

package migrations_test

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestMoveResendKey - the Resend key saved in the admin is copied, as it
// is, to the Resend module's setting, its old column left as it was (an
// older binary still sends with it); a second run changes nothing, a
// database without a key or a row is left alone.
func TestMoveResendKey(t *testing.T) {
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
		env := migrations.Env{DB: db.DB, Dialect: db.Dialect}
		if err := migrations.MoveResendKey(ctx, env); err != nil {
			t.Fatalf("no settings row: %v", err)
		}
		if _, err := db.DB.ExecContext(ctx, db.Dialect.Rebind("INSERT INTO mail_settings (id, transport, from_address, resend_api_key) VALUES (?, ?, ?, ?)"), 1, "resend", "site@example.com", "sealed:re_key"); err != nil {
			t.Fatal(err)
		}
		for run := 1; run <= 2; run++ {
			if err := migrations.MoveResendKey(ctx, env); err != nil {
				t.Fatalf("run %d: %v", run, err)
			}
			var n int
			var value, old, transport string
			if err := db.DB.QueryRowContext(ctx, db.Dialect.Rebind("SELECT COUNT(*), MAX(value) FROM mail_transport_settings WHERE transport = ? AND name = ?"), "resend", "api_key").Scan(&n, &value); err != nil {
				t.Fatal(err)
			}
			if err := db.DB.QueryRowContext(ctx, "SELECT resend_api_key, transport FROM mail_settings").Scan(&old, &transport); err != nil {
				t.Fatal(err)
			}
			if n != 1 || value != "sealed:re_key" || old != "sealed:re_key" || transport != "resend" {
				t.Errorf("run %d: %d %q, old %q, transport %q", run, n, value, old, transport)
			}
		}
	})
}

// TestShowLastCommentsTitle - the latest comments block's placements
// without "Show title" get it (the block's heading is the placement's
// now); one showing it already and other system blocks are left alone.
func TestShowLastCommentsTitle(t *testing.T) {
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
		rows := []struct {
			plugin, title string
			show          bool
		}{{"last_comments", "", false}, {"last_comments", "Talked about", false}, {"last_comments", "Latest", true}, {"tag_cloud", "Tags", false}}
		for i, p := range rows {
			if _, err := db.DB.ExecContext(ctx, db.Dialect.Rebind("INSERT INTO block_placements (placement_type, plugin_id, title, show_title, region, weight, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"),
				"plugin", p.plugin, p.title, p.show, "sidebar", i, now, now); err != nil {
				t.Fatal(err)
			}
		}
		env := migrations.Env{DB: db.DB, Dialect: db.Dialect}
		for run := 1; run <= 2; run++ {
			if err := migrations.ShowLastCommentsTitle(ctx, env); err != nil {
				t.Fatalf("run %d: %v", run, err)
			}
			got, err := db.DB.QueryContext(ctx, "SELECT plugin_id, title, show_title FROM block_placements ORDER BY weight")
			if err != nil {
				t.Fatal(err)
			}
			want := []bool{true, true, true, false}
			i := 0
			for got.Next() {
				var plugin, title string
				var show bool
				if err := got.Scan(&plugin, &title, &show); err != nil {
					t.Fatal(err)
				}
				if plugin != rows[i].plugin || title != rows[i].title || show != want[i] {
					t.Errorf("run %d, row %d: %s %q %v, want show_title %v", run, i, plugin, title, show, want[i])
				}
				i++
			}
			if err := got.Err(); err != nil {
				t.Fatal(err)
			}
			if err := got.Close(); err != nil {
				t.Fatal(err)
			}
			if i != len(rows) {
				t.Fatalf("run %d: %d rows", run, i)
			}
		}
	})
}
