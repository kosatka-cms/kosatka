//go:build dbintegration

package database_test

import (
	"context"
	"testing"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestPluginPlacementConfig - a plugin placement's settings are stored on
// update, kept by an update that doesn't carry them and copied with the
// placement to another theme.
func TestPluginPlacementConfig(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		repo := database.NewBlockRepository(generated.NewClient(generated.Driver(entsql.OpenDB(string(db.Dialect), db.DB))))
		id := entity.PluginBlockID("webform")
		p, err := repo.CreatePluginPlacement(ctx, entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &id,
			Theme: "default", Region: "sidebar", VisibilityScope: entity.BlockVisibilityAll})
		if err != nil {
			t.Fatal(err)
		}
		if p.PluginSetting(entity.PluginBlockTargetKey) != "" {
			t.Fatalf("new placement: %+v", p.PluginConfig)
		}
		update := *p
		update.PluginConfig = map[string]string{entity.PluginBlockTargetKey: "12"}
		if _, err := repo.UpdatePlacement(ctx, update); err != nil {
			t.Fatal(err)
		}
		update.PluginConfig = nil
		update.Title = "Contact us"
		if _, err := repo.UpdatePlacement(ctx, update); err != nil {
			t.Fatal(err)
		}
		got, err := repo.PlacementByID(ctx, p.ID)
		if err != nil || got.Title != "Contact us" || got.PluginSetting(entity.PluginBlockTargetKey) != "12" {
			t.Errorf("stored: %+v %v", got, err)
		}

		// Copied to another theme (its first layout), the settings go
		// along: the block shows the same form there.
		got.Theme = "retro"
		copied, err := repo.CopyPlacement(ctx, *got)
		if err != nil || copied.Theme != "retro" || copied.PluginSetting(entity.PluginBlockTargetKey) != "12" {
			t.Errorf("copied: %+v %v", copied, err)
		}
	})
}
