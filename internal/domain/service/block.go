package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// BlockService - the blocks and their placements in the themes' regions
// (custom blocks, plugin, View and menu blocks).
type BlockService interface {
	Blocks(ctx context.Context, theme string) ([]entity.BlockWithPlacement, error)
	BlockByID(ctx context.Context, id int) (*entity.BlockDef, error)
	// BlockByMachineName - the block named name (entity.BlockDef.MachineName).
	BlockByMachineName(ctx context.Context, name string) (*entity.BlockDef, error)
	// SearchBlocks - see entity.BlockRepository.SearchBlocks.
	SearchBlocks(ctx context.Context, query string, limit int) ([]entity.BlockDef, error)
	// CreateBlock, UpdateBlock: a MachineName must be valid and free
	// (ErrBlockMachineName); none - one from the title.
	CreateBlock(ctx context.Context, b entity.BlockDef, p entity.BlockPlacementDef) (*entity.BlockDef, error)
	UpdateBlock(ctx context.Context, b entity.BlockDef) (*entity.BlockDef, error)
	DeleteBlock(ctx context.Context, id int) error

	CreatePluginPlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error)
	CreateViewPlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error)
	CreateMenuPlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error)
	DeletePlacement(ctx context.Context, placementID int) error

	PlacementByBlockID(ctx context.Context, blockID int, theme string) (*entity.BlockPlacementDef, error)
	// InitThemePlacements gives a theme without placements a copy of
	// another theme's, in the regions it also has, and then the blocks the
	// theme comes with (themes.PlaceBlocks); false when the theme already has
	// its own.
	InitThemePlacements(ctx context.Context, theme, from string, regions entity.ThemeInfo) (bool, error)
	PlacementByID(ctx context.Context, id int) (*entity.BlockPlacementDef, error)
	UpdatePlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error)
	ReorderPlacements(ctx context.Context, updates []entity.BlockPlacementOrderUpdate) error

	// PublishedPlacementsByRegion returns published placements for one
	// region; see DefaultBlockService.PublishedPlacementsByRegion.
	PublishedPlacementsByRegion(ctx context.Context, theme string, region entity.Region) ([]entity.BlockWithPlacement, error)
}

// DefaultBlockService - the BlockService on the block repository.
type DefaultBlockService struct {
	repository entity.BlockRepository
	uow        entity.UnitOfWork
	logger     *slog.Logger
}

// NewBlockService - the block service on r (the blocks and placements),
// uow (the transactions) and baseLogger.
func NewBlockService(r entity.BlockRepository, uow entity.UnitOfWork, baseLogger *slog.Logger) BlockService {
	return &DefaultBlockService{
		repository: r,
		uow:        uow,
		logger:     baseLogger.With("component", "block.service"),
	}
}

// Blocks - every placement of theme, with its block, View or menu.
func (s *DefaultBlockService) Blocks(ctx context.Context, theme string) ([]entity.BlockWithPlacement, error) {
	blocks, err := s.repository.Blocks(ctx, theme)
	return entity.Wrapped(blocks, err, "listing blocks")
}

// BlockByID - the custom block id.
func (s *DefaultBlockService) BlockByID(ctx context.Context, id int) (*entity.BlockDef, error) {
	b, err := s.repository.BlockByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ErrBlockMachineName - a block's machine name isn't [a-z][a-z0-9_]*
// (up to 64), or another block has it.
var ErrBlockMachineName = errors.New("a block's machine name must be lowercase letters, digits and _, starting with a letter, and not another block's")

// blockMachineNameRe - a block's machine name.
var blockMachineNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// checkMachineName - ErrBlockMachineName when b's machine name (if any)
// is invalid or another block's.
func (s *DefaultBlockService) checkMachineName(ctx context.Context, b entity.BlockDef) error {
	if b.MachineName == "" {
		return nil
	}
	if !blockMachineNameRe.MatchString(b.MachineName) {
		return fmt.Errorf("%w: %q", ErrBlockMachineName, b.MachineName)
	}
	taken, err := s.repository.MachineNameTaken(ctx, b.MachineName, b.ID)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("%w: %q is taken", ErrBlockMachineName, b.MachineName)
	}
	return nil
}

// SearchBlocks - see BlockService.SearchBlocks.
func (s *DefaultBlockService) SearchBlocks(ctx context.Context, query string, limit int) ([]entity.BlockDef, error) {
	blocks, err := s.repository.SearchBlocks(ctx, query, limit)
	return entity.Wrapped(blocks, err, "searching blocks")
}

// BlockByMachineName - see BlockService.BlockByMachineName.
func (s *DefaultBlockService) BlockByMachineName(ctx context.Context, name string) (*entity.BlockDef, error) {
	b, err := s.repository.BlockByMachineName(ctx, name)
	return entity.Found(b, err, "loading block %q", name)
}

// CreateBlock creates the block row and its placement row in one
// transaction.
func (s *DefaultBlockService) CreateBlock(ctx context.Context, b entity.BlockDef, p entity.BlockPlacementDef) (*entity.BlockDef, error) {
	var created *entity.BlockDef
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if err := s.checkMachineName(ctx, b); err != nil {
			return err
		}
		var err error
		created, err = s.repository.CreateBlock(ctx, b, p)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("creating block (title %v): %w", b.Title, err)
	}
	return created, nil
}

// UpdateBlock saves block b; its MachineName must be valid and free
// (ErrBlockMachineName).
func (s *DefaultBlockService) UpdateBlock(ctx context.Context, b entity.BlockDef) (*entity.BlockDef, error) {
	if err := s.checkMachineName(ctx, b); err != nil {
		return nil, fmt.Errorf("updating block (id %v): %w", b.ID, err)
	}
	updated, err := s.repository.UpdateBlock(ctx, b)
	return entity.Wrapped(updated, err, "updating block (id %v)", b.ID)
}

// DeleteBlock deletes the block's placements, then the block row, in one
// transaction.
func (s *DefaultBlockService) DeleteBlock(ctx context.Context, id int) error {
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		return s.repository.DeleteBlock(ctx, id)
	})
	if err != nil {
		return fmt.Errorf("deleting block (id %v): %w", id, err)
	}
	return nil
}

// CreatePluginPlacement inserts a single placement row (no transaction
// needed).
func (s *DefaultBlockService) CreatePluginPlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	created, err := s.repository.CreatePluginPlacement(ctx, p)
	return entity.Wrapped(created, err, "creating plugin block placement (plugin_id %v, region %v)", p.PluginIDValue(), p.Region)
}

// CreateViewPlacement places a View display's block (no block row).
func (s *DefaultBlockService) CreateViewPlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	created, err := s.repository.CreateViewPlacement(ctx, p)
	return entity.Wrapped(created, err, "creating view block placement (view_display_id %v, region %v)", p.ViewDisplayIDValue(), p.Region)
}

// CreateMenuPlacement places a menu's block (no block row).
func (s *DefaultBlockService) CreateMenuPlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	created, err := s.repository.CreateMenuPlacement(ctx, p)
	return entity.Wrapped(created, err, "creating menu block placement (menu_id %v, region %v)", p.MenuIDValue(), p.Region)
}

// DeletePlacement deletes the placement placementID of any kind, leaving
// its block, if it has one.
func (s *DefaultBlockService) DeletePlacement(ctx context.Context, placementID int) error {
	if err := s.repository.DeletePlacement(ctx, placementID); err != nil {
		return fmt.Errorf("deleting block placement (id %v): %w", placementID, err)
	}
	return nil
}

// PlacementByBlockID - the placement of the custom block blockID in
// theme.
func (s *DefaultBlockService) PlacementByBlockID(ctx context.Context, blockID int, theme string) (*entity.BlockPlacementDef, error) {
	p, err := s.repository.PlacementByBlockID(ctx, blockID, theme)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// PlacementByID - the placement id, of any kind.
func (s *DefaultBlockService) PlacementByID(ctx context.Context, id int) (*entity.BlockPlacementDef, error) {
	p, err := s.repository.PlacementByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// UpdatePlacement saves placement p (its region, weight, visibility).
func (s *DefaultBlockService) UpdatePlacement(ctx context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	updated, err := s.repository.UpdatePlacement(ctx, p)
	return entity.Wrapped(updated, err, "updating block placement (id %v)", p.ID)
}

// ReorderPlacements applies the admin drag-and-drop order in one
// transaction (all-or-nothing).
func (s *DefaultBlockService) ReorderPlacements(ctx context.Context, updates []entity.BlockPlacementOrderUpdate) error {
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		return s.repository.ReorderPlacements(ctx, updates)
	})
	if err != nil {
		return fmt.Errorf("reordering block placements: %w", err)
	}
	return nil
}

// PublishedPlacementsByRegion returns published placements for one
// region - the raw data CacheService caches. Per-page visibility is
// filtered later, at render time against the current request (same
// "resolve once, filter per request" split as AnnotateActiveMenuTree).
func (s *DefaultBlockService) PublishedPlacementsByRegion(ctx context.Context, theme string, region entity.Region) ([]entity.BlockWithPlacement, error) {
	placements, err := s.repository.PublishedPlacementsByRegion(ctx, theme, region)
	return entity.Wrapped(placements, err, "listing published block placements (theme %v, region %v)", theme, region)
}

// MatchesVisibilityPages - the "Pages" visibility condition: currentPath
// (leading/trailing "/" trimmed) is matched against pagesText, one pattern
// per line ("*" - any characters, "<front>" - the homepage, i.e. an empty
// currentPath). BlockVisibilityPagesShow: visible only on a match; any
// other mode (including unset) behaves as BlockVisibilityPagesHide:
// visible everywhere except a match. Blank or malformed lines are skipped.
func MatchesVisibilityPages(currentPath, pagesText string, mode entity.BlockVisibilityPagesMode) bool {
	currentPath = strings.Trim(currentPath, "/")
	matched := false
	for _, line := range strings.Split(pagesText, "\n") {
		pattern := strings.Trim(strings.TrimSpace(line), "/")
		if pattern == "" {
			continue
		}
		if pattern == "<front>" {
			if currentPath == "" {
				matched = true
				break
			}
			continue
		}
		if matchWildcard(pattern, currentPath) {
			matched = true
			break
		}
	}
	if mode == entity.BlockVisibilityPagesShow {
		return matched
	}
	return !matched
}

// matchWildcard reports whether s is pattern, where each * stands for any
// run of characters (slashes too).
func matchWildcard(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for _, p := range parts[1 : len(parts)-1] {
		i := strings.Index(s, p)
		if i < 0 {
			return false
		}
		s = s[i+len(p):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}

// RegionHasVisibleBlocks reports whether region has a placement visible on
// the page described by ctx (the page's template data), using the same
// rules as region_blocks.html. In block demo mode every region counts as
// non-empty. Used to skip an empty region's wrapper markup.
func RegionHasVisibleBlocks(region entity.Region, ctx map[string]any) bool {
	if demo, _ := ctx["blockDemoMode"].(bool); demo {
		return true
	}
	placements, _ := ctx["blockPlacements"].(map[entity.Region][]entity.BlockWithPlacement)
	for _, bp := range placements[region] {
		if PlacementVisible(bp.Placement, ctx) {
			return true
		}
	}
	return false
}

// PlacementVisible reports whether placement p shows on the page ctx
// describes (region_blocks.html checks every placement with it): the
// visitor's roles (ctx["currentRoles"]) match the roles condition, and the
// page matches the scope.
func PlacementVisible(p entity.BlockPlacementDef, ctx map[string]any) bool {
	if p.PluginID != nil && !PluginBlockEnabled(*p.PluginID) {
		return false
	}
	roles, _ := ctx["currentRoles"].([]string)
	if !p.VisibleToRoles(roles) {
		return false
	}
	switch p.VisibilityScope {
	case entity.BlockVisibilityAll:
		return true
	case entity.BlockVisibilityHomepage:
		isHomepage, _ := ctx["isHomepage"].(bool)
		return isHomepage
	case entity.BlockVisibilityContentType:
		nodeType, hasNode := templateItemNodeType(ctx["item"])
		return hasNode && p.MatchesContentType(nodeType)
	case entity.BlockVisibilityPages:
		currentPath, _ := ctx["currentPath"].(string)
		return MatchesVisibilityPages(currentPath, p.VisibilityPages, p.VisibilityPagesMode)
	}
	return false
}

// HasVisibleCarouselBlock reports whether the page shows a View block
// displayed as a carousel (ctx["carouselPlacements"]), so html_start
// loads the Swiper stylesheet.
func HasVisibleCarouselBlock(ctx map[string]any) bool {
	carousels, _ := ctx["carouselPlacements"].(map[int]bool)
	if len(carousels) == 0 {
		return false
	}
	placements, _ := ctx["blockPlacements"].(map[entity.Region][]entity.BlockWithPlacement)
	for _, items := range placements {
		for _, bp := range items {
			if carousels[bp.Placement.ID] && PlacementVisible(bp.Placement, ctx) {
				return true
			}
		}
	}
	return false
}

// templateItemNodeType - the content type of a page's "item" (any value
// with a Node entity.Node field, as region_blocks.html reads .item.Node.Type).
func templateItemNodeType(item any) (string, bool) {
	if item == nil {
		return "", false
	}
	v := reflect.Indirect(reflect.ValueOf(item))
	if v.Kind() != reflect.Struct {
		return "", false
	}
	f := v.FieldByName("Node")
	if !f.IsValid() {
		return "", false
	}
	n, ok := f.Interface().(entity.Node)
	if !ok {
		return "", false
	}
	return string(n.Type), true
}

// RegionBlocksStandaloneFiles - the templates region_blocks needs when
// rendered on its own via RenderNamed: it references every plugin block
// partial, and html/template's escaper needs each reachable define (the
// modules' own block partials RenderNamed adds itself).
var RegionBlocksStandaloneFiles = []string{
	"partials/region_blocks.html", "blocks/logo.html", "blocks/search_form.html",
	"blocks/user_menu.html", "blocks/last_comments.html", "blocks/tag_cloud.html", "blocks/copyright.html",
}

// LazyPlacement returns lazy placement id from the cache, ready to render on
// its own (GET /api/block-placement/{id}): Lazy is turned off so the content
// renders instead of the placeholder, and visibility is "all" since the
// requesting page already checked it. false when no lazy placement has
// that id (a menu is never lazy: on its own it would miss the theme's
// menu markup).
func LazyPlacement(placements map[entity.Region][]entity.BlockWithPlacement, id int) (entity.BlockWithPlacement, bool) {
	for _, items := range placements {
		for _, item := range items {
			if item.Placement.ID == id && item.Placement.Lazy && !item.Placement.IsMenu() {
				item.Placement.Lazy = false
				item.Placement.VisibilityScope = entity.BlockVisibilityAll
				return item, true
			}
		}
	}
	return entity.BlockWithPlacement{}, false
}

// ViewPlacement returns View block placement id from the cache (with its
// display); false when no View placement has that id.
func ViewPlacement(placements map[entity.Region][]entity.BlockWithPlacement, id int) (entity.BlockWithPlacement, bool) {
	for _, items := range placements {
		for _, item := range items {
			if item.Placement.ID == id && item.Placement.IsView() && item.ViewDisplay != nil {
				return item, true
			}
		}
	}
	return entity.BlockWithPlacement{}, false
}

// InitThemePlacements - see BlockService.InitThemePlacements.
func (s *DefaultBlockService) InitThemePlacements(ctx context.Context, theme, from string, regions entity.ThemeInfo) (bool, error) {
	if theme == from {
		return false, nil
	}
	has, err := s.repository.ThemeHasPlacements(ctx, theme)
	if err != nil || has {
		return false, err
	}
	source, err := s.repository.Blocks(ctx, from)
	if err != nil {
		return false, err
	}
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		for _, bp := range source {
			p := bp.Placement
			if p.Region != entity.RegionDisabled && !regions.Has(p.Region) {
				continue
			}
			p.Theme = theme
			if _, err := s.repository.CopyPlacement(ctx, p); err != nil {
				return err
			}
		}
		return themes.PlaceBlocks(ctx, s.repository, theme, regions.Blocks)
	})
	if err != nil {
		return false, fmt.Errorf("copying block placements to a theme (theme %v, from %v): %w", theme, from, err)
	}
	return true, nil
}

// ActiveThemeKey - the placement theme key of the active theme: its
// placements are the ones rendered.
func ActiveThemeKey() string {
	return entity.ThemeKey(paths.Theme())
}
