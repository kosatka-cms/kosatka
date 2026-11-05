package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// placementOf - the placement {id} if it is of the kind (a
// BlockPlacementDef method: IsPlugin, IsView, IsMenu); false - answered
// with 404.
func (c *Controller) placementOf(w http.ResponseWriter, r *http.Request, kind func(entity.BlockPlacementDef) bool) (*entity.BlockPlacementDef, bool) {
	id, ok := c.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return nil, false
	}
	placement, err := c.blockService.PlacementByID(r.Context(), id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the placement", "id", id)
		return nil, false
	}
	if !kind(*placement) {
		http.NotFound(w, r)
		return nil, false
	}
	return placement, true
}

// updatePlacement saves a placement from its form and the modules'
// sections of it (interfaces.FormBlockPlacement).
func (c *Controller) updatePlacement(ctx context.Context, update entity.BlockPlacementDef, r *http.Request) error {
	if _, err := c.blockService.UpdatePlacement(ctx, update); err != nil {
		return err
	}
	return interfaces.SaveFormSections(ctx, interfaces.FormBlockPlacement, strconv.Itoa(update.ID), r)
}

// placementEditData - what every placement's edit page shows: its theme
// and regions, the title, the placement and the visibility options.
func (c *Controller) placementEditData(r *http.Request, placement *entity.BlockPlacementDef, title string) map[string]any {
	ctx := r.Context()
	data := c.getAdminData(r)
	data["blocksTheme"] = placement.Theme
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["placement"] = placement
	data["contentTypes"] = c.visibilityContentTypeOptions(ctx)
	data["visibilityRoles"] = c.visibilityRoleOptions(ctx)
	theme := interfaces.ThemeRegionsFor(c.logger, placement.Theme)
	data["allRegions"] = theme.RegionIDsWithDisabled()
	data["regionLabels"] = interfaces.TranslatedRegionLabels(ctx, theme)
	c.AddFormSectionsData(ctx, data, interfaces.FormBlockPlacement, strconv.Itoa(placement.ID))
	return data
}

// placementRegionFromForm - the region a placement form chose if the
// placement's theme has it, else the placement's own.
func (c *Controller) placementRegionFromForm(r *http.Request, placement *entity.BlockPlacementDef) entity.Region {
	region := entity.Region(r.FormValue("region"))
	if !interfaces.ThemeRegionsFor(c.logger, placement.Theme).Allows(region) {
		return placement.Region
	}
	return region
}

// PluginBlockCreate handles POST /admin/config/blocks/plugin - places a
// plugin block (see entity.PluginBlockID) at the end of the chosen region.
// Unlike BlockCreate, the region is picked up front.
func (c *Controller) PluginBlockCreate(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, blocksURL(themeKey, ""), http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	pluginID := entity.PluginBlockID(r.FormValue("plugin_id"))
	if _, ok := entity.PluginBlockLabels()[pluginID]; !ok {
		http.Redirect(w, r, blocksURL(themeKey, "error=plugin"), http.StatusSeeOther)
		return
	}
	region := entity.Region(r.FormValue("region"))
	if !interfaces.ThemeRegionsFor(c.logger, themeKey).Allows(region) {
		http.Redirect(w, r, blocksURL(themeKey, "error=plugin"), http.StatusSeeOther)
		return
	}

	existing, err := c.blockService.Blocks(ctx, themeKey)
	if err != nil {
		c.LogFailure(err, "error listing blocks before plugin create")
		httputil.InternalServerError(w)
		return
	}
	weight := 0
	for _, b := range existing {
		if b.Placement.Region == region && b.Placement.Weight >= weight {
			weight = b.Placement.Weight + 10
		}
	}

	created, err := c.blockService.CreatePluginPlacement(ctx, entity.BlockPlacementDef{
		PlacementType:   entity.BlockPlacementPlugin,
		PluginID:        &pluginID,
		Theme:           themeKey,
		Region:          region,
		Weight:          weight,
		VisibilityScope: entity.BlockVisibilityAll,
	})
	if err != nil {
		c.LogFailure(err, "error creating plugin block placement", "plugin_id", pluginID, "region", region)
		http.Redirect(w, r, blocksURL(themeKey, "error=plugin"), http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	// A block that shows a chosen thing (a form) shows nothing until it is
	// chosen: its placement page next.
	if _, hasTargets, _ := service.PluginBlockTargets(ctx, pluginID); hasTargets && created != nil {
		http.Redirect(w, r, "/admin/config/blocks/plugin/"+strconv.Itoa(created.ID)+"/edit?choose=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, blocksURL(themeKey, "success=1"), http.StatusSeeOther)
}

// pluginSettingField - a plugin block's setting on its placement page
// with the placement's value.
type pluginSettingField struct {
	entity.PluginBlockSetting
	Value string
}

// PluginPlacementEdit handles GET /admin/config/blocks/plugin/{id}/edit -
// a plugin placement's title and visibility. The plugin itself can't be
// changed after creation.
func (c *Controller) PluginPlacementEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	placement, ok := c.placementOf(w, r, entity.BlockPlacementDef.IsPlugin)
	if !ok {
		return
	}
	data := c.placementEditData(r, placement, i18n.T(ctx, "System block"))
	data["pluginLabel"] = i18n.T(ctx, entity.PluginBlockLabels()[placement.PluginIDValue()])
	// A block showing a chosen thing: the choice.
	targets, hasTargets, err := service.PluginBlockTargets(ctx, placement.PluginIDValue())
	if err != nil {
		c.LogFailure(err, "error listing plugin block targets", "plugin", placement.PluginIDValue())
	}
	data["hasTargets"] = hasTargets
	data["targets"] = targets
	data["target"] = placement.PluginSetting(entity.PluginBlockTargetKey)
	settings, err := service.PluginBlockSettings(ctx, placement.PluginIDValue())
	if err != nil {
		c.LogFailure(err, "error listing plugin block settings", "plugin", placement.PluginIDValue())
	}
	fields := make([]pluginSettingField, len(settings))
	for i, s := range settings {
		fields[i] = pluginSettingField{PluginBlockSetting: s, Value: placement.PluginSettingOr(s)}
	}
	data["pluginSettings"] = fields
	data["error"] = r.URL.Query().Get("error")
	data["choose"] = r.URL.Query().Get("choose") == "1"

	c.RenderOrFail(w, r, "admin/blocks/plugin_placement_edit.html", data)
}

// PluginPlacementUpdate handles POST
// /admin/config/blocks/plugin/{id}/edit.
func (c *Controller) PluginPlacementUpdate(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, blocksURL(themeKey, ""), http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	placement, ok := c.placementOf(w, r, entity.BlockPlacementDef.IsPlugin)
	if !ok {
		return
	}
	themeKey = placement.Theme

	// A block showing a chosen thing: the choice must be one it offers.
	var pluginConfig map[string]string
	if targets, hasTargets, terr := service.PluginBlockTargets(ctx, placement.PluginIDValue()); hasTargets {
		target := r.FormValue("target")
		known := false
		for _, t := range targets {
			known = known || t.Value == target
		}
		if terr != nil || !known {
			http.Redirect(w, r, "/admin/config/blocks/plugin/"+strconv.Itoa(placement.ID)+"/edit?error=target", http.StatusSeeOther)
			return
		}
		pluginConfig = map[string]string{entity.PluginBlockTargetKey: target}
	}
	// The block's settings: each must be one it accepts.
	settings, serr := service.PluginBlockSettings(ctx, placement.PluginIDValue())
	if serr != nil {
		c.LogFailure(serr, "error listing plugin block settings", "plugin", placement.PluginIDValue())
		http.Redirect(w, r, "/admin/config/blocks/plugin/"+strconv.Itoa(placement.ID)+"/edit?error=settings", http.StatusSeeOther)
		return
	}
	for _, s := range settings {
		value := strings.TrimSpace(r.FormValue("setting_" + s.Key))
		if !s.Check(value) {
			http.Redirect(w, r, "/admin/config/blocks/plugin/"+strconv.Itoa(placement.ID)+"/edit?error=settings", http.StatusSeeOther)
			return
		}
		if pluginConfig == nil {
			pluginConfig = map[string]string{}
		}
		pluginConfig[s.Key] = value
	}

	update := entity.BlockPlacementDef{
		ID:           placement.ID,
		Region:       c.placementRegionFromForm(r, placement),
		Weight:       placement.Weight,
		Title:        strings.TrimSpace(r.FormValue("title")),
		ShowTitle:    r.FormValue("show_title") == "1",
		PluginConfig: pluginConfig,
	}
	setPlacementVisibilityFromForm(r, &update)
	if err := c.updatePlacement(ctx, update, r); err != nil {
		c.LogFailure(err, "error updating plugin placement", "id", placement.ID)
		http.Redirect(w, r, blocksURL(themeKey, "error=plugin"), http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, blocksURL(themeKey, "success=1"), http.StatusSeeOther)
}

// ViewBlockCreate handles POST /admin/config/blocks/view - places a View's
// Block display (entity.ViewDisplayTypeBlock) at the end of the chosen
// region. Visibility can be set here too, not only later on
// ViewPlacementEdit.
func (c *Controller) ViewBlockCreate(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, blocksURL(themeKey, ""), http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	viewDisplayID, err := strconv.Atoi(r.FormValue("view_display_id"))
	if err != nil || viewDisplayID <= 0 {
		http.Redirect(w, r, blocksURL(themeKey, "error=view"), http.StatusSeeOther)
		return
	}
	view, err := c.viewService.ViewByID(ctx, viewDisplayID)
	if err != nil || view.DisplayType != entity.ViewDisplayTypeBlock {
		c.LogFailure(err, "loading the View display", "viewID", viewDisplayID)
		http.Redirect(w, r, blocksURL(themeKey, "error=view"), http.StatusSeeOther)
		return
	}
	region := entity.Region(r.FormValue("region"))
	if !interfaces.ThemeRegionsFor(c.logger, themeKey).Allows(region) {
		http.Redirect(w, r, blocksURL(themeKey, "error=view"), http.StatusSeeOther)
		return
	}
	scope := entity.BlockVisibilityScope(r.FormValue("visibility_scope"))
	if scope != entity.BlockVisibilityHomepage && scope != entity.BlockVisibilityContentType {
		scope = entity.BlockVisibilityAll
	}
	var visibilityContentType *string
	if scope == entity.BlockVisibilityContentType {
		visibilityContentType = parseVisibilityContentTypes(r)
	}

	existing, err := c.blockService.Blocks(ctx, themeKey)
	if err != nil {
		c.LogFailure(err, "error listing blocks before view-block create")
		httputil.InternalServerError(w)
		return
	}
	weight := 0
	for _, b := range existing {
		if b.Placement.Region == region && b.Placement.Weight >= weight {
			weight = b.Placement.Weight + 10
		}
	}

	_, err = c.blockService.CreateViewPlacement(ctx, entity.BlockPlacementDef{
		PlacementType: entity.BlockPlacementView,
		ViewDisplayID: &viewDisplayID,
		Theme:         themeKey,
		Region:        region,
		Weight:        weight,
		// ShowTitle=true shows the View label as the heading (region_blocks.html);
		// an empty Title falls back to the label.
		ShowTitle:             true,
		VisibilityScope:       scope,
		VisibilityRoles:       parseVisibilityRoles(r),
		VisibilityContentType: visibilityContentType,
	})
	if err != nil {
		c.LogFailure(err, "error creating view block placement", "view_display_id", viewDisplayID, "region", region)
		http.Redirect(w, r, blocksURL(themeKey, "error=view"), http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, blocksURL(themeKey, "success=1"), http.StatusSeeOther)
}

// ViewPlacementEdit handles GET /admin/config/blocks/view/{id}/edit - a
// View placement's title and visibility. The View itself can't be changed
// after creation.
func (c *Controller) ViewPlacementEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	placement, ok := c.placementOf(w, r, entity.BlockPlacementDef.IsView)
	if !ok {
		return
	}
	var viewDisplay *entity.ViewDisplayDef
	if placement.ViewDisplayID != nil {
		var err error
		viewDisplay, err = c.viewService.ViewByID(ctx, *placement.ViewDisplayID)
		c.LogFailure(err, "loading the View display", "viewID", *placement.ViewDisplayID)
	}

	data := c.placementEditData(r, placement, i18n.T(ctx, "View block"))
	data["viewDisplay"] = viewDisplay

	c.RenderOrFail(w, r, "admin/blocks/view_placement_edit.html", data)
}

// ViewPlacementUpdate handles POST /admin/config/blocks/view/{id}/edit.
func (c *Controller) ViewPlacementUpdate(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, blocksURL(themeKey, ""), http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	placement, ok := c.placementOf(w, r, entity.BlockPlacementDef.IsView)
	if !ok {
		return
	}
	themeKey = placement.Theme

	update := entity.BlockPlacementDef{
		ID:        placement.ID,
		Region:    c.placementRegionFromForm(r, placement),
		Weight:    placement.Weight,
		Title:     strings.TrimSpace(r.FormValue("title")),
		ShowTitle: r.FormValue("show_title") == "1",
		Lazy:      r.FormValue("lazy") == "1",
	}
	setPlacementVisibilityFromForm(r, &update)
	if err := c.updatePlacement(ctx, update, r); err != nil {
		c.LogFailure(err, "error updating view placement", "id", placement.ID)
		http.Redirect(w, r, blocksURL(themeKey, "error=view"), http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, blocksURL(themeKey, "success=1"), http.StatusSeeOther)
}

// MenuBlockCreate handles POST /admin/config/blocks/menu - places an
// existing menu as a block at the end of the chosen region (validated
// against the theme's regions). A theme can give a region's menu blocks
// its own markup ("menu_block_<region>", region_blocks.html).
func (c *Controller) MenuBlockCreate(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, blocksURL(themeKey, ""), http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	menuID, err := strconv.Atoi(r.FormValue("menu_id"))
	if err != nil || menuID <= 0 {
		http.Redirect(w, r, blocksURL(themeKey, "error=menu"), http.StatusSeeOther)
		return
	}
	if _, err := c.menuService.MenuByID(ctx, menuID); err != nil {
		http.Redirect(w, r, blocksURL(themeKey, "error=menu"), http.StatusSeeOther)
		return
	}
	region := entity.Region(r.FormValue("region"))
	if !interfaces.ThemeRegionsFor(c.logger, themeKey).Allows(region) {
		http.Redirect(w, r, blocksURL(themeKey, "error=menu"), http.StatusSeeOther)
		return
	}
	startLevel, _ := strconv.Atoi(r.FormValue("start_level"))
	levelsToShow, _ := strconv.Atoi(r.FormValue("levels_to_show"))
	startLevel, levelsToShow = entity.ClampMenuLevelSettings(startLevel, levelsToShow)
	scope := entity.BlockVisibilityScope(r.FormValue("visibility_scope"))
	if scope != entity.BlockVisibilityHomepage && scope != entity.BlockVisibilityContentType {
		scope = entity.BlockVisibilityAll
	}
	var visibilityContentType *string
	if scope == entity.BlockVisibilityContentType {
		visibilityContentType = parseVisibilityContentTypes(r)
	}

	existing, err := c.blockService.Blocks(ctx, themeKey)
	if err != nil {
		c.LogFailure(err, "error listing blocks before menu-block create")
		httputil.InternalServerError(w)
		return
	}
	weight := 0
	for _, b := range existing {
		if b.Placement.Region == region && b.Placement.Weight >= weight {
			weight = b.Placement.Weight + 10
		}
	}

	_, err = c.blockService.CreateMenuPlacement(ctx, entity.BlockPlacementDef{
		PlacementType:         entity.BlockPlacementMenu,
		MenuID:                &menuID,
		MenuStartLevel:        startLevel,
		MenuLevelsToShow:      levelsToShow,
		Theme:                 themeKey,
		Region:                region,
		Weight:                weight,
		VisibilityScope:       scope,
		VisibilityRoles:       parseVisibilityRoles(r),
		VisibilityContentType: visibilityContentType,
	})
	if err != nil {
		c.LogFailure(err, "error creating menu block placement", "menu_id", menuID, "region", region)
		http.Redirect(w, r, blocksURL(themeKey, "error=menu"), http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, blocksURL(themeKey, "success=1"), http.StatusSeeOther)
}

// MenuPlacementEdit renders GET /admin/config/blocks/menu/{id}/edit -
// region, visibility, start level and depth of a menu placement. The menu
// itself can't be changed; delete and re-add instead. "Placement" (not
// "Block") because plugin/view/menu placements have no owning BlockDef.
func (c *Controller) MenuPlacementEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	placement, ok := c.placementOf(w, r, entity.BlockPlacementDef.IsMenu)
	if !ok {
		return
	}
	var menu *entity.MenuDef
	if placement.MenuID != nil {
		menu, _ = c.menuService.MenuByID(ctx, *placement.MenuID)
	}

	data := c.placementEditData(r, placement, i18n.T(ctx, "Menu block"))
	data["menu"] = menu

	c.RenderOrFail(w, r, "admin/blocks/menu_placement_edit.html", data)
}

// MenuPlacementUpdate handles POST /admin/config/blocks/menu/{id}.
func (c *Controller) MenuPlacementUpdate(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, blocksURL(themeKey, ""), http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	placement, ok := c.placementOf(w, r, entity.BlockPlacementDef.IsMenu)
	if !ok {
		return
	}
	themeKey = placement.Theme

	startLevel, _ := strconv.Atoi(r.FormValue("start_level"))
	levelsToShow, _ := strconv.Atoi(r.FormValue("levels_to_show"))
	startLevel, levelsToShow = entity.ClampMenuLevelSettings(startLevel, levelsToShow)
	update := entity.BlockPlacementDef{
		ID:               placement.ID,
		Region:           c.placementRegionFromForm(r, placement),
		Weight:           placement.Weight,
		MenuStartLevel:   startLevel,
		MenuLevelsToShow: levelsToShow,
		Title:            strings.TrimSpace(r.FormValue("title")),
		ShowTitle:        r.FormValue("show_title") == "1",
	}
	setPlacementVisibilityFromForm(r, &update)
	if err := c.updatePlacement(ctx, update, r); err != nil {
		c.LogFailure(err, "error updating menu placement", "id", placement.ID)
		http.Redirect(w, r, blocksURL(themeKey, "error=menu"), http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, blocksURL(themeKey, "success=1"), http.StatusSeeOther)
}

// PluginPlacementDelete handles POST
// /admin/config/blocks/placement/{id}/delete - removes a plugin
// block's placement directly (there is no owning BlockDef to cascade
// through, unlike BlockDelete).
func (c *Controller) PluginPlacementDelete(w http.ResponseWriter, r *http.Request) {
	themeKey := blocksTheme(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, blocksURL(themeKey, ""), http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, blocksURL(themeKey, "error=delete-placement"), http.StatusSeeOther)
		return
	}

	if err := c.blockService.DeletePlacement(ctx, id); err != nil {
		c.LogFailure(err, "error deleting plugin block placement", "id", id)
		http.Redirect(w, r, blocksURL(themeKey, "error=delete-placement"), http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, blocksURL(themeKey, "success=1"), http.StatusSeeOther)
}
