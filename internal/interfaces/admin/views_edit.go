package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// viewGroupFieldOption - one "Group by field" choice: the field's machine
// name (a field storage's or a builtin's) and label.
type viewGroupFieldOption struct {
	Key   string
	Label string
}

// viewGroupFieldOptions - the display's own fields as grouping choices
// (ViewDisplayDef.GroupByField), in the display's field order.
func viewGroupFieldOptions(ctx context.Context, fields []entity.ViewFieldDef, machineNames map[int]string) []viewGroupFieldOption {
	var out []viewGroupFieldOption
	for _, f := range fields {
		// A module's field is the module's markup, nothing to group by.
		if views.IsModuleKey(string(f.BuiltinField)) {
			continue
		}
		key := string(f.BuiltinField)
		label := views.FieldBuiltinLabel(ctx, f.BuiltinField)
		if f.FieldStorageID != nil {
			key = machineNames[*f.FieldStorageID]
		}
		if f.Label != "" {
			label = f.Label
		}
		if key == "" {
			continue
		}
		if label == "" {
			label = key
		}
		out = append(out, viewGroupFieldOption{Key: key, Label: label})
	}
	return out
}

// renderSettingsSummaryFragment renders the "Page settings" card summary,
// refreshed after ViewUpdate's AJAX response. Uses EffectiveSettings so an
// inherited section shows Default's values (see
// entity.ViewDisplayDef.CanEditSettings).
func (c *Controller) renderSettingsSummaryFragment(ctx context.Context, w io.Writer, view *entity.ViewDisplayDef) error {
	effective, err := c.viewService.EffectiveSettings(ctx, *view)
	if err != nil {
		c.LogFailure(err, "error resolving effective settings for summary", "viewID", view.ID)
		effective = *view
	}
	data := map[string]any{"view": &effective}
	// fieldLabels - needed only for the glossary target's label when it is a
	// generic field (ViewEdit's full render has the whole catalog's labels).
	if effective.GlossaryFieldStorageID != nil {
		if fs, err := c.contentTypeService.FieldStorageByID(ctx, *effective.GlossaryFieldStorageID); err == nil {
			data["fieldLabels"] = c.resolveFieldStorageLabels(ctx, entity.FieldStorageDefs{*fs})
		} else {
			c.LogFailure(err, "error loading glossary field storage for settings summary", "viewID", view.ID)
		}
	}
	return c.templateService.RenderNamed(w, "view_settings_summary", data, viewFragmentFiles...)
}

// ViewEdit renders GET /admin/config/views/{id} - the single-page Views
// editor (admin/views/page.html) with Settings/Filters/Fields cards; Views
// is the one admin section not split into tabs (see admin/views/tabs.html).
func (c *Controller) ViewEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	rawView, err := c.viewService.ViewByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the View display", "viewID", id)
		return
	}
	// effective - Settings resolved against Default when inherited; identity
	// fields, override flags and IDs pass through unchanged, so it is safe to
	// use everywhere the template needs "this display".
	effective, err := c.viewService.EffectiveSettings(ctx, *rawView)
	if err != nil {
		c.LogFailure(err, "error resolving effective settings", "viewID", id)
		httputil.InternalServerError(w)
		return
	}
	view := &effective

	filters, err := c.viewService.EffectiveFiltersByView(ctx, *view)
	if err != nil {
		c.LogFailure(err, "error listing view filters", "viewID", id)
		httputil.InternalServerError(w)
		return
	}
	sorts, err := c.viewService.EffectiveSortsByView(ctx, *view)
	if err != nil {
		c.LogFailure(err, "error listing view sorts", "viewID", id)
		httputil.InternalServerError(w)
		return
	}
	fields, err := c.viewService.EffectiveFieldsByView(ctx, *view)
	if err != nil {
		c.LogFailure(err, "error listing view fields", "viewID", id)
		httputil.InternalServerError(w)
		return
	}
	cat, err := c.viewCatalogFor(ctx, view)
	if err != nil {
		c.LogFailure(err, "error loading the view page's catalogs", "viewID", id)
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = view.Label
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", view.Label)}
	data["view"] = view
	c.addViewTabs(ctx, data, view)
	data["sorts"] = sorts
	data["sortFieldOptions"] = cat.sortOptions
	data["sortBuiltinFields"] = cat.sortBuiltins
	data["sortBuiltinLabels"] = translatedLabelMap(ctx, sortBuiltinLabels)
	data["filters"] = filters
	data["filterFieldOptions"] = cat.filterOptions
	data["blockFilterOptions"] = cat.blockFilters
	data["fieldLabels"] = cat.fieldLabels
	data["operators"] = viewFilterOperators
	data["builtinFields"] = cat.filterBuiltins
	data["contentTypes"] = cat.contentTypes
	data["vocabularies"] = cat.vocabularies
	data["filterTargetOperators"] = cat.filterTargetOperators
	data["taxonomyFieldTerms"] = cat.taxonomyFieldTerms
	data["taxonomyTermLabels"] = cat.taxonomyTermLabels
	data["filterValueDisplays"] = filterValueDisplays(ctx, filters, cat.storageTypes, c.resolveTermNames(ctx, cat.allStorages))
	data["fields"] = fields
	data["fieldOptions"] = cat.fieldOptions
	data["fieldStorageLabels"] = cat.fieldLabels
	data["fieldStorageMachineNames"] = fieldStorageMachineNames(cat.allStorages)
	data["fieldStorageTypes"] = cat.storageTypes
	data["fieldBuiltinFields"] = cat.fieldBuiltins
	data["fieldBuiltinLabels"] = views.FieldBuiltinLabels(ctx)
	data["imageStyles"] = cat.imageStyles
	// linkFormatters - the modules' formatters a link field may show with.
	data["linkFormatters"] = service.FieldFormatters(entity.FieldTypeLink)
	data["tagFormats"] = viewTagFormats
	data["glossaryFieldOptions"] = cat.glossaryOptions
	data["groupFieldOptions"] = viewGroupFieldOptions(ctx, fields, fieldStorageMachineNames(cat.allStorages))
	if c.dateFormats != nil {
		if formats, dfErr := c.dateFormats.DateFormats(ctx); dfErr != nil {
			c.LogFailure(dfErr, "error listing date formats for view page")
		} else {
			data["dateFormats"] = formats
		}
	}
	addViewModuleCriteria(ctx, data, view)
	for _, o := range data["moduleFilters"].([]viewModuleOption) {
		cat.filterTargetOperators[o.Target] = o.Operators
	}
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"
	c.AddFormSectionsData(ctx, data, interfaces.FormView, strconv.Itoa(view.ID))

	c.RenderOrFail(w, r, "admin/views/page.html", data)
}

// addViewTabs adds the display tab strip's data - shown only when the
// display belongs to a View with more than one display (HasGroup()): the
// Default, the siblings and an "unpublished changes" badge per tab - and
// the action bar's state (hasUnpublishedChanges; a lone display isn't
// among the siblings). Failures are logged: the page shows without them.
func (c *Controller) addViewTabs(ctx context.Context, data map[string]any, view *entity.ViewDisplayDef) {
	var defaultDisplay *entity.ViewDisplayDef
	var siblingDisplays entity.ViewDisplayDefs
	if view.HasGroup() {
		var err error
		if defaultDisplay, err = c.viewService.DefaultDisplayOfView(ctx, view.ViewIDValue()); err != nil {
			c.LogFailure(err, "error loading default display for tab strip", "viewID", view.ViewIDValue())
		}
		if siblingDisplays, err = c.viewService.DisplaysOfView(ctx, view.ViewIDValue()); err != nil {
			c.LogFailure(err, "error loading sibling displays for tab strip", "viewID", view.ViewIDValue())
		}
	}
	siblingUnpublished := make(map[int]bool, len(siblingDisplays))
	for _, d := range siblingDisplays {
		has, err := c.viewService.HasUnpublishedChanges(ctx, d.ID)
		if err != nil {
			c.LogFailure(err, "error checking unpublished changes for tab badge", "displayID", d.ID)
			continue
		}
		siblingUnpublished[d.ID] = has
	}
	hasUnpublishedChanges, err := c.viewService.HasUnpublishedChanges(ctx, view.ID)
	if err != nil {
		c.LogFailure(err, "error checking unpublished changes for publish status", "id", view.ID)
	}
	data["defaultDisplay"] = defaultDisplay
	data["siblingDisplays"] = siblingDisplays
	data["siblingUnpublished"] = siblingUnpublished
	data["hasUnpublishedChanges"] = hasUnpublishedChanges
}

// viewCatalog - what the View page's pickers offer. It depends on
// BaseEntity: a comment display has no generic fields, taxonomy or image
// styles, so it gets a smaller catalog (page.html doesn't branch on
// BaseEntity; it renders whatever is here).
type viewCatalog struct {
	sortBuiltins   []entity.ViewSortBuiltin
	fieldBuiltins  []entity.ViewFieldBuiltinField
	filterBuiltins []entity.ViewFilterBuiltinField
	// contentTypes - for the content_type filter's value picker (a
	// machine name chosen from the catalog instead of free text; a
	// comment view's comment_node_type filter too); vocabularies.
	contentTypes entity.ContentTypeDefs
	vocabularies entity.TaxonomyVocabularies
	// allStorages - the whole reusable field catalog: a View isn't
	// scoped to one content type by a column (see entity.ViewDisplayDef),
	// so pickers aren't narrowed to one type's fields; narrowed into the
	// sortable, filterable, displayable and glossary sets, by label.
	allStorages, sortOptions, filterOptions, fieldOptions, glossaryOptions entity.FieldStorageDefs
	storageTypes                                                           map[int]entity.ContentTypeFieldType
	fieldLabels                                                            map[int]string
	// filterTargetOperators - "field:<id>"/"builtin:<name>"/"block:…" ->
	// the data-operators string for admin_views.js.
	filterTargetOperators map[string]string
	// taxonomyFieldTerms - the terms of every filterable taxonomy field's
	// vocabulary by FieldStorageID, for the filter modal's term picker
	// (SortByHierarchy order, labels with BuildHierarchy's "-- " depth).
	taxonomyFieldTerms map[int]entity.TaxonomyTerms
	taxonomyTermLabels map[int]string
	imageStyles        entity.ImageStyleDefs
	// blockFilters - the fields of the blocks fields' sets: filters in
	// the blocks ("Cast › Actor").
	blockFilters []blockFilterOption
}

// viewCatalogFor - view's viewCatalog; an error - one it can't do
// without (content types, vocabularies, the field catalog); the others
// are logged and left empty.
func (c *Controller) viewCatalogFor(ctx context.Context, view *entity.ViewDisplayDef) (viewCatalog, error) {
	cat := viewCatalog{
		sortBuiltins: views.SortBuiltins(*view), fieldBuiltins: views.FieldBuiltins(*view), filterBuiltins: views.FilterBuiltins(*view),
		storageTypes: map[int]entity.ContentTypeFieldType{}, fieldLabels: map[int]string{}, filterTargetOperators: map[string]string{},
		taxonomyFieldTerms: map[int]entity.TaxonomyTerms{}, taxonomyTermLabels: map[int]string{},
	}
	var err error
	if cat.contentTypes, err = c.contentTypeService.ContentTypes(ctx); err != nil {
		return cat, fmt.Errorf("loading content types: %w", err)
	}
	if cat.vocabularies, err = c.taxonomyService.Vocabularies(ctx); err != nil {
		return cat, fmt.Errorf("loading vocabularies: %w", err)
	}
	for _, bf := range cat.filterBuiltins {
		cat.filterTargetOperators["builtin:"+string(bf)] = operatorsAttr(views.FilterBuiltinOperators(bf))
	}
	if view.IsComment() {
		return cat, nil
	}
	if cat.allStorages, err = c.contentTypeService.FieldStorages(ctx); err != nil {
		return cat, fmt.Errorf("loading field storages: %w", err)
	}
	for _, fs := range cat.allStorages {
		c.addCatalogStorage(ctx, &cat, fs)
	}
	if cat.imageStyles, err = c.imageStyleService.ImageStyles(ctx); err != nil {
		c.LogFailure(err, "error loading image styles for view page", "viewID", view.ID)
	}
	cat.fieldLabels = c.resolveFieldStorageLabels(ctx, cat.allStorages)

	// A field only the blocks' sets have is no node's own.
	var setOnly map[int]bool
	if cat.blockFilters, setOnly, err = c.blockFilterOptions(ctx, cat.allStorages, cat.fieldLabels); err != nil {
		c.LogFailure(err, "error loading the blocks fields' sets for view page", "viewID", view.ID)
	}
	for _, o := range cat.blockFilters {
		cat.filterTargetOperators[o.Target] = o.Operators
	}
	notOwn := func(fs entity.FieldStorageDef) bool { return setOnly[fs.ID] }
	// The pickers list fields alphabetically by label; allStorages itself
	// stays ordered by machine name.
	byLabel := func(a, b entity.FieldStorageDef) int {
		return strings.Compare(strings.ToLower(cat.fieldLabels[a.ID]), strings.ToLower(cat.fieldLabels[b.ID]))
	}
	for _, opts := range []*entity.FieldStorageDefs{&cat.sortOptions, &cat.filterOptions, &cat.fieldOptions, &cat.glossaryOptions} {
		*opts = slices.DeleteFunc(*opts, notOwn)
		slices.SortFunc(*opts, byLabel)
	}
	return cat, nil
}

// addCatalogStorage adds field storage fs to the pickers it fits:
// sortable, filterable (its operators; a taxonomy field's terms),
// displayable, glossary.
func (c *Controller) addCatalogStorage(ctx context.Context, cat *viewCatalog, fs entity.FieldStorageDef) {
	cat.storageTypes[fs.ID] = fs.FieldType
	if views.SortableFieldType(fs.FieldType) {
		cat.sortOptions = append(cat.sortOptions, fs)
	}
	if views.FilterableFieldType(fs.FieldType) {
		cat.filterOptions = append(cat.filterOptions, fs)
		cat.filterTargetOperators["field:"+strconv.Itoa(fs.ID)] = operatorsAttr(views.FilterOperators(fs.FieldType))
		if fs.FieldType == entity.FieldTypeTaxonomy {
			terms, err := c.taxonomyService.TermsByVocabularyID(ctx, fs.VocabularyID())
			if err != nil {
				c.LogFailure(err, "error loading terms for filter taxonomy field", "fieldStorageID", fs.ID)
			} else {
				cat.taxonomyFieldTerms[fs.ID] = terms.SortByHierarchy()
				maps.Copy(cat.taxonomyTermLabels, terms.BuildHierarchy())
			}
		}
	}
	if views.DisplayableFieldType(fs.FieldType) {
		cat.fieldOptions = append(cat.fieldOptions, fs)
	}
	if views.GlossaryFieldType(fs.FieldType) {
		cat.glossaryOptions = append(cat.glossaryOptions, fs)
	}
}

// ViewUpdate handles POST /admin/config/views/{id} (a new MachineName
// renames the display: views.DefaultService.UpdateView). Answers with
// JSON+a re-rendered settings-card summary
// when the request came from the page's AJAX settings modal
// (wantsJSON) - the classic full-page redirect stays as the non-JS
// fallback.
func (c *Controller) ViewUpdate(w http.ResponseWriter, r *http.Request) {
	id, redirectBase, ok := c.viewPost(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	existing, err := c.viewService.ViewByID(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading the View display", "viewID", id)
		c.jsonOrRedirectError(w, r, redirectBase, "update")
		return
	}
	v, code := c.viewFromForm(ctx, r, *existing)
	if code != "" {
		c.jsonOrRedirectError(w, r, redirectBase, code)
		return
	}
	updated, err := c.viewService.UpdateView(ctx, v)
	if err == nil {
		err = interfaces.SaveFormSections(ctx, interfaces.FormView, strconv.Itoa(id), r)
	}
	if err != nil {
		if code := viewUpdateErrorCode(err); code != "" {
			c.jsonOrRedirectError(w, r, redirectBase, code)
			return
		}
		c.LogFailure(err, "error updating view", "id", id)
		c.jsonOrRedirectError(w, r, redirectBase, "update")
		return
	}
	c.jsonOrRedirectSuccess(w, r, redirectBase, func(w io.Writer) error {
		return c.renderSettingsSummaryFragment(ctx, w, updated)
	})
}

// viewFromForm - existing as the settings form posts it: its identity
// (label, meta, URL, display type and placement, theme template,
// status) always; its settings section (viewSettingsFromForm) only when
// it is this display's own (CanEditSettings) - while inherited the
// stored values are kept and stray form values ignored. A code - the
// form's error.
func (c *Controller) viewFromForm(ctx context.Context, r *http.Request, existing entity.ViewDisplayDef) (entity.ViewDisplayDef, string) {
	v := existing
	// Empty (an older form) - the display keeps its name.
	if name := strings.TrimSpace(r.FormValue("machine_name")); name != "" {
		v.MachineName = name
	}
	v.Label = strings.TrimSpace(r.FormValue("label"))
	if v.Label == "" {
		return v, "empty"
	}
	v.MetaTitle = strings.TrimSpace(r.FormValue("meta_title"))
	v.MetaDescription = strings.TrimSpace(r.FormValue("meta_description"))
	v.DisplayType = parseViewDisplayType(r.FormValue("display_type"))
	// A block's url_path is always cleared by UpdateView, whatever the hidden
	// input sends. For a page, an empty value (typical after switching from
	// block) is regenerated from the label rather than rejected.
	v.URLPath = strings.TrimSpace(r.FormValue("url_path"))
	if v.DisplayType.HasURL() && v.URLPath == "" {
		v.URLPath = resolveSlug("", v.Label)
	}
	viewPlacementFromForm(r, &v)
	v.ThemeTemplate = strings.TrimSpace(r.FormValue("theme_template"))
	v.Status = 0
	if r.FormValue("status") == "1" {
		v.Status = 1
	}
	if existing.CanEditSettings() {
		if code := c.viewSettingsFromForm(ctx, r, &v); code != "" {
			return v, code
		}
	}
	return v, ""
}

// viewPlacementFromForm sets where a display of v's type is placed:
// attachment - the content types, title and weight; term - the
// vocabularies and weight; user tab - its path, access and weight. Like
// url_path they are identity fields, not gated by CanEditSettings; the
// other types' are kept as they are.
func viewPlacementFromForm(r *http.Request, v *entity.ViewDisplayDef) {
	weight := func(name string) int {
		n, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(name)))
		return n
	}
	switch v.DisplayType {
	case entity.ViewDisplayTypeAttachment:
		v.AttachContentTypes = joinFormList(r, "attach_content_types")
		v.AttachWeight = weight("attach_weight")
		v.AttachTitle = strings.TrimSpace(r.FormValue("attach_title"))
		v.AttachShowTitle = r.FormValue("attach_show_title") == "1"
	case entity.ViewDisplayTypeTerm:
		v.TermVocabularies = joinFormList(r, "term_vocabularies")
		v.AttachWeight = weight("term_weight")
	case entity.ViewDisplayTypeUserTab:
		v.ProfileTabPath = strings.TrimSpace(r.FormValue("profile_tab_path"))
		v.ProfileTabAccess = entity.ViewProfileTabAccess(r.FormValue("profile_tab_access"))
		v.AttachWeight = weight("profile_tab_weight")
	}
}

// viewSettingsFromForm sets v's settings section (display style, grid,
// carousel, pagination, glossary, page size, texts, CSS, row template,
// grouping) from the form; an empty or invalid number keeps the stored
// value, an invalid choice falls back to the default. A code - the
// form's error (custom CSS closing its <style>).
func (c *Controller) viewSettingsFromForm(ctx context.Context, r *http.Request, v *entity.ViewDisplayDef) string {
	positive := func(name string, least int, dst *int) {
		if n, err := strconv.Atoi(r.FormValue(name)); err == nil && n >= least {
			*dst = n
		}
	}
	// An empty or invalid storage ID means the default, not a form error.
	storageID := func(name string) *int {
		if n, err := strconv.Atoi(r.FormValue(name)); err == nil && n > 0 {
			return &n
		}
		return nil
	}
	v.DisplayStyle = entity.ViewDisplayStyle(r.FormValue("display_style"))
	if v.DisplayStyle != entity.ViewDisplayStyleTable && v.DisplayStyle != entity.ViewDisplayStyleList && v.DisplayStyle != entity.ViewDisplayStyleCarousel {
		v.DisplayStyle = entity.ViewDisplayStyleGrid
	}
	positive("grid_columns", 1, &v.GridColumns)
	// overflow_after - 0 means disabled.
	positive("overflow_after", 0, &v.OverflowAfter)
	positive("carousel_slides_per_view", 1, &v.CarouselSlidesPerView)
	v.CarouselAutoplay = r.FormValue("carousel_autoplay") == "1"
	v.CarouselLoop = r.FormValue("carousel_loop") == "1"
	v.CarouselShowNavigation = r.FormValue("carousel_show_navigation") == "1"
	v.CarouselShowPagination = r.FormValue("carousel_show_pagination") == "1"
	v.CarouselLinkFieldStorageID = storageID("carousel_link_field_storage_id")
	v.CarouselEffect = r.FormValue("carousel_effect")
	if v.CarouselEffect != "fade" {
		v.CarouselEffect = "slide"
	}
	v.ShowPagination = r.FormValue("show_pagination") == "1"
	v.PaginationAjax = r.FormValue("pagination_ajax") == "1"
	v.ShowGlossary = r.FormValue("show_glossary") == "1"
	// glossary_field_storage_id - nil is the default (the title).
	v.GlossaryFieldStorageID = storageID("glossary_field_storage_id")
	positive("page_size", 1, &v.PageSize)
	// header_text/footer_text - admin-authored rich text (TinyMCE), sanitized
	// with SanitizeAdmin rather than the stricter end-user Sanitize so "class"
	// and text-align "style" survive.
	v.HeaderText = htmlsanitize.SanitizeAdmin(r.FormValue("header_text"))
	v.FooterText = htmlsanitize.SanitizeAdmin(r.FormValue("footer_text"))
	// overflow_header_text - a short plain-text label, just trimmed.
	v.OverflowHeaderText = strings.TrimSpace(r.FormValue("overflow_header_text"))
	// custom_css - raw CSS, so an HTML sanitizer would mangle it. Admin-only
	// like header_text; the only defense needed is against breaking out of
	// the <style> tag it's embedded in (dynamic/list.html).
	v.CustomCSS = strings.TrimSpace(r.FormValue("custom_css"))
	if strings.Contains(strings.ToLower(v.CustomCSS), "</style") {
		return "custom-css-invalid"
	}
	// row_template - genuine HTML with literal tokens, same trust tier
	// as rewrite_html; SanitizeAdminRowTemplate keeps the {{url}} token in href.
	v.RowTemplate = strings.TrimSpace(htmlsanitize.SanitizeAdminRowTemplate(r.FormValue("row_template")))
	// group_by_field - one of the display's own fields (by machine
	// name), else no grouping; group_date_format - a known format.
	v.GroupByField = c.validGroupByField(ctx, *v, strings.TrimSpace(r.FormValue("group_by_field")))
	v.GroupDateFormat = r.FormValue("group_date_format")
	if !entity.ValidViewGroupDateFormat(v.GroupDateFormat) {
		v.GroupDateFormat = ""
	}
	return ""
}

// viewUpdateErrorCode - the form's error code of UpdateView's refusal;
// "" - not one (a failure).
func viewUpdateErrorCode(err error) string {
	for target, code := range map[error]string{
		views.ErrDuplicate:                    "duplicate",
		views.ErrSortFieldTypeUnsupported:     "sort-field",
		views.ErrGlossaryFieldTypeUnsupported: "glossary-field",
		views.ErrProfileTabPathInvalid:        "profile-tab-path",
		views.ErrProfileTabPathTaken:          "profile-tab-path-taken",
		views.ErrURLArgumentInvalid:           "url-argument",
		views.ErrURLPathTaken:                 "url-path-taken",
		views.ErrThemeTemplateInvalid:         "theme-template",
		views.ErrMachineNameInvalid:           "machine-name",
	} {
		if errors.Is(err, target) {
			return code
		}
	}
	return ""
}

// validGroupByField returns key when it names one of the display's
// effective fields, else "" (no grouping).
func (c *Controller) validGroupByField(ctx context.Context, display entity.ViewDisplayDef, key string) string {
	if key == "" {
		return ""
	}
	fields, err := c.viewService.EffectiveFieldsByView(ctx, display)
	if err != nil {
		c.LogFailure(err, "error loading view fields for grouping", "viewID", display.ID)
		return ""
	}
	storages, err := c.contentTypeService.FieldStorages(ctx)
	if err != nil {
		c.LogFailure(err, "error loading field storages for grouping")
		return ""
	}
	for _, o := range viewGroupFieldOptions(ctx, fields, fieldStorageMachineNames(storages)) {
		if o.Key == key {
			return key
		}
	}
	return ""
}

// viewFragmentFiles - what the View page's fragments need when rendered on
// their own (RenderNamed): their defines and the row action icons.
var viewFragmentFiles = []string{"admin/views/fragments.html", "admin/icons.html"}
