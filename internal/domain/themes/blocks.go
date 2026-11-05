package themes

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
)

// PlaceBlocks places the blocks a theme comes with (theme.yaml
// "blocks", entity.ThemeBlock) that its layout lacks: a plugin block not
// in that region yet (the same one may stand in two: the site's branding
// in the header and its name in the footer), a custom block no custom
// block of the layout shares the title with; a plugin block no module offers (its
// module off) is left out. A custom block's body is sanitized as one saved
// in the admin (htmlsanitize.SanitizeAdmin): a theme from its own
// repository is someone else's HTML. The caller wraps it in a UOW.
func PlaceBlocks(ctx context.Context, repo entity.BlockRepository, theme string, blocks []entity.ThemeBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	layout, err := repo.Blocks(ctx, theme)
	if err != nil {
		return fmt.Errorf("listing the theme's blocks (theme %v): %w", theme, err)
	}
	type placed struct {
		plugin entity.PluginBlockID
		region entity.Region
	}
	plugins, titles := map[placed]bool{}, map[string]bool{}
	for _, bp := range layout {
		switch {
		case bp.Placement.IsPlugin():
			plugins[placed{bp.Placement.PluginIDValue(), bp.Placement.Region}] = true
		case bp.Placement.BlockID != nil:
			titles[bp.Block.Title] = true
		}
	}
	offered := entity.PluginBlockLabels()
	for _, b := range blocks {
		p := entity.BlockPlacementDef{Theme: theme, Region: b.Region, Weight: b.Weight,
			VisibilityScope: entity.BlockVisibilityAll, VisibilityPagesMode: entity.BlockVisibilityPagesShow}
		if b.IsCustom() {
			if titles[b.Title] {
				continue
			}
			p.PlacementType = entity.BlockPlacementCustom
			block := entity.BlockDef{Title: b.Title, ShowTitle: b.ShowTitle, Body: htmlsanitize.SanitizeAdmin(b.Body), Status: true}
			if _, err := repo.CreateBlock(ctx, block, p); err != nil {
				return fmt.Errorf("placing the theme's block (theme %v, title %v): %w", theme, b.Title, err)
			}
			titles[b.Title] = true
			continue
		}
		if _, ok := offered[b.Plugin]; !ok || plugins[placed{b.Plugin, b.Region}] {
			continue
		}
		id := b.Plugin
		p.PlacementType, p.PluginID, p.Title, p.ShowTitle, p.PluginConfig = entity.BlockPlacementPlugin, &id, b.Title, b.ShowTitle, b.Settings
		if _, err := repo.CreatePluginPlacement(ctx, p); err != nil {
			return fmt.Errorf("placing the theme's block (theme %v, plugin %v): %w", theme, b.Plugin, err)
		}
		plugins[placed{b.Plugin, b.Region}] = true
	}
	return nil
}
