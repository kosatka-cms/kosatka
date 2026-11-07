//go:build dbintegration

package migrations_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestApplyAllSets - the embedded migrations of every set create the whole
// schema on an empty MySQL and PostgreSQL database, are recorded, and a
// second run has nothing to do; on PostgreSQL, which doesn't index a
// foreign key itself, each has an index starting with its columns.
func TestApplyAllSets(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		if err := r.EnsureTable(ctx); err != nil {
			t.Fatal(err)
		}
		run := func() int {
			n := 0
			for _, s := range migrations.Sets() {
				files, err := migrations.Files(db.Dialect, s.Name)
				if err != nil {
					t.Fatal(err)
				}
				pending, err := r.Pending(ctx, s.Name, files)
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range pending {
					if err := r.Apply(ctx, s.Name, f); err != nil {
						t.Fatal(err)
					}
					n++
				}
			}
			return n
		}
		if n := run(); n == 0 {
			t.Fatal("nothing applied on an empty database")
		}
		if n := run(); n != 0 {
			t.Errorf("second run applied %d", n)
		}
		q := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()"
		if db.Driver == "pgx" {
			q = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'"
		}
		var tables int
		if err := db.DB.QueryRow(q).Scan(&tables); err != nil {
			t.Fatal(err)
		}
		want := 1 // schema_migrations
		for _, s := range migrations.Sets() {
			want += len(s.Tables)
		}
		if tables != want {
			t.Errorf("%d tables, want %d", tables, want)
		}
		if db.Driver == "pgx" {
			rows, err := db.DB.QueryContext(ctx, `SELECT c.conrelid::regclass::text, c.conname FROM pg_constraint c
				WHERE c.contype = 'f' AND NOT EXISTS (SELECT 1 FROM pg_index i WHERE i.indrelid = c.conrelid
					AND (i.indkey::int2[])[0:array_length(c.conkey, 1) - 1] @> c.conkey
					AND (i.indkey::int2[])[0:array_length(c.conkey, 1) - 1] <@ c.conkey)`)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var table, fk string
				if err := rows.Scan(&table, &fk); err != nil {
					t.Fatal(err)
				}
				t.Errorf("%s: foreign key %s has no index", table, fk)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// TestBaselineRecordsWithoutRunning - MarkApplied records a file without
// running it.
func TestBaselineRecordsWithoutRunning(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		if err := r.EnsureTable(ctx); err != nil {
			t.Fatal(err)
		}
		files, err := migrations.Files(db.Dialect, migrations.CoreSet)
		if err != nil || len(files) == 0 {
			t.Fatalf("core files: %d %v", len(files), err)
		}
		for _, f := range files {
			if err := r.MarkApplied(ctx, migrations.CoreSet, f); err != nil {
				t.Fatal(err)
			}
		}
		pending, err := r.Pending(ctx, migrations.CoreSet, files)
		if err != nil || len(pending) != 0 {
			t.Errorf("pending after baseline: %d %v", len(pending), err)
		}
		var n int
		q := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?"
		if db.Driver == "pgx" {
			q = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = ?"
		}
		if err := db.DB.QueryRow(db.Dialect.Rebind(q), "users").Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Error("baseline ran the migration")
		}
	})
}

// byteWise - the tables compared byte by byte on purpose: the search
// index's words (schema.SearchWord), as in PostgreSQL.
var byteWise = map[string]bool{"search_words": true}

// TestMySQLCaseInsensitive - on MySQL every table and text column the
// migrations create compares case-insensitively (utf8mb4_unicode_ci, not
// ent's default utf8mb4_bin), as a site's MySQL database does - but the
// search index's (byteWise).
func TestMySQLCaseInsensitive(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		if db.Dialect != sqldialect.MySQL {
			t.Skip("MySQL only")
		}
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		for _, s := range migrations.Sets() {
			if _, err := r.Install(ctx, s.Name); err != nil {
				t.Fatal(s.Name, err)
			}
		}
		rows, err := db.DB.Query(`SELECT CONCAT(table_name, '.', column_name), collation_name FROM information_schema.columns
			WHERE table_schema = DATABASE() AND collation_name IS NOT NULL AND collation_name <> 'utf8mb4_unicode_ci'
			UNION ALL SELECT table_name, table_collation FROM information_schema.tables
			WHERE table_schema = DATABASE() AND table_collation <> 'utf8mb4_unicode_ci'`)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var name, collation string
			if err := rows.Scan(&name, &collation); err != nil {
				t.Fatal(err)
			}
			if table, _, _ := strings.Cut(name, "."); byteWise[table] && collation == "utf8mb4_bin" {
				continue
			}
			t.Errorf("%s: %s", name, collation)
		}
		var n int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE set_name = 'CORE'`).Scan(&n); err != nil || n == 0 {
			t.Errorf("a case-insensitive lookup found %d rows: %v", n, err)
		}
	})
}

// TestLastApplied - the set's step applied last, with its name and when
// (Unix seconds: right whatever the session's time zone); of the steps of
// one second, the latest version, not the install step's record; none -
// false.
func TestLastApplied(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		if err := r.EnsureTable(ctx); err != nil {
			t.Fatal(err)
		}
		if _, ok, err := r.LastApplied(ctx, "lastset"); ok || err != nil {
			t.Fatalf("nothing recorded: %v %v", ok, err)
		}
		for _, f := range []migrations.File{{Version: "20260101000000", Description: "baseline"}, {Version: migrations.InstallVersion, Description: "install"}, {Version: "20260201000000", Description: "data_step"}} {
			if err := r.MarkApplied(ctx, "lastset", f); err != nil {
				t.Fatal(err)
			}
		}
		last, ok, err := r.LastApplied(ctx, "lastset")
		if err != nil || !ok || last.Version != "20260201000000" || last.Description != "data_step" {
			t.Fatalf("last = %+v %v %v", last, ok, err)
		}
		if d := time.Since(last.AppliedAt); d < -time.Minute || d > time.Minute {
			t.Errorf("applied at %v, now %v", last.AppliedAt, time.Now())
		}
		if _, err := db.DB.ExecContext(ctx, db.Dialect.Rebind("UPDATE schema_migrations SET applied_at = ? WHERE set_name = ? AND version = ?"), time.Now().Add(time.Hour).UTC().Format("2006-01-02 15:04:05"), "lastset", migrations.InstallVersion); err != nil {
			t.Fatal(err)
		}
		if last, _, _ := r.LastApplied(ctx, "lastset"); last.Version != migrations.InstallVersion {
			t.Errorf("the one applied later: %+v", last)
		}
	})
}
