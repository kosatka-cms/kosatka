//go:build dbintegration

package migrations_test

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestKeepLiveViewsLive - a published display whose published version
// showed nowhere (a block, a comment View's page, the front page's View
// even as a page) loses it; a content View's page keeps its, and a display
// never published stays so; a second run changes nothing.
func TestKeepLiveViewsLive(t *testing.T) {
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
		exec("INSERT INTO site_information (id, front_feed_view) VALUES (?, ?)", 1, "front")
		rows := []struct {
			name, display, base string
			published, want     bool
		}{
			{"articles", "page", "content", true, true},
			{"front", "page", "content", true, false},
			{"latest", "block", "content", true, false},
			{"comments", "page", "comment", true, false},
			{"tags", "term", "content", true, false},
			{"draft", "block", "content", false, false},
		}
		for i, row := range rows {
			exec("INSERT INTO view_displays (id, machine_name, label, display_type, base_entity, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
				i+1, row.name, row.name, row.display, row.base, 1, now, now)
			if row.published {
				exec("UPDATE view_displays SET published_snapshot = ?, published_at = ? WHERE id = ?", "{}", now, i+1)
			}
		}
		env := migrations.Env{DB: db.DB, Dialect: db.Dialect}
		for run := 1; run <= 2; run++ {
			if err := migrations.KeepLiveViewsLive(ctx, env); err != nil {
				t.Fatalf("run %d: %v", run, err)
			}
			for i, row := range rows {
				var published bool
				if err := db.DB.QueryRowContext(ctx, db.Dialect.Rebind("SELECT published_at IS NOT NULL AND published_snapshot IS NOT NULL FROM view_displays WHERE id = ?"), i+1).Scan(&published); err != nil {
					t.Fatal(err)
				}
				if published != row.want {
					t.Errorf("run %d: %s published %v, want %v", run, row.name, published, row.want)
				}
			}
		}
	})
}
