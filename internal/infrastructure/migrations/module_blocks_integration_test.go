//go:build dbintegration

package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestPlaceModuleBlocks - a module's block that names a region goes into
// that region of the site's theme, once; one whose region the theme
// lacks, or that names none, is left to the Blocks page.
func TestPlaceModuleBlocks(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("../../.."); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	saved := entity.Modules
	t.Cleanup(func() { entity.Modules = saved })
	entity.Modules = append(append([]entity.Module(nil), saved...), entity.Module{Name: "blocks_test", PluginBlocks: []entity.ModulePluginBlock{
		{ID: "blocks_test_inline", Label: "Inline", Region: "content_inline"},
		{ID: "blocks_test_nowhere", Label: "Nowhere", Region: "no_such_region"},
		{ID: "blocks_test_manual", Label: "Manual"},
	}})
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := Runner{DB: db.DB, Dialect: db.Dialect}
		if _, err := r.Install(ctx, CoreSet); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.ExecContext(ctx, `UPDATE site_information SET theme = 'kosatka' WHERE id = 1`); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if err := r.PlaceModuleBlocks(ctx, "blocks_test"); err != nil {
				t.Fatal(err)
			}
		}
		rows, err := db.DB.QueryContext(ctx, `SELECT plugin_id, theme, region FROM block_placements WHERE plugin_id LIKE 'blocks_test%'`)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		var got []string
		for rows.Next() {
			var plugin, theme, region string
			if err := rows.Scan(&plugin, &theme, &region); err != nil {
				t.Fatal(err)
			}
			got = append(got, plugin+" "+theme+" "+region)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != "blocks_test_inline kosatka content_inline" {
			t.Errorf("placed: %v", got)
		}
		if err := r.PlaceModuleBlocks(ctx, "no_such_module"); err != nil {
			t.Errorf("an unknown module: %v", err)
		}
	})
}

// TestPlaceModuleBlocksConfigTheme - a site that chose no theme on the
// Appearance page gets the block in the config's theme (paths.Theme, set
// by "app migrate" as by the server), not in the engine's templates.
func TestPlaceModuleBlocksConfigTheme(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("../../.."); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	saved := entity.Modules
	t.Cleanup(func() { entity.Modules = saved })
	entity.Modules = append(append([]entity.Module(nil), saved...), entity.Module{Name: "blocks_test", PluginBlocks: []entity.ModulePluginBlock{
		{ID: "blocks_test_inline", Label: "Inline", Region: "content_inline"},
	}})
	// The engine's tests' theme (testdata/themes/fixture): its manifest
	// has the block's region.
	dir, err := filepath.Abs(filepath.Join("testdata", "themes"))
	if err != nil {
		t.Fatal(err)
	}
	paths.SetThemesDir(dir)
	t.Cleanup(func() { paths.SetThemesDir("") })
	savedTheme := paths.Theme()
	t.Cleanup(func() { paths.SetTheme(savedTheme) })
	paths.SetTheme("fixture")
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := Runner{DB: db.DB, Dialect: db.Dialect}
		if _, err := r.Install(ctx, CoreSet); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.ExecContext(ctx, `UPDATE site_information SET theme = NULL WHERE id = 1`); err != nil {
			t.Fatal(err)
		}
		if err := r.PlaceModuleBlocks(ctx, "blocks_test"); err != nil {
			t.Fatal(err)
		}
		var theme string
		if err := db.DB.QueryRowContext(ctx, `SELECT theme FROM block_placements WHERE plugin_id = 'blocks_test_inline'`).Scan(&theme); err != nil {
			t.Fatal(err)
		}
		if theme != "fixture" {
			t.Errorf("placed in theme %q, want the config's fixture", theme)
		}
	})
}
