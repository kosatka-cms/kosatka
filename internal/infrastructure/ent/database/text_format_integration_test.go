//go:build dbintegration

package database_test

import (
	"context"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

// TestTextFormatFilters - a format's text filters are saved on create and
// update and read back; a new format has none.
func TestTextFormatFilters(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		formats := database.NewTextFormatRepository(entClient(db))
		created, err := formats.CreateTextFormat(ctx, entity.TextFormatDef{MachineName: "rich", Label: "Rich", Status: 1})
		if err != nil || created.Filters != "" {
			t.Fatalf("created: %+v %v", created, err)
		}
		created.Filters = "video_embed other"
		if _, err := formats.UpdateTextFormat(ctx, *created); err != nil {
			t.Fatal(err)
		}
		if f, err := formats.TextFormatByID(ctx, created.ID); err != nil || f.Filters != "video_embed other" || len(f.FilterNames()) != 2 {
			t.Errorf("updated: %+v %v", f, err)
		}
	})
}

// TestTextFiltersInstallUninstall - a module's install step turns its
// filter on for the formats with the visual editor (once), its uninstall
// takes it out of every format, the others' filters kept.
func TestTextFiltersInstallUninstall(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		client := entClient(db)
		formats := database.NewTextFormatRepository(client)
		rich, err := formats.CreateTextFormat(ctx, entity.TextFormatDef{MachineName: "rich", Label: "Rich", HasRichEditor: true, Filters: "other", Status: 1})
		if err != nil {
			t.Fatal(err)
		}
		plain, err := formats.CreateTextFormat(ctx, entity.TextFormatDef{MachineName: "plain", Label: "Plain", Status: 1})
		if err != nil {
			t.Fatal(err)
		}
		env := manifestkit.InstallEnv{DB: db.DB, Dialect: db.Dialect}
		for range 2 {
			if err := manifestkit.EnableTextFilter(ctx, env, "video_embed"); err != nil {
				t.Fatal(err)
			}
		}
		if f, _ := formats.TextFormatByID(ctx, rich.ID); f.Filters != "other video_embed" {
			t.Errorf("rich after install: %q", f.Filters)
		}
		if f, _ := formats.TextFormatByID(ctx, plain.ID); f.Filters != "" {
			t.Errorf("plain after install: %q", f.Filters)
		}

		left, err := database.NewModuleUninstallRepository(db.DB).RemoveTextFilters(ctx, []string{"video_embed"})
		if err != nil || !slices.Equal(left[rich.ID], []string{"other"}) {
			t.Errorf("removed: %v %v", left, err)
		}
		if _, ok := left[plain.ID]; ok {
			t.Error("plain had no filter to remove")
		}
		all, _ := formats.TextFormats(ctx)
		for _, f := range all {
			if slices.Contains(f.FilterNames(), "video_embed") {
				t.Errorf("%s keeps the filter after uninstall", f.MachineName)
			}
		}
	})
}
