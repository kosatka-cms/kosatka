//go:build dbintegration

package app

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestMigrateInstallsEnabledModule - a module enabled on a site that
// doesn't have it ("app install -modules none", then switched on in the
// modules table) is installed by "app migrate -apply" as the Modules page
// installs it: its tables, then its block in the site's theme, once.
func TestMigrateInstallsEnabledModule(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSite(t, db, nil, "-modules", "none")
		const galleries, block = "galleries", "node_galleries"
		if _, err := db.DB.Exec(db.Dialect.Rebind("UPDATE modules SET enabled = ? WHERE name = ?"), true, galleries); err != nil {
			t.Fatal(err)
		}
		if out := s.app("migrate"); !strings.Contains(out, galleries+": pending") {
			t.Fatalf("the plan has no %s:\n%s", galleries, out)
		}
		if out := s.app("migrate", "-apply"); !strings.Contains(out, galleries+": installed -") {
			t.Errorf("app migrate -apply:\n%s", out)
		}
		s.app("migrate", "-apply") // nothing left to do: the block isn't placed again

		var n int
		if err := db.DB.QueryRow(db.Dialect.Rebind("SELECT COUNT(*) FROM block_placements WHERE plugin_id = ? AND region = ?"), block, "content_inline").Scan(&n); err != nil || n != 1 {
			t.Errorf("the galleries' block: %d placements %v, want 1", n, err)
		}
		if _, err := db.DB.Exec("SELECT 1 FROM galleries WHERE 1 = 0"); err != nil {
			t.Errorf("the galleries' table: %v", err)
		}
	})
}

// TestMigrateAdditive - "app migrate -apply -additive" runs what only
// adds: a module enabled on the site is installed by it (its baseline
// creates tables), nothing is left. Where it stops - the first step that
// drops or changes something - migrations.AdditivePrefix (TestAdditivePrefix).
func TestMigrateAdditive(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSite(t, db, nil, "-modules", "none")
		if _, err := db.DB.Exec(db.Dialect.Rebind("UPDATE modules SET enabled = ? WHERE name = ?"), true, "ratings"); err != nil {
			t.Fatal(err)
		}
		out := s.app("migrate", "-apply", "-additive")
		if !strings.Contains(out, "ratings: applied ") || !strings.Contains(out, "_baseline\n") || !strings.Contains(out, "ratings: installed -") || strings.Contains(out, "stopped before") {
			t.Errorf("app migrate -apply -additive:\n%s", out)
		}
		if out := s.app("migrate"); strings.Contains(out, "pending") {
			t.Errorf("left after it:\n%s", out)
		}
	})
}
