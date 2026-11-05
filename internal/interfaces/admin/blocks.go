package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// blocksTheme - the placement key of the theme whose block layout the
// page works on (a tab of the blocks admin): the "theme" value when it names an
// available theme, else the active theme.
func blocksTheme(r *http.Request) string {
	if key := r.FormValue("theme"); key != "" && themes.IsAvailable(entity.ThemeDir(key)) {
		return key
	}
	return service.ActiveThemeKey()
}

// blocksURL - the blocks admin page of a theme, with an optional extra
// query ("success=1").
func blocksURL(themeKey, query string) string {
	u := "/admin/config/blocks?theme=" + url.QueryEscape(themeKey)
	if query != "" {
		u += "&" + query
	}
	return u
}

// BlockList renders GET /admin/config/blocks - the block list with a
// title-only quick-add. Each row's "Region" select moves the block between
// regions; order within a region is drag-and-drop.
func (c *Controller) BlockList(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	ctx := r.Context()
	// A theme opened for the first time gets the active theme's layout in
	// the regions it shares.
	if active := service.ActiveThemeKey(); themeKey != active {
		if _, err := c.blockService.InitThemePlacements(ctx, themeKey, active, interfaces.ThemeRegionsFor(c.logger, themeKey)); err != nil {
			c.LogFailure(err, "error initializing theme block placements", "theme", themeKey)
		}
	}
	blocks, err := c.blockService.Blocks(ctx, themeKey)
	if err != nil {
		c.LogFailure(err, "error listing blocks")
		httputil.InternalServerError(w)
		return
	}

	// viewBlockOptions - candidates for the "Add View block" picker: every
	// Block-type display of either base entity, regardless of status (placing
	// doesn't require publishing). Views() returns one representative display
	// per View, while Block displays are usually non-default siblings, so each
	// View's displays are fetched via DisplaysOfView.
	var viewBlockOptions entity.ViewDisplayDefs
	groups, err := c.viewService.Views(ctx)
	if err != nil {
		c.LogFailure(err, "error listing views for view-block picker")
	} else {
		addIfEligible := func(v entity.ViewDisplayDef) {
			if v.DisplayType == entity.ViewDisplayTypeBlock {
				viewBlockOptions = append(viewBlockOptions, v)
			}
		}
		for _, group := range groups {
			addIfEligible(group)
			if group.ViewID == nil {
				continue
			}
			siblings, err := c.viewService.DisplaysOfView(ctx, *group.ViewID)
			if err != nil {
				c.LogFailure(err, "error listing displays of view for view-block picker", "viewID", *group.ViewID)
				continue
			}
			for _, v := range siblings {
				addIfEligible(v)
			}
		}
	}

	// blocksByRegion - groups the (Region, Weight)-sorted blocks into one
	// drag-and-drop zone per region: the theme's regions, "Hidden"
	// (entity.RegionDisabled), and any region the theme no longer declares,
	// so its placements aren't silently dropped from the page.
	theme := interfaces.ThemeRegionsFor(c.logger, themeKey)
	labels := interfaces.TranslatedRegionLabels(ctx, theme)
	zones := theme.RegionIDsWithDisabled()
	blocksByRegion := make(map[entity.Region][]entity.BlockWithPlacement, len(zones))
	for _, b := range blocks {
		region := b.Placement.Region
		if _, known := labels[region]; !known {
			// A region the theme doesn't declare (renamed or dropped in
			// theme.yaml): its placements render nowhere - shown as an
			// extra zone so they can be moved.
			labels[region] = i18n.T(ctx, "@region - not in the theme, not displayed", "@region", string(region))
			zones = append(zones, region)
		}
		blocksByRegion[region] = append(blocksByRegion[region], b)
	}

	// menuOptions - candidates for the "Add menu block" picker: every menu,
	// whether or not it is already placed.
	menuOptions, err := c.menuService.Menus(ctx)
	if err != nil {
		c.LogFailure(err, "error listing menus for menu-block picker")
	}

	data := c.getAdminData(r)
	data["blocksTheme"] = themeKey
	data["title"] = i18n.T(ctx, "Blocks")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["blocks"] = blocks
	data["blocksByRegion"] = blocksByRegion
	data["themeTabs"] = c.blocksThemeTabs(ctx, themeKey)
	data["blocksThemeActive"] = themeKey == service.ActiveThemeKey()
	data["roleLabels"] = c.visibilityRoleLabels(ctx)
	data["allRegions"] = theme.RegionIDs()
	// Where a new block goes (BlockCreate): the theme's default region.
	data["defaultRegion"] = theme.DefaultRegion
	data["allRegionsWithDisabled"] = zones
	data["regionLabels"] = labels
	data["allPluginBlocks"] = service.EnabledPluginBlocks()
	data["pluginBlockLabels"] = translatedLabelMap(ctx, entity.PluginBlockLabels())
	data["viewBlockOptions"] = viewBlockOptions
	data["menuOptions"] = menuOptions
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"

	c.RenderOrFail(w, r, "admin/blocks/index.html", data)
}

// BlockCreate handles POST /admin/config/blocks - a new block starts
// empty at the end of the theme's default region, visible everywhere; the
// admin adjusts body, region and visibility afterwards.
func (c *Controller) BlockCreate(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, blocksURL(themeKey, ""), http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Redirect(w, r, blocksURL(themeKey, "error=empty"), http.StatusSeeOther)
		return
	}

	existing, err := c.blockService.Blocks(ctx, themeKey)
	if err != nil {
		c.LogFailure(err, "error listing blocks before create")
		httputil.InternalServerError(w)
		return
	}
	defaultRegion := interfaces.ThemeRegionsFor(c.logger, themeKey).DefaultRegion
	weight := 0
	for _, b := range existing {
		if b.Placement.Region == defaultRegion && b.Placement.Weight >= weight {
			weight = b.Placement.Weight + 10
		}
	}

	_, err = c.blockService.CreateBlock(ctx, entity.BlockDef{
		Title:     title,
		ShowTitle: true,
		Status:    true,
	}, entity.BlockPlacementDef{
		Theme:           themeKey,
		Region:          defaultRegion,
		Weight:          weight,
		VisibilityScope: entity.BlockVisibilityAll,
	})
	if err != nil {
		c.LogFailure(err, "error creating block", "title", title)
		http.Redirect(w, r, blocksURL(themeKey, "error=create"), http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, blocksURL(themeKey, "success=1"), http.StatusSeeOther)
}

// BlockEdit renders GET /admin/config/blocks/{id}/edit - title, body,
// published flag and visibility.
func (c *Controller) BlockEdit(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	block, err := c.blockService.BlockByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the block", "id", id)
		return
	}
	placement, err := c.blockService.PlacementByBlockID(ctx, id, themeKey)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the block's placement", "id", id)
		return
	}

	contentTypes, ctErr := c.contentTypeService.ContentTypes(ctx)
	if ctErr != nil {
		c.LogFailure(ctErr, "error listing content types for block visibility picker")
	}
	textFormats, tfErr := c.textFormatService.TextFormats(ctx)
	if tfErr != nil {
		c.LogFailure(tfErr, "error listing text formats for block body picker")
	}

	data := c.getAdminData(r)
	data["blocksTheme"] = themeKey
	data["title"] = i18n.T(ctx, "Block \"@title\"", "@title", block.Title)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", i18n.T(ctx, "Block @title", "@title", block.Title))}
	data["block"] = block
	data["placement"] = placement
	data["contentTypes"] = contentTypes
	data["visibilityRoles"] = c.visibilityRoleOptions(r.Context())
	data["textFormats"] = textFormats
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"
	c.AddFormSectionsData(ctx, data, interfaces.FormBlock, strconv.Itoa(id))
	c.AddFormSectionsData(ctx, data, interfaces.FormBlockPlacement, strconv.Itoa(placement.ID))

	c.RenderOrFail(w, r, "admin/blocks/edit.html", data)
}

// BlockUpdate handles POST /admin/config/blocks/{id} - title/body/published
// go to the BlockDef row, visibility (scope + content types) to its
// placement.
func (c *Controller) BlockUpdate(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	editRedirect := fmt.Sprintf("/admin/config/blocks/%d/edit", id)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, editRedirect, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Redirect(w, r, editRedirect+"?error=empty", http.StatusSeeOther)
		return
	}

	placement, err := c.blockService.PlacementByBlockID(ctx, id, themeKey)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the block's placement", "id", id)
		return
	}

	// text_format_id - "" (the default) keeps the fixed SanitizeAdmin policy;
	// a chosen format applies its own AllowedTags via SanitizeWithFormat (the
	// only way a block body can keep a raw <script>, e.g. an ad snippet; see
	// entity.BlockDef.TextFormatID). CanUseTextFormat is checked even though
	// the page is admin-only, as every format consumer does.
	user, _ := c.GetCurrentUser(r)
	var textFormatID *int
	var body string
	if raw := strings.TrimSpace(r.FormValue("text_format_id")); raw != "" {
		if fid, err := strconv.Atoi(raw); err == nil {
			if format, err := c.textFormatService.TextFormatByID(ctx, fid); err == nil && format != nil && c.authService.CanUseTextFormat(ctx, user, *format) {
				textFormatID = &fid
				body = htmlsanitize.SanitizeWithFormat(r.FormValue("body"), format.AllowedTags, format.AllowClassAttribute)
			} else {
				c.LogFailure(err, "loading the text format", "formatID", fid)
			}
		}
	}
	if textFormatID == nil {
		body = htmlsanitize.SanitizeAdmin(r.FormValue("body"))
	}

	_, err = c.blockService.UpdateBlock(ctx, entity.BlockDef{
		ID:           id,
		Title:        title,
		MachineName:  strings.TrimSpace(r.FormValue("machine_name")),
		ShowTitle:    r.FormValue("show_title") == "1",
		Body:         body,
		TextFormatID: textFormatID,
		Status:       r.FormValue("status") == "1",
	})
	if errors.Is(err, service.ErrBlockMachineName) {
		http.Redirect(w, r, editRedirect+"?error=machine_name", http.StatusSeeOther)
		return
	}
	if err != nil {
		c.LogFailure(err, "error updating block", "id", id)
		http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		return
	}

	update := entity.BlockPlacementDef{
		ID:     placement.ID,
		Region: placement.Region,
		Weight: placement.Weight,
		Lazy:   r.FormValue("lazy") == "1",
	}
	setPlacementVisibilityFromForm(r, &update)
	err = c.updatePlacement(ctx, update, r)
	if err == nil {
		err = interfaces.SaveFormSections(ctx, interfaces.FormBlock, strconv.Itoa(id), r)
	}
	if err != nil {
		c.LogFailure(err, "error updating block placement", "id", placement.ID)
		http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		return
	}

	c.cacheService.ForceUpdate()
	http.Redirect(w, r, editRedirect+"?success=1", http.StatusSeeOther)
}

// BlockDeleteConfirm renders GET /admin/config/blocks/{id}/delete.
func (c *Controller) BlockDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	block, err := c.blockService.BlockByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the block", "id", id)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Delete block \"@title\"", "@title", block.Title)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", i18n.T(ctx, "Delete block @title", "@title", block.Title))}
	data["block"] = block

	c.RenderOrFail(w, r, "admin/blocks/delete.html", data)
}

// BlockDelete handles POST /admin/config/blocks/{id}/delete.
func (c *Controller) BlockDelete(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, blocksURL(themeKey, ""), http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, blocksURL(themeKey, "error=invalid"), http.StatusSeeOther)
		return
	}

	if err := c.blockService.DeleteBlock(ctx, id); err != nil {
		c.LogFailure(err, "error deleting block", "id", id)
		http.Redirect(w, r, blocksURL(themeKey, "error=delete"), http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, blocksURL(themeKey, "deleted=1"), http.StatusSeeOther)
}

// BlockRegionDemo renders GET /admin/config/blocks/demo - the real public
// layout with every region outlined and labelled "Label (machine_name)"
// (region_blocks.html's blockDemoMode), so an admin can see where each
// region sits. Rendered through the public template set (no "admin/" path
// prefix, see pageFilesToParse) to reuse the public partials unchanged;
// the admin bar still shows because data["isAdmin"] is true.
func (c *Controller) BlockRegionDemo(w http.ResponseWriter, r *http.Request) {
	data := c.GetCommonData(r)
	data["title"] = i18n.T(r.Context(), "Region demo mode")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", data["title"]), NoIndex: true}
	data["blockDemoMode"] = true
	theme := interfaces.ThemeRegions(c.logger)
	data["regionLabels"] = interfaces.TranslatedRegionLabels(r.Context(), theme)
	data["allRegions"] = theme.RegionIDs()

	c.RenderOrFail(w, r, "block_demo.html", data)
}

// BlocksReorder handles POST /admin/config/blocks/reorder - bulk
// Region/Weight update from the drag-and-drop list: "order=id1,id2,..."
// plus "region_<placementID>" per row.
func (c *Controller) BlocksReorder(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, blocksURL(themeKey, ""), http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	order := strings.Split(r.FormValue("order"), ",")
	if len(order) == 0 || strings.TrimSpace(r.FormValue("order")) == "" {
		http.Redirect(w, r, blocksURL(themeKey, "error=empty-order"), http.StatusSeeOther)
		return
	}

	theme := interfaces.ThemeRegionsFor(c.logger, themeKey)
	if len(theme.Regions) == 0 {
		http.Redirect(w, r, blocksURL(themeKey, "error=theme-regions"), http.StatusSeeOther)
		return
	}
	updates := make([]entity.BlockPlacementOrderUpdate, 0, len(order))
	for i, idStr := range order {
		placementID, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			continue
		}
		// A region the theme doesn't declare goes to "Hidden": it renders nowhere
		// either way and stays visible in the admin.
		region := entity.Region(r.FormValue(fmt.Sprintf("region_%d", placementID)))
		if !theme.Allows(region) {
			region = entity.RegionDisabled
		}
		updates = append(updates, entity.BlockPlacementOrderUpdate{
			PlacementID: placementID,
			Region:      region,
			Weight:      i * 10,
		})
	}

	if err := c.blockService.ReorderPlacements(ctx, updates); err != nil {
		c.LogFailure(err, "error reordering block placements")
		http.Redirect(w, r, blocksURL(themeKey, "error=reorder"), http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, blocksURL(themeKey, "success=1"), http.StatusSeeOther)
}

// blocksThemeTab - a theme tab of the blocks admin.
type blocksThemeTab struct {
	Key, Name        string
	Active, Selected bool
}

// blocksThemeTabs - one tab per available theme; Active marks the theme
// the site renders, Selected the one shown.
func (c *Controller) blocksThemeTabs(ctx context.Context, selected string) []blocksThemeTab {
	themes, err := themes.Available()
	if err != nil {
		c.LogFailure(err, "error listing themes for block tabs")
	}
	active := service.ActiveThemeKey()
	tabs := make([]blocksThemeTab, 0, len(themes))
	for _, t := range themes {
		key := entity.ThemeKey(t.Dir)
		tabs = append(tabs, blocksThemeTab{Key: key, Name: i18n.T(ctx, t.Name), Active: key == active, Selected: key == selected})
	}
	return tabs
}
