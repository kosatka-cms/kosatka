//go:build dbintegration

package database_test

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// A theme's values: saved into a new row, then replaced in it.
func TestThemeSettingsRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		repo := database.NewThemeSettingsRepository(db.DB)
		if err := repo.Save(ctx, "retro", map[string]string{"sky": "#000022", "marquee": "0"}); err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, "kosatka", map[string]string{"accent": "#aa0000"}); err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, "retro", map[string]string{"marquee": "1"}); err != nil {
			t.Fatal(err)
		}
		all, err := repo.All(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(all) != 2 || all["kosatka"]["accent"] != "#aa0000" || all["retro"]["marquee"] != "1" || all["retro"]["sky"] != "" {
			t.Errorf("all: %v", all)
		}
	})
}
