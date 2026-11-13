//go:build dbintegration

package migrations_test

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestMoveResendKey - on the schema the step runs on (the files before
// it), the Resend key saved in the admin is copied, as it is, to the
// Resend module's setting, its old column left as it was (an older binary
// still sends with it); a second run changes nothing, a database without
// a key or a row is left alone. The files after the step drop the column
// and keep the moved key.
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
		const stepVersion = "20261106173400"
		var later []migrations.File
		for _, f := range files {
			if f.Version > stepVersion {
				later = append(later, f)
				continue
			}
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
		for _, f := range later {
			if err := r.Apply(ctx, migrations.CoreSet, f); err != nil {
				t.Fatal(err)
			}
		}
		var value string
		if err := db.DB.QueryRowContext(ctx, db.Dialect.Rebind("SELECT value FROM mail_transport_settings WHERE transport = ? AND name = ?"), "resend", "api_key").Scan(&value); err != nil || value != "sealed:re_key" {
			t.Errorf("the moved key after the later files: %q, %v", value, err)
		}
		if _, err := db.DB.ExecContext(ctx, "SELECT resend_api_key FROM mail_settings"); err == nil {
			t.Error("the old column is still there")
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

// TestClearStarterLogo - the engine's logo, which app install used to
// save as the site's, is cleared (the theme's shows); a logo the site
// uploaded stays; a second run changes nothing.
func TestClearStarterLogo(t *testing.T) {
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
		for _, c := range []struct {
			logo, want string
		}{{"/static/images/kosatka-logo.svg", ""}, {"public://site/logo/mark.png", "public://site/logo/mark.png"}} {
			if _, err := db.DB.ExecContext(ctx, "DELETE FROM site_information"); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB.ExecContext(ctx, db.Dialect.Rebind("INSERT INTO site_information (id, logo) VALUES (?, ?)"), 1, c.logo); err != nil {
				t.Fatal(err)
			}
			for run := 1; run <= 2; run++ {
				if err := migrations.ClearStarterLogo(ctx, env); err != nil {
					t.Fatalf("%s, run %d: %v", c.logo, run, err)
				}
				var got *string
				if err := db.DB.QueryRowContext(ctx, "SELECT logo FROM site_information").Scan(&got); err != nil {
					t.Fatal(err)
				}
				if (got == nil && c.want != "") || (got != nil && *got != c.want) {
					t.Errorf("%s, run %d: %v, want %q", c.logo, run, got, c.want)
				}
			}
		}
	})
}
