package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// PlaceModuleBlocks places the module's plugin blocks that name a region
// (entity.ModulePluginBlock.Region) in the site's active theme: where the
// theme has the region (its own or its base's) and the region lacks the
// block; one whose region the theme lacks is left to the Blocks page. Run
// when the module is installed (app install, the Modules page) and by a
// module's data step for a site that had it already.
func PlaceModuleBlocks(ctx context.Context, env Env, module string) error {
	m, ok := entity.ModuleByName(module)
	if !ok {
		return nil
	}
	var blocks []entity.ThemeBlock
	for _, b := range m.PluginBlocks {
		if b.Region != "" {
			blocks = append(blocks, entity.ThemeBlock{Plugin: b.ID, Region: b.Region})
		}
	}
	if len(blocks) == 0 {
		return nil
	}
	dir, err := activeThemeDir(ctx, env)
	if err != nil {
		return fmt.Errorf("the site's theme: %w", err)
	}
	info, err := themes.LoadInfoFor(dir)
	if err != nil {
		return fmt.Errorf("theme %q: %w", dir, err)
	}
	var placed []entity.ThemeBlock
	for _, b := range blocks {
		if info.Has(b.Region) {
			placed = append(placed, b)
		}
	}
	if err := placePluginBlocks(ctx, env, entity.ThemeKey(dir), placed); err != nil {
		return fmt.Errorf("the blocks of module %s: %w", module, err)
	}
	return nil
}

// PlaceModuleBlocks - PlaceModuleBlocks on the runner's database.
func (r Runner) PlaceModuleBlocks(ctx context.Context, module string) error {
	return PlaceModuleBlocks(ctx, r.env(), module)
}

// activeThemeDir - the theme chosen on the Appearance page
// (site_information.theme), else the config's.
func activeThemeDir(ctx context.Context, env Env) (string, error) {
	var theme sql.NullString
	err := env.DB.QueryRowContext(ctx, `SELECT theme FROM site_information WHERE id = 1`).Scan(&theme)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if theme.Valid {
		return entity.ThemeDir(theme.String), nil
	}
	return paths.Theme(), nil
}

// placePluginBlocks - the plugin blocks in the theme's regions where the
// region lacks them, as themes.PlaceBlocks does, in SQL on env.DB: a
// module's data step may run on an older schema than the ent client's.
func placePluginBlocks(ctx context.Context, env Env, theme string, blocks []entity.ThemeBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	placedAt := map[[2]string]bool{}
	err := eachRow(ctx, env, `SELECT plugin_id, region FROM block_placements WHERE theme = ? AND placement_type = 'plugin'`, []any{theme}, func(rows *sql.Rows) error {
		var plugin, region string
		if err := rows.Scan(&plugin, &region); err != nil {
			return err
		}
		placedAt[[2]string{plugin, region}] = true
		return nil
	})
	if err != nil {
		return err
	}
	jsonCast := "CAST(? AS JSON)"
	if env.Dialect == sqldialect.Postgres {
		jsonCast = "CAST(? AS jsonb)"
	}
	offered := entity.PluginBlockLabels()
	for _, b := range blocks {
		key := [2]string{string(b.Plugin), string(b.Region)}
		if _, ok := offered[b.Plugin]; !ok || placedAt[key] {
			continue
		}
		config := "{}"
		if len(b.Settings) > 0 {
			encoded, err := json.Marshal(b.Settings)
			if err != nil {
				return err
			}
			config = string(encoded)
		}
		if _, err := env.DB.ExecContext(ctx, env.Dialect.Rebind(`INSERT INTO block_placements (theme, region, weight, placement_type, plugin_id, title, show_title, plugin_config, visibility_scope, visibility_pages_mode, created_at, updated_at)
			VALUES (?, ?, ?, 'plugin', ?, ?, ?, `+jsonCast+`, 'all', 'show', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`), theme, string(b.Region), b.Weight, string(b.Plugin), b.Title, b.ShowTitle, config); err != nil {
			return fmt.Errorf("plugin block %s: %w", b.Plugin, err)
		}
		placedAt[key] = true
	}
	return nil
}
