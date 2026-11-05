//go:build dbintegration

package migrations_test

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestUninstallModule - uninstalling a module drops its tables (a
// referenced one after the one referencing it, with data in both) and
// forgets its migrations; installing it again starts from scratch. Other
// sets are untouched; the core can't be uninstalled.
func TestUninstallModule(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		for _, set := range []string{migrations.CoreSet, "complaints", "error_reports"} {
			if _, err := r.Install(ctx, set); err != nil {
				t.Fatal(set, err)
			}
		}
		tableCount := func(names ...string) int {
			t.Helper()
			q := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?"
			if db.Driver == "pgx" {
				q = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = ?"
			}
			total := 0
			for _, name := range names {
				var n int
				if err := db.DB.QueryRow(db.Dialect.Rebind(q), name).Scan(&n); err != nil {
					t.Fatal(err)
				}
				total += n
			}
			return total
		}
		if _, err := db.DB.Exec("INSERT INTO complaint_targets (kind, object_id, node_id, last_at, created_at) VALUES ('node', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.Exec("INSERT INTO complaints (user_id, reason, created_at, target_id) SELECT 1, 'spam', CURRENT_TIMESTAMP, id FROM complaint_targets"); err != nil {
			t.Fatal(err)
		}

		if err := r.Uninstall(ctx, "complaints"); err != nil {
			t.Fatal(err)
		}
		if n := tableCount("complaints", "complaint_targets", "complaint_settings"); n != 0 {
			t.Errorf("complaints tables left: %d", n)
		}
		if ok, err := r.Installed(ctx, "complaints"); err != nil || ok {
			t.Errorf("complaints still installed: %v %v", ok, err)
		}
		if ok, _ := r.Installed(ctx, "error_reports"); !ok || tableCount("error_reports", "users") != 2 {
			t.Error("other sets must stay")
		}
		if err := r.Uninstall(ctx, "complaints"); err != nil {
			t.Errorf("second uninstall: %v", err)
		}

		if _, err := r.Install(ctx, "complaints"); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM complaints").Scan(&n); err != nil || n != 0 || tableCount("complaints", "complaint_targets", "complaint_settings") != 3 {
			t.Errorf("reinstalled complaints: %d rows, %v", n, err)
		}
		if err := r.Uninstall(ctx, migrations.CoreSet); err == nil {
			t.Error("the core was uninstalled")
		}
	})
}

// TestReinstallGalleries - the galleries' image styles stay after an
// uninstall (the site may use them), so installing again keeps them.
func TestReinstallGalleries(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		for _, set := range []string{migrations.CoreSet, "galleries"} {
			if _, err := r.Install(ctx, set); err != nil {
				t.Fatal(set, err)
			}
		}
		if err := r.Uninstall(ctx, "galleries"); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Install(ctx, "galleries"); err != nil {
			t.Fatalf("reinstall: %v", err)
		}
		var styles int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM image_styles WHERE name IN ('gallery_cover', 'gallery_thumbnail', 'gallery_medium')").Scan(&styles); err != nil || styles != 3 {
			t.Errorf("styles: %d %v", styles, err)
		}
	})
}
