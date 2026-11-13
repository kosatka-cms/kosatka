package service

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
	"github.com/kosatka-cms/kosatka/pkg/panicsafe"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// defaultCacheUpdateTimeout bounds one sidebar refresh cycle so a hung DB
// query cannot block wg.Wait() forever (SetUpdateTimeout - the config's).
const defaultCacheUpdateTimeout = 15 * time.Second

// SidebarData holds the cached sidebar and block data.
type SidebarData struct {
	LatestComments entity.Comments
	// MenuTrees - the resolved tree of every menu, keyed by MachineName.
	// Trees are request-independent (Active/ActiveTrail are applied per
	// request, see AnnotateActiveMenuTree), so all menus are cached, not
	// only placed ones. Looked up by region_blocks.html's "menu" branch.
	MenuTrees map[string][]MenuTreeItem
	// BlockPlacements - published block placements of the default theme,
	// by region. Per-page visibility is not applied here; region_blocks.html
	// filters per request.
	BlockPlacements map[entity.Region][]entity.BlockWithPlacement
	// ViewBlockContent - pre-rendered HTML per View block placement ID.
	// Templates cannot query the DB mid-render, so it is built during the
	// refresh (see resolveViewBlockContent).
	ViewBlockContent map[int]template.HTML
	// CarouselPlacements - the View block placements whose display is a
	// carousel: a page showing one loads the Swiper stylesheet
	// (HasVisibleCarouselBlock).
	CarouselPlacements map[int]bool
	// ZoneDependent - the View block placements whose output depends on
	// the time zone (they show times): rendered again for a user's own
	// zone (ViewBlockContentFor); the others are shared by everyone.
	// Probed only while user time zones are on.
	ZoneDependent map[int]bool
	// AccessDependent - the View block placements whose output differs
	// for a visitor who may view the content closed to guests (a closed
	// type's, an item's with view roles): rendered again for each other
	// visitor's access (entity.ContentVisitor.Key; SidebarDataFor); the
	// others are shared by everyone. Probed only while some content is
	// closed to guests.
	AccessDependent map[int]bool
	LastUpdated     time.Time

	// guestKey - a guest's access (entity.ContentVisitor.Key), the
	// shared data's; commentsDependent, pluginDependent - whether the
	// latest comments, which plugin placements' data differ for a visitor
	// who may view the content closed to guests (probed as
	// AccessDependent).
	guestKey          string
	commentsDependent bool
	pluginDependent   map[int]bool
	// gen - the refresh the data is of (CacheService.gen): what is built
	// from it per visitor is kept for it only (visitorCache).
	gen uint64
}

// visitorCache - a part of the cache built on demand per visitor, zone,
// theme… (by key) from one refresh's data (its gen): what a request built
// from an older refresh's isn't kept, nor what was kept read for a newer
// one - a refresh landing meanwhile would otherwise leave it till the
// next.
type visitorCache[T any] struct {
	mu      sync.Mutex
	gen     uint64
	entries map[string]T
}

// get - what is kept under key for refresh gen.
func (c *visitorCache[T]) get(gen uint64, key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[key]
	if c.gen != gen {
		var zero T
		return zero, false
	}
	return v, ok
}

// put keeps v under key for refresh gen: an older refresh's is dropped, a
// newer one's drops what was kept.
func (c *visitorCache[T]) put(gen uint64, key string, v T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen < c.gen {
		return
	}
	if gen > c.gen || c.entries == nil {
		c.gen, c.entries = gen, map[string]T{}
	}
	c.entries[key] = v
}

// reset drops what was kept (refresh gen is out).
func (c *visitorCache[T]) reset(gen uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen, c.entries = max(c.gen, gen), nil
}

// CacheService periodically caches sidebar and block data.
type CacheService struct {
	// guestVisitor - what of the content a guest may not view
	// (SetGuestVisitor); nil - nothing.
	guestVisitor func(ctx context.Context) (entity.ContentVisitor, error)
	// paused - the refresh skips its turns while it says so (SetPaused).
	paused func() bool
	data   SidebarData
	mu     sync.RWMutex
	// gen - the last refresh's number (SidebarData.gen).
	gen atomic.Uint64
	// refreshTimeout - one refresh cycle's limit (0 - the default).
	refreshTimeout time.Duration
	// zoneContent - ViewBlockContent per user time zone, visitor's access
	// and language (→ placement → HTML), filled on demand, dropped on every
	// refresh.
	zoneContent visitorCache[map[int]template.HTML]
	// accessData - the data per set of closed content types other than the
	// guests' (their names joined → SidebarData), its access-dependent
	// parts loaded for it; filled on demand, dropped on every refresh.
	accessData visitorCache[SidebarData]
	// embedded - the Views embedded in texts and templates (ViewEmbed)
	// per display, visitor's access, zone and language; filled on demand,
	// dropped on every refresh.
	embedded visitorCache[template.HTML]
	// themeData - a previewed theme's blocks (theme key → its
	// BlockPlacements, ViewBlockContent, CarouselPlacements), built on
	// demand (SidebarDataFor), dropped on every refresh.
	themeData visitorCache[SidebarData]
	updateMu  sync.Mutex // serializes update (ticker + ForceUpdate)
	// forceMu guards forceRunning/forcePending/stopped: ForceUpdate runs
	// one refresh at a time, and the calls made meanwhile add up to one
	// more; none starts once the cache has stopped (Start's ctx ended).
	forceMu      sync.Mutex
	forceRunning bool
	forcePending bool
	stopped      bool
	// forces - the ForceUpdate refreshes running (Done waits for them).
	forces sync.WaitGroup
	// rebuild - the refresh ForceUpdate runs (update; tests replace it).
	rebuild        func(ctx context.Context) error
	updateInterval time.Duration
	// itemsLimit - how many latest comments the cache keeps.
	itemsLimit int
	// done - closed when the refresh loop Start began has returned.
	done            chan struct{}
	commentsService CommentsService
	menuService     MenuService
	blockService    BlockService
	// viewService/templateService - used to render View blocks.
	// nil-tolerant (as are the other optional dependencies below), so
	// tests that don't exercise View blocks need not stub them.
	viewService     views.Service
	templateService TemplateService
	// teasers - teaser items for View blocks with a theme template
	// (SetTeasers); nil - such blocks render their fields.
	teasers *TeaserBuilder
	// files - View-block image fields' files (SetFiles).
	files views.FileLoader
	users views.UsersLoader
	// contentTypeRepo - resolves generic View block fields and the
	// content type builtin field; when nil, such fields render empty.
	contentTypeRepo entity.ContentTypeRepository
	// taxonomyService - term names for taxonomy fields of View block rows.
	taxonomyService TaxonomyService
	logger          *slog.Logger
}

// NewCacheService creates a CacheService.
func NewCacheService(
	updateInterval time.Duration,
	itemsLimit int,
	commentsService CommentsService,
	menuService MenuService,
	blockService BlockService,
	viewService views.Service,
	templateService TemplateService,
	contentTypeRepo entity.ContentTypeRepository,
	taxonomyService TaxonomyService,
	baseLogger *slog.Logger,
) *CacheService {
	return &CacheService{
		updateInterval:  updateInterval,
		itemsLimit:      itemsLimit,
		done:            make(chan struct{}),
		commentsService: commentsService,
		menuService:     menuService,
		blockService:    blockService,
		viewService:     viewService,
		templateService: templateService,
		contentTypeRepo: contentTypeRepo,
		taxonomyService: taxonomyService,
		logger:          baseLogger.With("component", "cache.service"),
	}
}

// Start fills the cache and starts the background refresh, until ctx
// ends (Done).
func (s *CacheService) Start(ctx context.Context) error {
	if err := s.update(ctx); err != nil {
		return err
	}

	// A refresh under way when ctx ends finishes (update bounds it by the
	// cache update timeout): the loop and Done wait for it.
	work := context.WithoutCancel(ctx)
	ticker := time.NewTicker(s.updateInterval)
	go func() {
		defer close(s.done)
		defer ticker.Stop()
		defer panicsafe.Recover(s.logger, "cache.ticker")
		for {
			select {
			case <-ticker.C:
				if s.paused != nil && s.paused() {
					continue
				}
				if err := s.update(work); err != nil {
					s.logger.Error("Error updating cache", "error", err)
				}
			case <-ctx.Done():
				s.stop()
				return
			}
		}
	}()

	s.logger.Info(fmt.Sprintf("CacheService started with interval: %v", s.updateInterval))
	return nil
}

// stop - no ForceUpdate starts any more; returns once the running one
// has.
func (s *CacheService) stop() {
	s.forceMu.Lock()
	s.stopped = true
	s.forceMu.Unlock()
	s.forces.Wait()
}

// Done - closed once the refresh loop and the ForceUpdate refreshes
// running have returned (Start's ctx ended): nothing of the cache's reads
// the database after it.
func (s *CacheService) Done() <-chan struct{} { return s.done }

// asGuest - ctx whose node and comment queries see what a guest may
// (guestVisitor): for what is cached for everyone.
func (s *CacheService) asGuest(ctx context.Context) (context.Context, error) {
	if s.guestVisitor == nil {
		return entity.WithContentVisitor(ctx, entity.ContentVisitor{}), nil
	}
	v, err := s.guestVisitor(ctx)
	if err != nil {
		return nil, fmt.Errorf("the content closed to guests: %w", err)
	}
	return entity.WithContentVisitor(ctx, v), nil
}

// SetPaused - the refresh skips its turns while paused says so (the
// database doesn't answer): the cache keeps what it has. Before Start.
func (s *CacheService) SetPaused(paused func() bool) { s.paused = paused }

// SetGuestVisitor sets what of the content a guest may not view
// (AuthService.ContentVisitor for nil), left out of what the cache
// renders for everyone (wiring).
func (s *CacheService) SetGuestVisitor(f func(ctx context.Context) (entity.ContentVisitor, error)) {
	s.guestVisitor = f
}

// update reloads all cached data.
func (s *CacheService) update(ctx context.Context) error {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()

	updateCtx, cancel := context.WithTimeout(ctx, s.updateTimeout())
	defer cancel()
	// Rendered once for everyone: as for a guest - no content of a type
	// closed to guests gets into a cached block.
	updateCtx, err := s.asGuest(updateCtx)
	if err != nil {
		return err
	}

	s.logger.Info("Updating cache...")
	// What a part failing to load keeps (rather than pages without menus
	// or a region's blocks till the next refresh).
	s.mu.RLock()
	prev := s.data
	s.mu.RUnlock()

	// Fetch in parallel.
	var wg sync.WaitGroup
	var comments entity.Comments
	var menuTrees map[string][]MenuTreeItem
	var blockPlacements map[entity.Region][]entity.BlockWithPlacement
	var errs []error
	var errMu sync.Mutex

	wg.Add(3)

	go func() {
		defer wg.Done()
		defer panicsafe.Recover(s.logger, "cache.update.comments")
		var err error
		comments, err = s.commentsService.GetLatest(updateCtx, s.itemsLimit, 1)
		if err != nil {
			errMu.Lock()
			errs = append(errs, fmt.Errorf("loading the latest comments: %w", err))
			errMu.Unlock()
		}
	}()

	// Resolve every menu's tree (see SidebarData.MenuTrees).
	go func() {
		defer wg.Done()
		defer panicsafe.Recover(s.logger, "cache.update.menu_trees")
		if s.menuService == nil {
			return
		}
		menus, err := s.menuService.Menus(updateCtx)
		if err != nil {
			s.logger.Error("error listing menus, keeping the previous ones", "error", err)
			menuTrees = prev.MenuTrees
			return
		}
		out := make(map[string][]MenuTreeItem, len(menus))
		for _, m := range menus {
			tree, err := s.menuService.ResolvedTree(updateCtx, m.MachineName)
			if err != nil {
				// Deleted after Menus listed it: nothing to cache.
				if IsNotFound(err) {
					s.logger.Debug("menu gone while the cache refreshed", "error", err, "machine_name", m.MachineName)
					continue
				}
				s.logger.Error("error resolving menu tree, keeping the previous one", "error", err, "machine_name", m.MachineName)
				if tree, ok := prev.MenuTrees[m.MachineName]; ok {
					out[m.MachineName] = tree
				}
				continue
			}
			out[m.MachineName] = tree
		}
		menuTrees = out
	}()

	// Resolve published block placements for every region of the active
	// theme. If theme.yaml can't be read, keep the previous placements
	// rather than rendering pages without blocks.
	go func() {
		defer wg.Done()
		defer panicsafe.Recover(s.logger, "cache.update.block_placements")
		if s.blockService == nil {
			return
		}
		theme, err := themes.LoadInfo()
		if err != nil {
			s.logger.Error("error loading theme regions, keeping the previous block placements", "error", err)
			blockPlacements = prev.BlockPlacements
			return
		}
		out := make(map[entity.Region][]entity.BlockWithPlacement, len(theme.Regions))
		for _, region := range theme.RegionIDs() {
			placements, err := s.blockService.PublishedPlacementsByRegion(updateCtx, ActiveThemeKey(), region)
			if err != nil {
				s.logger.Error("error resolving block placements, keeping the region's previous ones", "error", err, "region", region)
				if placements, ok := prev.BlockPlacements[region]; ok {
					out[region] = placements
				}
				continue
			}
			out[region] = placements
		}
		blockPlacements = out
	}()

	wg.Wait()

	// Only the comments fetch reports errors; the others just log.
	if len(errs) > 0 {
		return errs[0]
	}

	// Runs after wg.Wait(): they need blockPlacements.
	blockPlacements = s.resolvePluginBlockData(updateCtx, blockPlacements)
	blocks := s.resolveViewBlockContent(updateCtx, blockPlacements, true)

	data := SidebarData{
		LatestComments:     comments,
		MenuTrees:          menuTrees,
		BlockPlacements:    blockPlacements,
		ViewBlockContent:   blocks.content,
		CarouselPlacements: blocks.carousels,
		ZoneDependent:      blocks.zoneDependent,
		AccessDependent:    blocks.accessDependent,
		LastUpdated:        time.Now(),
	}
	// What a visitor who may view the content closed to guests would see
	// otherwise: their own variant (SidebarDataFor) loads only that.
	if guest, _ := entity.VisitorOf(updateCtx); guest.Restricts() {
		data.guestKey = guest.Key()
		allCtx := entity.WithAllNodeTypes(updateCtx)
		if all, err := s.commentsService.GetLatest(allCtx, s.itemsLimit, 1); err != nil || !sameComments(all, comments) {
			data.commentsDependent = true
		}
		data.pluginDependent = s.pluginAccessDependent(allCtx, blockPlacements)
	}

	s.mu.Lock()
	data.gen = s.gen.Add(1)
	s.data = data
	s.mu.Unlock()
	s.zoneContent.reset(data.gen)
	s.accessData.reset(data.gen)
	s.embedded.reset(data.gen)
	s.themeData.reset(data.gen)

	s.logger.Info(fmt.Sprintf("Cache updated successfully at %v", time.Now()))
	return nil
}

// SetUsers gives View blocks their user fields' users (wiring).
func (s *CacheService) SetUsers(u views.UsersLoader) {
	s.users = u
}

// SetFiles gives View blocks their image fields' files (wiring).
func (s *CacheService) SetFiles(f views.FileLoader) {
	s.files = f
}

// SetTeasers gives View blocks with a theme template their teaser
// items (wiring).
func (s *CacheService) SetTeasers(t *TeaserBuilder) {
	s.teasers = t
}

// resolvePluginBlockData - blockPlacements with each plugin placement's
// PluginData loaded (SetPluginBlockData), as new slices: the given ones
// may be the cache's previous, read by pages meanwhile. A placement whose
// data fails to load keeps none (its block shows nothing).
func (s *CacheService) resolvePluginBlockData(ctx context.Context, blockPlacements map[entity.Region][]entity.BlockWithPlacement) map[entity.Region][]entity.BlockWithPlacement {
	if blockPlacements == nil {
		return nil
	}
	out := make(map[entity.Region][]entity.BlockWithPlacement, len(blockPlacements))
	for region, placements := range blockPlacements {
		items := make([]entity.BlockWithPlacement, len(placements))
		for i, bp := range placements {
			bp.PluginData = nil
			if bp.Placement.IsPlugin() {
				if load, ok := PluginBlockDataLoader(bp.Placement.PluginIDValue()); ok {
					data, err := load(ctx, bp.Placement)
					if err != nil {
						s.logger.Error("error loading plugin block data", "error", err, "plugin", bp.Placement.PluginIDValue(), "placementID", bp.Placement.ID)
					} else {
						bp.PluginData = data
					}
				}
			}
			items[i] = bp
		}
		out[region] = items
	}
	return out
}

// sameComments reports whether a and b are the same comments, in order.
func sameComments(a, b entity.Comments) bool {
	return slices.EqualFunc(a, b, func(x, y entity.Comment) bool { return x.ID == y.ID })
}

// pluginAccessDependent - the plugin placements of blockPlacements whose
// data (loaded as ctx's visitor's) differs from theirs; a failed load
// counts as different.
func (s *CacheService) pluginAccessDependent(ctx context.Context, blockPlacements map[entity.Region][]entity.BlockWithPlacement) map[int]bool {
	dependent := map[int]bool{}
	for _, placements := range blockPlacements {
		for _, bp := range placements {
			if !bp.Placement.IsPlugin() {
				continue
			}
			load, ok := PluginBlockDataLoader(bp.Placement.PluginIDValue())
			if !ok {
				continue
			}
			if data, err := load(ctx, bp.Placement); err != nil || !reflect.DeepEqual(data, bp.PluginData) {
				dependent[bp.Placement.ID] = true
			}
		}
	}
	return dependent
}

// viewBlocks - the View blocks of a refresh (resolveViewBlockContent):
// each placement's pre-rendered HTML, and its carousel, zone-dependent and
// access-dependent placements.
type viewBlocks struct {
	content                                   map[int]template.HTML
	carousels, zoneDependent, accessDependent map[int]bool
}

// resolveViewBlockContent pre-renders each View block placement once per
// refresh; region_blocks.html looks the fragments up by placement ID.
// probeAccess: also find which ones differ for a visitor who may view the
// content types ctx leaves out (rendered again as theirs).
func (s *CacheService) resolveViewBlockContent(ctx context.Context, blockPlacements map[entity.Region][]entity.BlockWithPlacement, probeAccess bool) viewBlocks {
	if s.viewService == nil || s.templateService == nil {
		return viewBlocks{}
	}
	out := make(map[int]template.HTML)
	carousels := make(map[int]bool)
	dependent := make(map[int]bool)
	accessDependent := make(map[int]bool)
	probe := zoneProbe()
	var allCtx context.Context
	if v, _ := entity.VisitorOf(ctx); probeAccess && v.Restricts() {
		allCtx = entity.WithAllNodeTypes(ctx)
	}
	for _, placements := range blockPlacements {
		for _, bp := range placements {
			if !bp.Placement.IsView() || bp.ViewDisplay == nil {
				continue
			}
			// The display as visitors see it (its published version).
			cfg, err := s.viewService.PublishedConfig(ctx, *bp.ViewDisplay)
			if err != nil {
				s.logger.Error("error loading view settings for block", "error", err, "viewDisplayID", bp.ViewDisplay.ID, "placementID", bp.Placement.ID)
				continue
			}
			// Displays with contextual filters depend on the viewed node,
			// so they are skipped here and rendered per request by
			// DynamicContentController.attachContextualViewBlocks.
			if cfg.Filters.HasContextual() {
				continue
			}
			html, err := s.renderViewBlock(ctx, cfg, viewBlockPart{placementID: bp.Placement.ID, page: 1}, nil)
			if err != nil {
				s.logger.Error("error rendering view block", "error", err, "viewDisplayID", bp.ViewDisplay.ID, "placementID", bp.Placement.ID)
				continue
			}
			out[bp.Placement.ID] = html
			// Does the block show times? Render it in a far-off zone too:
			// a different output means it does (user time zones only).
			if probe != nil {
				if probed, err := s.renderViewBlock(ctx, cfg, viewBlockPart{placementID: bp.Placement.ID, page: 1}, probe); err == nil && probed != html {
					dependent[bp.Placement.ID] = true
				}
			}
			// Does it show content of a type closed to guests? Render it for
			// a visitor who may view every type: a different output (or
			// none) means it does.
			if allCtx != nil {
				if all, err := s.renderViewBlock(allCtx, cfg, viewBlockPart{placementID: bp.Placement.ID, page: 1}, nil); err != nil || all != html {
					accessDependent[bp.Placement.ID] = true
				}
			}
			if cfg.View.DisplayStyle == entity.ViewDisplayStyleCarousel {
				carousels[bp.Placement.ID] = true
			}
		}
	}
	return viewBlocks{content: out, carousels: carousels, zoneDependent: dependent, accessDependent: accessDependent}
}

// zoneProbe - a time zone far from the site's to tell whether a block's
// output depends on the zone; nil while user time zones are off.
func zoneProbe() *time.Location {
	if !sitetime.UserZonesEnabled() {
		return nil
	}
	name := "Pacific/Kiritimati" // UTC+14
	if sitetime.Location().String() == name {
		name = "Pacific/Pago_Pago" // UTC-11
	}
	loc, err := sitetime.Load(name)
	if err != nil {
		return time.FixedZone("UTC+14", 14*3600)
	}
	return loc
}

// viewBlockPart - what of a View block to render: its page of placement
// placementID, the whole block's content or (body) only the list and pager
// a page's link swaps in.
type viewBlockPart struct {
	placementID int
	page        int
	body        bool
}

// ViewBlockPagePath - the address of placement placementID's pages, a
// paged View block's links (GET /api/view-block/{id}).
func ViewBlockPagePath(placementID int) string {
	return fmt.Sprintf("/api/view-block/%d", placementID)
}

// viewBlockPageSize - how many of the pager's page numbers a block shows
// (a sidebar is narrow).
const viewBlockPageSize = 5

// viewBlockPager - the pager of a View block whose display pages (view -
// its settings resolved), or nil: its links load the pages through
// ViewBlockPagePath, in place (app.js, data-ajax-pagination).
func viewBlockPager(view entity.ViewDisplayDef, part viewBlockPart, total int) *pagination.Pagination {
	if part.placementID == 0 || !view.ShowPagination || view.PageSize <= 0 {
		return nil
	}
	return pagination.NewWithOptions(part.page, total, view.PageSize, viewBlockPageSize, ViewBlockPagePath(part.placementID), nil, "")
}

// pagedThemeBlock - a theme template's output of a paged View block in
// the container its page links swap (the body alone for a page's link).
func pagedThemeBlock(html template.HTML, pager *pagination.Pagination, part viewBlockPart) template.HTML {
	if pager == nil || part.body {
		return html
	}
	return template.HTML(`<div class="view-block-pages" data-ajax-pagination="1">`) + html + template.HTML(`</div>`)
}

// renderViewBlockOf - renderViewBlock of display as visitors see it.
func (s *CacheService) renderViewBlockOf(ctx context.Context, display entity.ViewDisplayDef, part viewBlockPart, zone *time.Location) (template.HTML, error) {
	cfg, err := s.viewService.PublishedConfig(ctx, display)
	if err != nil {
		return "", fmt.Errorf("loading the settings of a View block (display %d): %w", display.ID, err)
	}
	return s.renderViewBlock(ctx, cfg, part, zone)
}

// renderViewBlock - part of one View block (cfg - its display as visitors
// see it: views.Service.PublishedConfig) in zone (nil - the site's).
func (s *CacheService) renderViewBlock(ctx context.Context, cfg views.Config, part viewBlockPart, zone *time.Location) (template.HTML, error) {
	if cfg.View.IsComment() {
		return s.renderCommentViewBlock(ctx, cfg, part, zone)
	}
	return s.renderContentViewBlock(ctx, cfg, part, zone)
}

// ViewBlockPage - page of the View block of placement placementID for
// GET /api/view-block/{id}: the list and its pager, in zone; false when
// its display doesn't page or hangs on the viewed content (contextual
// filters: rendered with the page, not cached).
func (s *CacheService) ViewBlockPage(ctx context.Context, view entity.ViewDisplayDef, placementID, page int, zone *time.Location) (template.HTML, bool, error) {
	if s.viewService == nil || s.templateService == nil {
		return "", false, nil
	}
	cfg, err := s.viewService.PublishedConfig(ctx, view)
	if err != nil {
		return "", false, fmt.Errorf("loading the settings of a View block (display %d): %w", view.ID, err)
	}
	if cfg.Filters.HasContextual() {
		return "", false, nil
	}
	part := viewBlockPart{placementID: placementID, page: page, body: true}
	if viewBlockPager(cfg.View, part, 0) == nil {
		return "", false, nil
	}
	html, err := s.renderViewBlock(ctx, cfg, part, zone)
	return html, err == nil, err
}

// ViewBlockContentFor - the View blocks' content for a page in zone,
// for ctx's visitor (SidebarDataFor): the shared content for the site's
// zone and language; for a user's own zone the zone-dependent blocks
// rendered in it, for another language than the site's (the address's,
// entity.LanguageOfPrefix) every View block rendered in it (cached per
// zone, language and set of closed types until the next refresh), the
// rest as for the site's.
func (s *CacheService) ViewBlockContentFor(ctx context.Context, zone *time.Location) map[int]template.HTML {
	data := s.SidebarDataFor(ctx)
	lang := i18n.FromContext(ctx)
	otherLang := lang != i18n.Default()
	// A previewed theme's blocks: in the site's zone.
	if _, ok := paths.PreviewTheme(ctx); ok || (!otherLang && (sitetime.IsSite(zone) || len(data.ZoneDependent) == 0)) {
		return data.ViewBlockContent
	}
	accessCtx, access, own := accessVisitor(ctx, data)
	key := zone.String() + "|" + access + "|" + lang
	if cached, ok := s.zoneContent.get(data.gen, key); ok {
		return cached
	}
	// Cached for everyone in the zone with the visitor's access: rendered
	// as theirs, or as for a guest.
	if own {
		ctx = accessCtx
	} else {
		var err error
		if ctx, err = s.asGuest(ctx); err != nil {
			s.logger.Error("the content types closed to guests, for a time zone's blocks", "error", err, "zone", key)
			return data.ViewBlockContent
		}
	}
	out := maps.Clone(data.ViewBlockContent)
	if out == nil {
		out = map[int]template.HTML{}
	}
	for _, placements := range data.BlockPlacements {
		for _, bp := range placements {
			if bp.ViewDisplay == nil || (!otherLang && !data.ZoneDependent[bp.Placement.ID]) {
				continue
			}
			html, err := s.renderViewBlockOf(ctx, *bp.ViewDisplay, viewBlockPart{placementID: bp.Placement.ID, page: 1}, zone)
			if err != nil {
				s.logger.Error("error rendering view block for a time zone", "error", err, "placementID", bp.Placement.ID, "zone", key)
				continue
			}
			out[bp.Placement.ID] = html
		}
	}
	s.zoneContent.put(data.gen, key, out)
	return out
}

// renderCommentViewBlock renders a comment-based View block through the
// "content" define of dynamic/comment_list.html, the same define the page
// render and admin preview use (part.body: its "comment_list_body"). No
// draft banner (only published View blocks reach here); a pager when the
// display pages (viewBlockPager).
func (s *CacheService) renderCommentViewBlock(ctx context.Context, cfg views.Config, part viewBlockPart, zone *time.Location) (template.HTML, error) {
	view := cfg.View
	comments, total, err := s.viewService.RunCommentConfig(ctx, cfg, views.RunContext{Page: part.page})
	if err != nil {
		return "", fmt.Errorf("error running comment view for block: %w", err)
	}
	pager := viewBlockPager(view, part, total)
	if view.ThemeTemplate != "" && views.ThemeTemplateExists(ctx, view.ThemeTemplate) {
		html, err := views.RenderThemeTemplate(ctx, s.templateService, view.ThemeTemplate, map[string]any{"comments": comments, "pagerCompact": true, RequestLocationKey: zone, RequestLanguageKey: i18n.FromContext(ctx), RequestThemeKey: RequestTheme(ctx)}, pager)
		return pagedThemeBlock(views.WithViewTexts(view, html), pager, part), err
	}
	data := views.CommentListData(ctx, view, cfg.Fields, comments)
	data["viewUnpublished"] = false
	data["showPagination"] = pager != nil
	data["pagination"] = pager
	data["paginationAjax"] = pager != nil
	data["pagerCompact"] = true
	// isViewBlock/viewID give the block a container id unique per View
	// (see renderContentViewBlock).
	data["isViewBlock"] = true
	data["viewID"] = view.ID

	var buf bytes.Buffer
	data[RequestLocationKey] = zone
	data[RequestLanguageKey] = i18n.FromContext(ctx)
	data[RequestThemeKey] = RequestTheme(ctx)
	define := "content"
	if part.body {
		define = "comment_list_body"
	}
	if err := s.templateService.RenderNamed(&buf, define, data, "dynamic/comment_list.html", "pagination.html"); err != nil {
		return "", fmt.Errorf("error rendering comment view block: %w", err)
	}
	return template.HTML(buf.String()), nil
}

// renderContentViewBlock renders a content-based View block through the
// "content" define of dynamic/list.html (part.body: its "view_list_body"),
// as ViewPageFallback does for a page, but without glossary or exposed
// filters; a pager when the display pages (viewBlockPager). Field
// metadata is resolved once per block, not per node.
func (s *CacheService) renderContentViewBlock(ctx context.Context, cfg views.Config, part viewBlockPart, zone *time.Location) (template.HTML, error) {
	view := cfg.View
	nodes, total, err := s.viewService.RunConfig(ctx, cfg, views.RunContext{Page: part.page})
	if err != nil {
		return "", fmt.Errorf("error running content view for block: %w", err)
	}
	pager := viewBlockPager(view, part, total)
	// A theme template (ViewDisplayDef.ThemeTemplate) renders the block
	// instead of the field list, as on View pages.
	if view.ThemeTemplate != "" && s.teasers != nil && views.ThemeTemplateExists(ctx, view.ThemeTemplate) {
		items, err := s.teasers.Build(ctx, nodes)
		if err != nil {
			return "", fmt.Errorf("error building teaser items for view block: %w", err)
		}
		html, err := views.RenderThemeTemplate(ctx, s.templateService, view.ThemeTemplate, map[string]any{"items": items, "pagerCompact": true, RequestLocationKey: zone, RequestLanguageKey: i18n.FromContext(ctx), RequestThemeKey: RequestTheme(ctx)}, pager)
		return pagedThemeBlock(views.WithViewTexts(view, html), pager, part), err
	}
	viewFields := cfg.Fields
	listFields, items, err := views.BuildContentList(ctx, views.ContentListSources{Types: s.contentTypeRepo,
		Terms: s.taxonomyService, Files: s.files, Comments: s.commentsService, Users: s.users, Logger: s.logger}, view.ID, viewFields, nodes)
	if err != nil {
		return "", fmt.Errorf("error building content view block items: %w", err)
	}

	carouselLinkURLs, err := views.CarouselLinkURLs(ctx, s.contentTypeRepo, view.CarouselLinkFieldStorageIDValue(), nodes)
	if err != nil {
		// Slides without their links: the block still shows.
		LogFallback(s.logger, err, "resolving the carousel slides' links for a View block", "viewID", view.ID)
	}
	data := views.ContentListData(view, listFields, items, carouselLinkURLs)
	// isViewBlock/viewID give each block a container id unique per View,
	// so a page View's CustomCSS scoped under #view-list-container doesn't
	// leak into blocks, and blocks on the same page don't collide.
	data["isViewBlock"] = true
	data["viewID"] = view.ID
	data["showPagination"] = pager != nil
	data["pagination"] = pager
	data["paginationAjax"] = pager != nil
	data["pagerCompact"] = true

	var buf bytes.Buffer
	data[RequestLocationKey] = zone
	data[RequestLanguageKey] = i18n.FromContext(ctx)
	data[RequestThemeKey] = RequestTheme(ctx)
	define := "content"
	if part.body {
		define = "view_list_body"
	}
	if err := s.templateService.RenderNamed(&buf, define, data, "dynamic/list.html", "pagination.html"); err != nil {
		return "", fmt.Errorf("error rendering content view block: %w", err)
	}
	return template.HTML(buf.String()), nil
}

// UpdateInterval - how often the cache refreshes.
func (s *CacheService) UpdateInterval() time.Duration { return s.updateInterval }

// GetSidebarData returns the cached data (none without a cache, e.g. in a
// handler's test).
func (s *CacheService) GetSidebarData() SidebarData {
	if s == nil {
		return SidebarData{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// SidebarDataFor - GetSidebarData for ctx: a previewed theme
// (paths.WithTheme) gets its own block placements and View blocks
// (rendered in it, as for ctx's visitor - a guest without one), built on
// first use per theme and kind of access and kept until the next
// refresh; a visitor who may view other content than a guest gets the
// parts that differ for them (forAccess); else the shared data.
func (s *CacheService) SidebarDataFor(ctx context.Context) SidebarData {
	data := s.GetSidebarData()
	if s == nil {
		return data
	}
	dir, ok := paths.PreviewTheme(ctx)
	if !ok {
		return s.forAccess(ctx, data)
	}
	// The previewer's own access (their own content aside, as the shared
	// variants), or a guest's without a visitor.
	key := entity.ThemeKey(dir)
	var buildCtx context.Context
	if v, ok := entity.VisitorOf(ctx); ok {
		key += "|" + v.Key()
		buildCtx = entity.WithContentVisitor(ctx, v.Shared())
	} else {
		var err error
		if buildCtx, err = s.asGuest(ctx); err != nil {
			s.logger.Error("the content closed to guests, for a previewed theme's blocks", "error", err)
			return data
		}
	}
	// The latest comments as the previewer may see them.
	data = s.forAccess(ctx, data)
	blocks, ok := s.themeData.get(data.gen, key)
	if !ok {
		blocks = s.themeBlocks(buildCtx, dir)
		s.themeData.put(data.gen, key, blocks)
	}
	data.BlockPlacements = blocks.BlockPlacements
	data.ViewBlockContent = blocks.ViewBlockContent
	data.CarouselPlacements = blocks.CarouselPlacements
	data.ZoneDependent = nil
	data.AccessDependent, data.commentsDependent, data.pluginDependent = nil, false, nil
	return data
}

// accessVisitor - the key of ctx's visitor's variant of data (their
// access, entity.ContentVisitor.Key) and the ctx to load it in (theirs,
// without their own items: the variant is every such visitor's);
// own=false when they get the shared data: no visitor (a background
// job), a guest's access, or nothing in data differs for them.
func accessVisitor(ctx context.Context, data SidebarData) (context.Context, string, bool) {
	if len(data.AccessDependent) == 0 && !data.commentsDependent && len(data.pluginDependent) == 0 {
		return ctx, "", false
	}
	v, ok := entity.VisitorOf(ctx)
	if !ok || v.Key() == data.guestKey {
		return ctx, "", false
	}
	return entity.WithContentVisitor(ctx, v.Shared()), v.Key(), true
}

// forAccess - data for ctx's visitor (accessVisitor): the latest comments,
// plugin blocks' data and View blocks that differ for them loaded as
// theirs, on first use, and kept until the next refresh. A part that
// fails to load stays as for a guest (and the variant isn't kept).
func (s *CacheService) forAccess(ctx context.Context, data SidebarData) SidebarData {
	ctx, key, own := accessVisitor(ctx, data)
	if !own {
		return data
	}
	if cached, ok := s.accessData.get(data.gen, key); ok {
		return cached
	}
	complete := true
	if data.commentsDependent {
		if comments, err := s.commentsService.GetLatest(ctx, s.itemsLimit, 1); err != nil {
			LogFallback(s.logger, err, "loading the latest comments for a visitor's content types (as for a guest)")
			complete = false
		} else {
			data.LatestComments = comments
		}
	}
	if len(data.pluginDependent) > 0 {
		placements := make(map[entity.Region][]entity.BlockWithPlacement, len(data.BlockPlacements))
		for region, list := range data.BlockPlacements {
			items := slices.Clone(list)
			for i, bp := range items {
				if !data.pluginDependent[bp.Placement.ID] {
					continue
				}
				load, _ := PluginBlockDataLoader(bp.Placement.PluginIDValue())
				if load == nil {
					continue
				}
				if pd, err := load(ctx, bp.Placement); err != nil {
					LogFallback(s.logger, err, "loading a plugin block's data for a visitor's content types (as for a guest)", "placementID", bp.Placement.ID)
					complete = false
				} else {
					items[i].PluginData = pd
				}
			}
			placements[region] = items
		}
		data.BlockPlacements = placements
	}
	if len(data.AccessDependent) > 0 {
		content := maps.Clone(data.ViewBlockContent)
		for _, list := range data.BlockPlacements {
			for _, bp := range list {
				if !data.AccessDependent[bp.Placement.ID] || bp.ViewDisplay == nil {
					continue
				}
				html, err := s.renderViewBlockOf(ctx, *bp.ViewDisplay, viewBlockPart{placementID: bp.Placement.ID, page: 1}, nil)
				if err != nil {
					LogFallback(s.logger, err, "rendering a View block for a visitor's content types (as for a guest)", "placementID", bp.Placement.ID)
					complete = false
					continue
				}
				content[bp.Placement.ID] = html
			}
		}
		data.ViewBlockContent = content
	}
	if complete {
		s.accessData.put(data.gen, key, data)
	}
	return data
}

// themeBlocks - the published block placements of theme dir's regions and
// its View blocks rendered in it (ctx renders in dir).
func (s *CacheService) themeBlocks(ctx context.Context, dir string) SidebarData {
	var out SidebarData
	if s.blockService == nil {
		return out
	}
	info, err := themes.LoadInfoFor(dir)
	if err != nil {
		s.logger.Error("error loading the previewed theme's regions", "error", err, "theme", dir)
		return out
	}
	out.BlockPlacements = make(map[entity.Region][]entity.BlockWithPlacement, len(info.Regions))
	for _, region := range info.RegionIDs() {
		placements, err := s.blockService.PublishedPlacementsByRegion(ctx, entity.ThemeKey(dir), region)
		if err != nil {
			s.logger.Error("error resolving the previewed theme's block placements", "error", err, "theme", dir, "region", region)
			continue
		}
		out.BlockPlacements[region] = placements
	}
	out.BlockPlacements = s.resolvePluginBlockData(ctx, out.BlockPlacements)
	blocks := s.resolveViewBlockContent(ctx, out.BlockPlacements, false)
	out.ViewBlockContent, out.CarouselPlacements = blocks.content, blocks.carousels
	return out
}

// ForceUpdate refreshes the cache asynchronously, e.g. after a new
// comment or a saved block. One refresh runs at a time; the calls made
// while it runs add up to one more after it (a run of admin saves
// rebuilds the cache twice, not once per save).
func (s *CacheService) ForceUpdate() {
	s.forceMu.Lock()
	if s.stopped {
		s.forceMu.Unlock()
		return
	}
	if s.forceRunning {
		s.forcePending = true
		s.forceMu.Unlock()
		return
	}
	s.forceRunning = true
	s.forces.Add(1)
	s.forceMu.Unlock()
	rebuild := s.rebuild
	if rebuild == nil {
		rebuild = s.update
	}
	go func() {
		defer s.forces.Done()
		done := false
		defer func() {
			if !done { // a panic: let the next call start a refresh
				s.forceMu.Lock()
				s.forceRunning, s.forcePending = false, false
				s.forceMu.Unlock()
			}
		}()
		defer panicsafe.Recover(s.logger, "cache.force_update")
		for {
			// A refresh outlives the request that asked for it (update
			// bounds it by the cache update timeout).
			if err := rebuild(context.Background()); err != nil {
				s.logger.Error("Error in force update", "error", err)
			}
			s.forceMu.Lock()
			if !s.forcePending {
				s.forceRunning = false
				s.forceMu.Unlock()
				done = true
				return
			}
			s.forcePending = false
			s.forceMu.Unlock()
		}
	}()
}

// RefreshNow reloads the cache synchronously (admin "clear cache" action).
func (s *CacheService) RefreshNow(ctx context.Context) error {
	return s.update(ctx)
}

// SetUpdateTimeout sets the limit of one refresh cycle (config
// timeouts.cache_update); called at startup.
func (s *CacheService) SetUpdateTimeout(d time.Duration) { s.refreshTimeout = d }

func (s *CacheService) updateTimeout() time.Duration {
	if s.refreshTimeout > 0 {
		return s.refreshTimeout
	}
	return defaultCacheUpdateTimeout
}

// ViewEmbed - the kind "view": a View block display by its machine name,
// its list as the block shows it (its first page, no pager), for ctx's
// visitor in their zone and language (their own content aside: what is
// kept is every such visitor's); kept until the next refresh. A display
// of another type, disabled, or hanging on the viewed content
// (contextual filters) - nothing.
func (s *CacheService) ViewEmbed() EmbedKind {
	return EmbedKind{Name: "view", Label: i18n.N("List (View)"), Search: s.searchViewEmbeds, Describe: s.describeViewEmbed, Render: func(ctx context.Context, name string) (template.HTML, error) {
		if s.viewService == nil || s.templateService == nil {
			return "", nil
		}
		view, err := s.viewService.ViewByMachineName(ctx, name)
		if err != nil {
			return "", err
		}
		if view.DisplayType != entity.ViewDisplayTypeBlock || view.Status != 1 {
			return "", nil
		}
		cfg, err := s.viewService.PublishedConfig(ctx, *view)
		if err != nil {
			return "", fmt.Errorf("loading the settings of the embedded View %q: %w", name, err)
		}
		if cfg.Filters.HasContextual() {
			return "", nil
		}
		// The refresh it is rendered after (kept for it only).
		gen := s.gen.Load()
		zone := sitetime.FromContext(ctx)
		// No visitor (no request's): as for a guest.
		if _, ok := entity.VisitorOf(ctx); !ok {
			if ctx, err = s.asGuest(ctx); err != nil {
				return "", err
			}
		}
		visitor, _ := entity.VisitorOf(ctx)
		ctx = entity.WithContentVisitor(ctx, visitor.Shared())
		key := fmt.Sprintf("%d|%s|%s|%s", view.ID, visitor.Key(), zone, i18n.FromContext(ctx))
		if html, ok := s.embedded.get(gen, key); ok {
			return html, nil
		}
		html, err := s.renderViewBlock(ctx, cfg, viewBlockPart{page: 1}, zone)
		if err != nil {
			return "", fmt.Errorf("rendering the embedded View %q: %w", name, err)
		}
		s.embedded.put(gen, key, html)
		return html, nil
	}}
}

// embeddableView - whether view is a View block display a text can embed:
// a block display, enabled, not hanging on the viewed content.
func (s *CacheService) embeddableView(ctx context.Context, view entity.ViewDisplayDef) bool {
	if view.DisplayType != entity.ViewDisplayTypeBlock || view.Status != 1 || view.IsDefault {
		return false
	}
	cfg, err := s.viewService.PublishedConfig(ctx, view)
	return err == nil && !cfg.Filters.HasContextual()
}

// searchViewEmbeds - the View block displays a text can embed whose label
// or machine name has query, by label (the "view" kind's Search).
func (s *CacheService) searchViewEmbeds(ctx context.Context, query string, limit int) ([]EmbedOption, error) {
	if s.viewService == nil {
		return nil, nil
	}
	views, err := s.viewService.Views(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the Views: %w", err)
	}
	slices.SortFunc(views, func(a, b entity.ViewDisplayDef) int { return strings.Compare(a.Label, b.Label) })
	q := strings.ToLower(query)
	var out []EmbedOption
	for _, v := range views {
		if len(out) >= limit {
			break
		}
		if q != "" && !strings.Contains(strings.ToLower(v.Label), q) && !strings.Contains(v.MachineName, q) {
			continue
		}
		if s.embeddableView(ctx, v) {
			out = append(out, EmbedOption{Name: v.MachineName, Title: v.Label, Meta: v.MachineName})
		}
	}
	return out, nil
}

// embedPreviewLines - how many of a list's items the editor's preview shows.
const embedPreviewLines = 3

// describeViewEmbed - a View block display for the editor: its first
// items' titles (as ctx's visitor sees the list) and how many more (the
// "view" kind's Describe).
func (s *CacheService) describeViewEmbed(ctx context.Context, name string) (EmbedOption, error) {
	if s.viewService == nil {
		return EmbedOption{}, entity.ErrEntityNotFound
	}
	view, err := s.viewService.ViewByMachineName(ctx, name)
	if err != nil {
		return EmbedOption{}, err
	}
	opt := EmbedOption{Title: view.Label, Meta: view.MachineName}
	if !s.embeddableView(ctx, *view) {
		opt.Note = i18n.T(ctx, "Not a published block display of its own: nothing shows in its place.")
		return opt, nil
	}
	var total int
	if view.IsComment() {
		comments, n, err := s.viewService.RunCommentView(ctx, *view, nil, 1)
		if err != nil {
			return EmbedOption{}, fmt.Errorf("running the View %q: %w", name, err)
		}
		for _, c := range comments {
			if len(opt.Lines) < embedPreviewLines {
				opt.Lines = append(opt.Lines, c.Summary())
			}
		}
		total = n
	} else {
		nodes, n, err := s.viewService.RunView(ctx, *view, nil, 1)
		if err != nil {
			return EmbedOption{}, fmt.Errorf("running the View %q: %w", name, err)
		}
		for _, n := range nodes {
			if len(opt.Lines) < embedPreviewLines {
				opt.Lines = append(opt.Lines, n.Title)
			}
		}
		total = n
	}
	opt.More = max(total-len(opt.Lines), 0)
	opt.Note = i18n.T(ctx, "Each visitor sees the items they may.")
	return opt, nil
}
