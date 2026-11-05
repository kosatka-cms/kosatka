package themes

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeBlocksRepo struct {
	entity.BlockRepository
	layout  []entity.BlockWithPlacement
	created []entity.BlockWithPlacement
}

func (f *fakeBlocksRepo) Blocks(context.Context, string) ([]entity.BlockWithPlacement, error) {
	return f.layout, nil
}

func (f *fakeBlocksRepo) CreateBlock(_ context.Context, b entity.BlockDef, p entity.BlockPlacementDef) (*entity.BlockDef, error) {
	f.created = append(f.created, entity.BlockWithPlacement{Block: b, Placement: p})
	return &b, nil
}

func (f *fakeBlocksRepo) CreatePluginPlacement(_ context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	f.created = append(f.created, entity.BlockWithPlacement{Placement: p})
	return &p, nil
}

// TestPlaceBlocks - the theme's blocks its layout lacks are placed: a
// plugin block not in that region yet (in another it may stand too), a
// custom block with its body sanitized unless one with its title is there;
// a plugin block no module offers is left out.
func TestPlaceBlocks(t *testing.T) {
	copyright, blockID := entity.PluginBlockCopyright, 7
	repo := &fakeBlocksRepo{layout: []entity.BlockWithPlacement{
		{Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &copyright, Region: "footer"}},
		{Block: entity.BlockDef{Title: "Already here"}, Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementCustom, BlockID: &blockID, Region: "footer"}},
	}}
	err := PlaceBlocks(context.Background(), repo, "fixture", []entity.ThemeBlock{
		{Plugin: entity.PluginBlockCopyright, Region: "footer"},
		{Plugin: entity.PluginBlockCopyright, Region: "footer_bottom", Settings: map[string]string{"since": "2008"}},
		{Plugin: "no_such_block", Region: "footer"},
		{Title: "Already here", Body: "<p>x</p>", Region: "footer"},
		{Title: "Buttons", Body: `<div class="rt-buttons"><span class="rt-88">88</span><script>alert(1)</script></div>`, Region: "footer", Weight: -10},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.created) != 2 {
		t.Fatalf("placed %d blocks, want 2: %+v", len(repo.created), repo.created)
	}
	plugin, custom := repo.created[0], repo.created[1]
	if plugin.Placement.PluginIDValue() != entity.PluginBlockCopyright || plugin.Placement.Region != "footer_bottom" ||
		plugin.Placement.Theme != "fixture" || plugin.Placement.PluginConfig["since"] != "2008" || plugin.Placement.VisibilityScope != entity.BlockVisibilityAll {
		t.Errorf("the plugin block: %+v", plugin.Placement)
	}
	if custom.Block.Title != "Buttons" || custom.Block.Body != `<div class="rt-buttons"><span class="rt-88">88</span></div>` ||
		!custom.Block.Status || custom.Placement.Weight != -10 || custom.Placement.PlacementType != entity.BlockPlacementCustom {
		t.Errorf("the custom block: %+v", custom)
	}
}
