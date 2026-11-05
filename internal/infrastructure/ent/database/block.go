package database

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/blockdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/blockplacementdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewdisplaydef"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// NewBlockRepository - the block repository on client.
func NewBlockRepository(client *generated.Client) *BlockRepository {
	return &BlockRepository{client: client}
}

// BlockRepository is the ent adapter for entity.BlockRepository.
type BlockRepository struct {
	client *generated.Client
}

// toBlockWithPlacement pairs a BlockDef with its first eager-loaded
// placement. A custom block has exactly one placement (CreateBlock creates
// both together); a block with none yields a zero-value placement.
func toBlockWithPlacement(v *generated.BlockDef) entity.BlockWithPlacement {
	bp := entity.BlockWithPlacement{Block: dao.ToBlockDefEntity(v)}
	if len(v.Edges.Placements) > 0 {
		bp.Placement = dao.ToBlockPlacementDefEntity(v.Edges.Placements[0])
	}
	return bp
}

// toPluginBlockWithPlacement wraps a plugin-only BlockPlacementDef row (no
// BlockDef, see entity.PluginBlockID). Block stays the zero value; admin
// templates branch on Placement.IsPlugin().
func toPluginBlockWithPlacement(v *generated.BlockPlacementDef) entity.BlockWithPlacement {
	return entity.BlockWithPlacement{Placement: dao.ToBlockPlacementDefEntity(v)}
}

// toViewBlockWithPlacement wraps a view-only BlockPlacementDef row (no
// BlockDef, see entity.BlockPlacementDef.ViewDisplayID), with the
// eager-loaded ViewDisplayDef.
func toViewBlockWithPlacement(v *generated.BlockPlacementDef) entity.BlockWithPlacement {
	bp := entity.BlockWithPlacement{Placement: dao.ToBlockPlacementDefEntity(v)}
	if v.Edges.ViewDisplay != nil {
		vd := dao.ToViewDisplayDefEntity(v.Edges.ViewDisplay)
		bp.ViewDisplay = &vd
	}
	return bp
}

// toMenuBlockWithPlacement wraps a menu-only BlockPlacementDef row (no
// BlockDef, see entity.BlockPlacementDef.MenuID), with the eager-loaded
// MenuDef.
func toMenuBlockWithPlacement(v *generated.BlockPlacementDef) entity.BlockWithPlacement {
	bp := entity.BlockWithPlacement{Placement: dao.ToBlockPlacementDefEntity(v)}
	if v.Edges.Menu != nil {
		m := dao.ToMenuDefEntity(v.Edges.Menu)
		bp.Menu = &m
	}
	return bp
}

// Blocks - see entity.BlockRepository.Blocks; by region, then weight.
func (r BlockRepository) Blocks(ctx context.Context, theme string) ([]entity.BlockWithPlacement, error) {
	client := GetClient(ctx, r.client)
	res, err := client.BlockDef.Query().
		Where(blockdef.HasPlacementsWith(blockplacementdef.Theme(theme))).
		WithPlacements(func(q *generated.BlockPlacementDefQuery) { q.Where(blockplacementdef.Theme(theme)) }).
		Order(generated.Asc(blockdef.FieldTitle)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("BlockRepository.Blocks: %w", err)
	}

	pluginRows, err := client.BlockPlacementDef.Query().
		Where(blockplacementdef.PlacementType(string(entity.BlockPlacementPlugin)), blockplacementdef.Theme(theme)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("BlockRepository.Blocks: %w", err)
	}

	viewRows, err := client.BlockPlacementDef.Query().
		Where(blockplacementdef.PlacementType(string(entity.BlockPlacementView)), blockplacementdef.Theme(theme)).
		WithViewDisplay().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("BlockRepository.Blocks: %w", err)
	}

	menuRows, err := client.BlockPlacementDef.Query().
		Where(blockplacementdef.PlacementType(string(entity.BlockPlacementMenu)), blockplacementdef.Theme(theme)).
		WithMenu().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("BlockRepository.Blocks: %w", err)
	}

	blocks := make([]entity.BlockWithPlacement, 0, len(res)+len(pluginRows)+len(viewRows)+len(menuRows))
	for _, v := range res {
		blocks = append(blocks, toBlockWithPlacement(v))
	}
	for _, v := range pluginRows {
		blocks = append(blocks, toPluginBlockWithPlacement(v))
	}
	for _, v := range viewRows {
		blocks = append(blocks, toViewBlockWithPlacement(v))
	}
	for _, v := range menuRows {
		blocks = append(blocks, toMenuBlockWithPlacement(v))
	}
	// Sort by (region, weight) - the drag-and-drop order within a region;
	// the title is only a tie-break. Done in Go because ent can't easily
	// order by a field of the related placement.
	sort.SliceStable(blocks, func(i, j int) bool {
		if blocks[i].Placement.Region != blocks[j].Placement.Region {
			return blocks[i].Placement.Region < blocks[j].Placement.Region
		}
		return blocks[i].Placement.Weight < blocks[j].Placement.Weight
	})
	return blocks, nil
}

// BlockByID - the block with id; ErrEntityNotFound if there is none.
func (r BlockRepository) BlockByID(ctx context.Context, id int) (*entity.BlockDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.BlockDef.Get(ctx, id)
	return one(res, err, dao.ToBlockDefEntity, "block with id %d", id)
}

// BlockByMachineName - the block named name (entity.BlockDef.MachineName).
func (r BlockRepository) BlockByMachineName(ctx context.Context, name string) (*entity.BlockDef, error) {
	res, err := GetClient(ctx, r.client).BlockDef.Query().Where(blockdef.MachineName(name)).Only(ctx)
	return one(res, err, dao.ToBlockDefEntity, "block %q", name)
}

// SearchBlocks - see entity.BlockRepository.SearchBlocks.
func (r BlockRepository) SearchBlocks(ctx context.Context, query string, limit int) ([]entity.BlockDef, error) {
	q := GetClient(ctx, r.client).BlockDef.Query()
	if query != "" {
		q = q.Where(blockdef.Or(blockdef.TitleContainsFold(query), blockdef.MachineNameContainsFold(query)))
	}
	rows, err := q.Order(generated.Asc(blockdef.FieldTitle)).Limit(limit).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("searching blocks for %q: %w", query, err)
	}
	out := make([]entity.BlockDef, len(rows))
	for i, row := range rows {
		out[i] = dao.ToBlockDefEntity(row)
	}
	return out, nil
}

// MachineNameTaken reports whether a block other than excludeID (0 -
// none) is named name.
func (r BlockRepository) MachineNameTaken(ctx context.Context, name string, excludeID int) (bool, error) {
	ok, err := GetClient(ctx, r.client).BlockDef.Query().Where(blockdef.MachineName(name), blockdef.IDNEQ(excludeID)).Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("checking the block name %q: %w", name, err)
	}
	return ok, nil
}

// freeMachineName - b's machine name, or for a block without one a free
// one from its title (slug.MachineName, "_2", "_3"… when another block
// has it).
func (r BlockRepository) freeMachineName(ctx context.Context, b entity.BlockDef) (string, error) {
	if b.MachineName != "" {
		return b.MachineName, nil
	}
	base := slug.MachineName(b.Title, "block")
	name := base
	for n := 2; ; n++ {
		taken, err := r.MachineNameTaken(ctx, name, b.ID)
		if err != nil || !taken {
			return name, err
		}
		name = fmt.Sprintf("%s_%d", base, n)
	}
}

// CreateBlock creates the block and its single placement together; the
// caller wraps it in a UOW.
func (r BlockRepository) CreateBlock(ctx context.Context, b entity.BlockDef, p entity.BlockPlacementDef) (*entity.BlockDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	name, err := r.freeMachineName(ctx, b)
	if err != nil {
		return nil, err
	}
	v, err := client.BlockDef.Create().
		SetTitle(b.Title).
		SetMachineName(name).
		SetShowTitle(b.ShowTitle).
		SetBody(b.Body).
		SetNillableTextFormatID(b.TextFormatID).
		SetStatus(b.Status).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating block: %w", err)
	}

	if _, err := client.BlockPlacementDef.Create().
		SetBlockID(v.ID).
		SetTheme(p.Theme).
		SetRegion(string(p.Region)).
		SetWeight(p.Weight).
		SetVisibilityScope(string(p.VisibilityScope)).
		SetNillableVisibilityContentType(p.VisibilityContentType).
		SetVisibilityPages(p.VisibilityPages).
		SetVisibilityPagesMode(string(p.VisibilityPagesMode)).
		SetVisibilityRoles(p.VisibilityRoles).
		SetLazy(p.Lazy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("error creating block placement: %w", err)
	}

	created := dao.ToBlockDefEntity(v)
	return &created, nil
}

// UpdateBlock saves the block (one without a MachineName named from its
// title).
func (r BlockRepository) UpdateBlock(ctx context.Context, b entity.BlockDef) (*entity.BlockDef, error) {
	client := GetClient(ctx, r.client)
	name, err := r.freeMachineName(ctx, b)
	if err != nil {
		return nil, err
	}
	upd := client.BlockDef.UpdateOneID(b.ID).
		SetTitle(b.Title).
		SetMachineName(name).
		SetShowTitle(b.ShowTitle).
		SetBody(b.Body).
		SetStatus(b.Status).
		SetUpdatedAt(time.Now())
	// SetNillableTextFormatID doesn't touch the column on nil, so an
	// explicit clear is needed when the admin resets the format to none.
	if b.TextFormatID != nil {
		upd = upd.SetTextFormatID(*b.TextFormatID)
	} else {
		upd = upd.ClearTextFormatID()
	}
	v, err := upd.Save(ctx)
	return one(v, err, dao.ToBlockDefEntity, "block with id %d", b.ID)
}

// DeleteBlock removes the block along with its placements
// (application-level cascade, not FK ON DELETE).
func (r BlockRepository) DeleteBlock(ctx context.Context, id int) error {
	client := GetClient(ctx, r.client)
	if _, err := client.BlockPlacementDef.Delete().Where(blockplacementdef.BlockID(id)).Exec(ctx); err != nil {
		return fmt.Errorf("error deleting block placements: %w", err)
	}
	return notFoundAs(client.BlockDef.DeleteOneID(id).Exec(ctx), "block with id %d", id)
}

// CreatePluginPlacement creates a plugin-block placement (no BlockDef
// row - see entity.PluginBlockID).
func (r BlockRepository) CreatePluginPlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	create := client.BlockPlacementDef.Create()
	if p.PluginConfig != nil {
		create.SetPluginConfig(p.PluginConfig)
	}
	v, err := create.
		SetPlacementType(string(entity.BlockPlacementPlugin)).
		SetPluginID(string(p.PluginIDValue())).
		SetTheme(p.Theme).
		SetRegion(string(p.Region)).
		SetWeight(p.Weight).
		SetTitle(p.Title).
		SetShowTitle(p.ShowTitle).
		SetVisibilityScope(string(p.VisibilityScope)).
		SetNillableVisibilityContentType(p.VisibilityContentType).
		SetVisibilityPages(p.VisibilityPages).
		SetVisibilityPagesMode(string(p.VisibilityPagesMode)).
		SetVisibilityRoles(p.VisibilityRoles).
		SetLazy(p.Lazy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating plugin block placement: %w", err)
	}

	created := dao.ToBlockPlacementDefEntity(v)
	return &created, nil
}

// CreateViewPlacement creates a View-block placement (no BlockDef row -
// see entity.BlockPlacementDef.ViewDisplayID).
func (r BlockRepository) CreateViewPlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	v, err := client.BlockPlacementDef.Create().
		SetPlacementType(string(entity.BlockPlacementView)).
		SetViewDisplayID(p.ViewDisplayIDValue()).
		SetTheme(p.Theme).
		SetRegion(string(p.Region)).
		SetWeight(p.Weight).
		SetTitle(p.Title).
		SetShowTitle(p.ShowTitle).
		SetVisibilityScope(string(p.VisibilityScope)).
		SetNillableVisibilityContentType(p.VisibilityContentType).
		SetVisibilityPages(p.VisibilityPages).
		SetVisibilityPagesMode(string(p.VisibilityPagesMode)).
		SetVisibilityRoles(p.VisibilityRoles).
		SetLazy(p.Lazy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating view block placement: %w", err)
	}

	created := dao.ToBlockPlacementDefEntity(v)
	return &created, nil
}

// CreateMenuPlacement creates a menu-block placement (no BlockDef row -
// see entity.BlockPlacementDef.MenuID).
func (r BlockRepository) CreateMenuPlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	v, err := client.BlockPlacementDef.Create().
		SetPlacementType(string(entity.BlockPlacementMenu)).
		SetMenuID(p.MenuIDValue()).
		SetMenuStartLevel(p.MenuStartLevel).
		SetMenuLevelsToShow(p.MenuLevelsToShow).
		SetTheme(p.Theme).
		SetRegion(string(p.Region)).
		SetWeight(p.Weight).
		SetVisibilityScope(string(p.VisibilityScope)).
		SetNillableVisibilityContentType(p.VisibilityContentType).
		SetVisibilityPages(p.VisibilityPages).
		SetVisibilityPagesMode(string(p.VisibilityPagesMode)).
		SetVisibilityRoles(p.VisibilityRoles).
		SetLazy(p.Lazy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating menu block placement: %w", err)
	}

	created := dao.ToBlockPlacementDefEntity(v)
	return &created, nil
}

// DeletePlacement removes one placement row directly (custom or
// plugin) - unlike DeleteBlock, never touches a BlockDef row.
func (r BlockRepository) DeletePlacement(ctx context.Context, placementID int) error {
	client := GetClient(ctx, r.client)
	return notFoundAs(client.BlockPlacementDef.DeleteOneID(placementID).Exec(ctx), "block placement with id %d", placementID)
}

// PlacementByBlockID - the custom block's placement in theme;
// ErrEntityNotFound if it has none there.
func (r BlockRepository) PlacementByBlockID(ctx context.Context, blockID int, theme string) (*entity.BlockPlacementDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.BlockPlacementDef.Query().Where(blockplacementdef.BlockID(blockID), blockplacementdef.Theme(theme)).Only(ctx)
	return one(res, err, dao.ToBlockPlacementDefEntity, "block placement for block %d", blockID)
}

// PlacementByID returns any placement (custom/plugin/view/menu) by its own
// ID; PlacementByBlockID can't find plugin/view/menu placements, which have
// no owning BlockDef.
func (r BlockRepository) PlacementByID(ctx context.Context, id int) (*entity.BlockPlacementDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.BlockPlacementDef.Get(ctx, id)
	return one(res, err, dao.ToBlockPlacementDefEntity, "block placement with id %d", id)
}

// UpdatePlacement saves the placement's region, weight, title and
// visibility (a nil PluginConfig keeps the stored one) and returns it.
func (r BlockRepository) UpdatePlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	client := GetClient(ctx, r.client)
	update := client.BlockPlacementDef.UpdateOneID(p.ID).
		SetRegion(string(p.Region)).
		SetWeight(p.Weight).
		SetVisibilityScope(string(p.VisibilityScope)).
		// MenuStartLevel/MenuLevelsToShow are meaningful only for menu
		// placements; writing them for others is a harmless no-op.
		SetMenuStartLevel(p.MenuStartLevel).
		SetMenuLevelsToShow(p.MenuLevelsToShow).
		// Title/ShowTitle/VisibilityPages/VisibilityPagesMode are shared
		// columns, written for every placement type.
		SetTitle(p.Title).
		SetShowTitle(p.ShowTitle).
		SetVisibilityPages(p.VisibilityPages).
		SetVisibilityPagesMode(string(p.VisibilityPagesMode)).
		SetVisibilityRoles(p.VisibilityRoles).
		SetLazy(p.Lazy).
		SetUpdatedAt(time.Now())

	if p.VisibilityContentType != nil {
		update.SetVisibilityContentType(*p.VisibilityContentType)
	} else {
		update.ClearVisibilityContentType()
	}
	// nil - the stored settings stay (pages that don't edit them).
	if p.PluginConfig != nil {
		update.SetPluginConfig(p.PluginConfig)
	}

	v, err := update.Save(ctx)
	return one(v, err, dao.ToBlockPlacementDefEntity, "block placement with id %d", p.ID)
}

// PublishedPlacementsByRegion returns published placements for one region
// of one theme, weight-sorted - raw data for CacheService; per-page
// visibility is filtered per request (see
// entity.BlockRepository.PublishedPlacementsByRegion).
func (r BlockRepository) PublishedPlacementsByRegion(ctx context.Context, theme string, region entity.Region) ([]entity.BlockWithPlacement, error) {
	client := GetClient(ctx, r.client)
	res, err := client.BlockPlacementDef.Query().
		Where(
			blockplacementdef.Theme(theme),
			blockplacementdef.Region(string(region)),
			blockplacementdef.Or(
				blockplacementdef.HasBlockWith(blockdef.Status(true)),
				blockplacementdef.PlacementType(string(entity.BlockPlacementPlugin)),
				// A view placement has no BlockDef; it is published when its
				// View display is published (Status==1).
				blockplacementdef.And(
					blockplacementdef.PlacementType(string(entity.BlockPlacementView)),
					blockplacementdef.HasViewDisplayWith(viewdisplaydef.Status(1)),
				),
				// Menu placements are always published, like plugin blocks:
				// MenuDef has no draft/published state.
				blockplacementdef.PlacementType(string(entity.BlockPlacementMenu)),
			),
		).
		WithBlock().
		WithViewDisplay().
		WithMenu().
		Order(generated.Asc(blockplacementdef.FieldWeight)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]entity.BlockWithPlacement, len(res))
	for i, v := range res {
		bp := entity.BlockWithPlacement{Placement: dao.ToBlockPlacementDefEntity(v)}
		if v.Edges.Block != nil {
			bp.Block = dao.ToBlockDefEntity(v.Edges.Block)
		}
		if v.Edges.ViewDisplay != nil {
			vd := dao.ToViewDisplayDefEntity(v.Edges.ViewDisplay)
			bp.ViewDisplay = &vd
		}
		if v.Edges.Menu != nil {
			m := dao.ToMenuDefEntity(v.Edges.Menu)
			bp.Menu = &m
		}
		out[i] = bp
	}
	return out, nil
}

// ReorderPlacements bulk-applies Region/Weight from the admin
// drag-and-drop list (one UpdateOneID call per row).
func (r BlockRepository) ReorderPlacements(ctx context.Context, updates []entity.BlockPlacementOrderUpdate) error {
	client := GetClient(ctx, r.client)
	for _, u := range updates {
		err := client.BlockPlacementDef.UpdateOneID(u.PlacementID).
			SetRegion(string(u.Region)).
			SetWeight(u.Weight).
			Exec(ctx)
		if err != nil {
			if generated.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("error reordering block placement %d: %w", u.PlacementID, err)
		}
	}
	return nil
}

// ThemeHasPlacements reports whether the theme has any placement.
func (r BlockRepository) ThemeHasPlacements(ctx context.Context, theme string) (bool, error) {
	return GetClient(ctx, r.client).BlockPlacementDef.Query().Where(blockplacementdef.Theme(theme)).Exist(ctx)
}

// CopyPlacement creates a copy of p (any type: a custom block's placement
// points at the same block) - used to give a new theme the placements of
// the previous one.
func (r BlockRepository) CopyPlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	create := client.BlockPlacementDef.Create().
		SetPlacementType(string(p.PlacementType)).
		SetNillableBlockID(p.BlockID).
		SetNillablePluginID(pluginIDPtr(p.PluginID)).
		SetNillableViewDisplayID(p.ViewDisplayID).
		SetNillableMenuID(p.MenuID).
		SetMenuStartLevel(p.MenuStartLevel).
		SetMenuLevelsToShow(p.MenuLevelsToShow).
		SetTheme(p.Theme).
		SetRegion(string(p.Region)).
		SetWeight(p.Weight).
		SetTitle(p.Title).
		SetShowTitle(p.ShowTitle).
		SetVisibilityScope(string(p.VisibilityScope)).
		SetNillableVisibilityContentType(p.VisibilityContentType).
		SetVisibilityPages(p.VisibilityPages).
		SetVisibilityPagesMode(string(p.VisibilityPagesMode)).
		SetVisibilityRoles(p.VisibilityRoles).
		SetLazy(p.Lazy).
		SetCreatedAt(now).
		SetUpdatedAt(now)
	// A plugin block's settings (the poll or the form it shows) go along.
	if len(p.PluginConfig) > 0 {
		create.SetPluginConfig(p.PluginConfig)
	}
	v, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error copying block placement: %w", err)
	}
	created := dao.ToBlockPlacementDefEntity(v)
	return &created, nil
}

func pluginIDPtr(id *entity.PluginBlockID) *string {
	if id == nil {
		return nil
	}
	s := string(*id)
	return &s
}
