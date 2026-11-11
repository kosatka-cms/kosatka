package interfaces

import (
	"bytes"
	"context"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// viewGlossaryItem - a glossary bar entry plus its URL, keeping
// entity.GlossaryItem presentation-free.
type viewGlossaryItem struct {
	entity.GlossaryItem
	URL string
}

// exposedFilterKind - which HTML control an exposedFilterFormField
// renders as (dynamic/list.html's view_exposed_filters block).
type exposedFilterKind string

const (
	exposedFilterKindText   exposedFilterKind = "text"   // <input>, type set by InputType
	exposedFilterKindSelect exposedFilterKind = "select" // <select>, single-value
	// exposedFilterKindCheckboxes - a checkbox per option, several may be
	// ticked: an in/not_in filter (the values come comma-joined,
	// queryFilterValues).
	exposedFilterKindCheckboxes exposedFilterKind = "checkboxes"
	// exposedFilterKindSort - the visitors' choice of the exposed sort
	// criteria: a select with no "All" (one of them always applies).
	exposedFilterKindSort exposedFilterKind = "sort"
)

// exposedFilterFormField - the public form control for one exposed
// ViewFilterDef, resolved by buildExposedFilterFields (label fallback,
// taxonomy options).
type exposedFilterFormField struct {
	Label     string
	Param     string
	Kind      exposedFilterKind
	InputType string // "search" | "number" | "date" - only meaningful when Kind==exposedFilterKindText
	Value     string // current value from the request, for round-tripping into the control
	Options   []exposedFilterOption
	// Checked - the ticked options of a checkboxes field (Value split).
	Checked map[string]bool
}

// choiceFilterField - an exposed filter on options: a select for eq (one
// of them), checkboxes for in and not_in (several), the current value
// (comma-separated for several) marked.
func choiceFilterField(label, param string, op entity.ViewFilterOperator, value string, options []exposedFilterOption) exposedFilterFormField {
	field := exposedFilterFormField{Label: label, Param: param, Kind: exposedFilterKindSelect, Value: value, Options: options}
	if op == entity.ViewFilterOperatorIn || op == entity.ViewFilterOperatorNotIn {
		field.Kind, field.Checked = exposedFilterKindCheckboxes, map[string]bool{}
		for _, v := range strings.Split(value, ",") {
			if v = strings.TrimSpace(v); v != "" {
				field.Checked[v] = true
			}
		}
	}
	return field
}

type exposedFilterOption struct {
	Value string
	Label string
}

// exposedSortField - the visitors' choice of the display's exposed sort
// criteria (sorts in order), the applied one (entity.ViewSortDefs.
// WithChoice: chosen, else the first) selected; false - none offered.
func exposedSortField(ctx context.Context, sorts entity.ViewSortDefs, chosen string) (exposedFilterFormField, bool) {
	exposed := sorts.Exposed()
	if len(exposed) == 0 {
		return exposedFilterFormField{}, false
	}
	field := exposedFilterFormField{Label: i18n.T(ctx, "Sort by"), Param: entity.ViewSortParam, Kind: exposedFilterKindSort, Value: exposed[0].ExposedKey}
	for _, s := range exposed {
		field.Options = append(field.Options, exposedFilterOption{Value: s.ExposedKey, Label: s.ExposedLabel})
		if s.ExposedKey == chosen {
			field.Value = chosen
		}
	}
	return field, true
}

// buildExposedFilterFields resolves each exposed filter (sorted by the caller)
// into its public form control. fieldStorages, fieldStorageLabels and
// taxonomyOptions are pre-resolved by the caller.
//
// A choice filter (taxonomy, a list's options, a module's choices) is a
// select for eq and checkboxes for in/not_in (their repeated parameter
// comes comma-joined, queryFilterValues). Filters without a meaningful
// public control are omitted:
//   - empty/not_empty operators (no value to enter);
//   - field types without a control (image, reference, group) and
//     builtins other than "title" (the rest are staff-only filters).
func buildExposedFilterFields(
	ctx context.Context,
	filters entity.ViewFilterDefs,
	fieldStorages map[int]entity.FieldStorageDef,
	fieldStorageLabels map[int]string,
	taxonomyOptions map[int][]exposedFilterOption,
	queryParams map[string]string,
) []exposedFilterFormField {
	var out []exposedFilterFormField
	for _, f := range filters {
		if !f.Exposed || f.ExposedParam == "" {
			continue
		}
		label := func(fallback string) string {
			if f.ExposedLabel != "" {
				return f.ExposedLabel
			}
			return fallback
		}
		switch {
		case f.FieldStorageID != nil:
			fs, ok := fieldStorages[*f.FieldStorageID]
			if !ok {
				continue
			}
			fallbackLabel := fieldStorageLabels[fs.ID]
			if fallbackLabel == "" {
				fallbackLabel = fs.MachineName
			}
			switch fs.FieldType {
			case entity.FieldTypeText, entity.FieldTypeEmail, entity.FieldTypePhone:
				if f.Operator != entity.ViewFilterOperatorContains && f.Operator != entity.ViewFilterOperatorEq {
					continue
				}
				out = append(out, exposedFilterFormField{
					Label: label(fallbackLabel), Param: f.ExposedParam,
					Kind: exposedFilterKindText, InputType: "search",
					Value: queryParams[f.ExposedParam],
				})
			case entity.FieldTypeNumber:
				if f.Operator != entity.ViewFilterOperatorEq && f.Operator != entity.ViewFilterOperatorGte && f.Operator != entity.ViewFilterOperatorLte {
					continue
				}
				out = append(out, exposedFilterFormField{
					Label: label(fallbackLabel), Param: f.ExposedParam,
					Kind: exposedFilterKindText, InputType: "number",
					Value: queryParams[f.ExposedParam],
				})
			case entity.FieldTypeDate:
				if f.Operator != entity.ViewFilterOperatorEq && f.Operator != entity.ViewFilterOperatorGte && f.Operator != entity.ViewFilterOperatorLte {
					continue
				}
				out = append(out, exposedFilterFormField{
					Label: label(fallbackLabel), Param: f.ExposedParam,
					Kind: exposedFilterKindText, InputType: "date",
					Value: queryParams[f.ExposedParam],
				})
			case entity.FieldTypeBool:
				if f.Operator != entity.ViewFilterOperatorEq {
					continue
				}
				out = append(out, exposedFilterFormField{
					Label: label(fallbackLabel), Param: f.ExposedParam,
					Kind: exposedFilterKindSelect, Value: queryParams[f.ExposedParam],
					Options: []exposedFilterOption{{Value: "true", Label: i18n.T(ctx, "Yes")}, {Value: "false", Label: i18n.T(ctx, "No")}},
				})
			case entity.FieldTypeTaxonomy:
				// One term (eq), or several (in, not_in).
				if !isChoiceOperator(f.Operator) {
					continue
				}
				out = append(out, choiceFilterField(label(fallbackLabel), f.ExposedParam, f.Operator, queryParams[f.ExposedParam], taxonomyOptions[fs.ID]))
			case entity.FieldTypeList:
				// One of the list's options (eq), or several (in, not_in).
				if !isChoiceOperator(f.Operator) {
					continue
				}
				var options []exposedFilterOption
				for _, o := range (entity.FieldInstanceDef{Config: fs.Config}).AllowedValues() {
					options = append(options, exposedFilterOption{Value: o.Value, Label: o.Label})
				}
				out = append(out, choiceFilterField(label(fallbackLabel), f.ExposedParam, f.Operator, queryParams[f.ExposedParam], options))
			default:
				continue
			}
		case moduleChoiceFilter(f) != nil:
			// A module's choice filter: one of its options.
			if f.Operator != entity.ViewFilterOperatorEq && f.Operator != entity.ViewFilterOperatorIn {
				continue
			}
			mf := moduleChoiceFilter(f)
			opts, err := mf.Options(ctx)
			if err != nil {
				continue
			}
			var options []exposedFilterOption
			for _, o := range opts {
				options = append(options, exposedFilterOption{Value: o.Value, Label: o.Label})
			}
			out = append(out, choiceFilterField(label(i18n.T(ctx, mf.Label)), f.ExposedParam, f.Operator, queryParams[f.ExposedParam], options))
		case f.BuiltinField == entity.ViewFilterBuiltinTitle:
			if f.Operator != entity.ViewFilterOperatorContains && f.Operator != entity.ViewFilterOperatorEq {
				continue
			}
			out = append(out, exposedFilterFormField{
				Label: label(i18n.T(ctx, "Title")), Param: f.ExposedParam,
				Kind: exposedFilterKindText, InputType: "search",
				Value: queryParams[f.ExposedParam],
			})
		default:
			continue
		}
	}
	return out
}

// moduleChoiceFilter - f's module choice filter (views.ModuleFilter with
// Options), nil - f isn't one.
func moduleChoiceFilter(f entity.ViewFilterDef) *entity.ViewModuleFilter {
	if f.FieldStorageID != nil {
		return nil
	}
	mf, ok := views.ModuleFilter(string(f.BuiltinField))
	if !ok || !mf.Choice() {
		return nil
	}
	return &mf
}

// buildViewExposedFilterFields loads the field storages and taxonomy options
// the exposed filters need and builds their public form controls. Shared by
// ViewPageFallback (query string) and ViewPreview (posted draft values).
func (c *DynamicContentController) buildViewExposedFilterFields(ctx context.Context, viewID int, filters entity.ViewFilterDefs, queryParams map[string]string) []exposedFilterFormField {
	exposedFieldStorages := make(map[int]entity.FieldStorageDef)
	for _, f := range filters {
		if !f.Exposed || f.FieldStorageID == nil {
			continue
		}
		fs, err := c.contentTypeService.FieldStorageByID(ctx, *f.FieldStorageID)
		if err != nil {
			c.LogFailure(err, "error loading exposed filter's field storage", "viewID", viewID, "fieldStorageID", *f.FieldStorageID)
			continue
		}
		exposedFieldStorages[*f.FieldStorageID] = *fs
	}
	exposedFieldStorageLabels, err := c.contentTypeService.FieldInstanceLabelsByFieldStorageIDs(ctx, slices.Collect(maps.Keys(exposedFieldStorages)))
	if err != nil {
		c.LogFailure(err, "error loading exposed filter field labels", "viewID", viewID)
	}
	taxonomyOptions := make(map[int][]exposedFilterOption)
	for _, fs := range exposedFieldStorages {
		if fs.FieldType != entity.FieldTypeTaxonomy {
			continue
		}
		terms, err := c.taxonomyService.TermsByVocabularyID(ctx, fs.VocabularyID())
		if err != nil {
			c.LogFailure(err, "error loading exposed filter's taxonomy terms", "viewID", viewID, "fieldStorageID", fs.ID)
			continue
		}
		// Indented labels (BuildHierarchy) so child terms read as nested in
		// the <select>.
		hierarchyLabels := terms.BuildHierarchy()
		opts := make([]exposedFilterOption, 0, len(terms))
		for _, t := range terms.SortByHierarchy() {
			opts = append(opts, exposedFilterOption{Value: strconv.Itoa(t.ID), Label: hierarchyLabels[t.ID]})
		}
		taxonomyOptions[fs.ID] = opts
	}
	return buildExposedFilterFields(ctx, filters.SortBySortOrder(), exposedFieldStorages, exposedFieldStorageLabels, taxonomyOptions, queryParams)
}

// glossaryLetterURL builds a glossary bar link: the current query minus "page"
// (reset on letter switch) and "letter", so other params such as active
// exposed filters are preserved.
func glossaryLetterURL(baseURL string, current url.Values, letter string) string {
	params := url.Values{}
	for k, vs := range current {
		if k == "page" || k == "letter" {
			continue
		}
		params[k] = vs
	}
	params.Set("letter", letter)
	return baseURL + "?" + params.Encode()
}

// ViewPageFallback serves an admin-created View page (GET /{view.URLPath})
// when the slug lookup finds no node. It adds only an indexed lookup to the
// 404 path. url_path may have several segments; literal routes in router.go
// always take precedence, so a View can't shadow them.
func (c *DynamicContentController) ViewPageFallback(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Trim(r.URL.Path, "/")
	if path == "" {
		return false
	}
	ctx := r.Context()
	view, argUser, ok := c.viewPageOf(ctx, path)
	if !ok {
		return false
	}
	contextUserID := 0
	if argUser != nil {
		contextUserID = argUser.ID
	}
	// Disabled views are 404 for visitors, but admins/moderators can open them
	// to check the design (same gate as RequireVisibleOrModerator).
	user, _ := c.GetCurrentUser(r)
	if !c.IsViewVisible(r.Context(), view, user) {
		return false
	}
	unpublishedPreview := view.Status != 1
	// Only page displays have a page; other displays' URLPath is never
	// resolved.
	if view.DisplayType != entity.ViewDisplayTypePage {
		return false
	}
	// Comment views have their own, simpler render path (see
	// renderCommentViewPage).
	if view.BaseEntity == entity.ViewBaseEntityComment {
		return c.renderCommentViewPage(w, r, view, unpublishedPreview)
	}
	state, err := c.viewStateOf(ctx, *view)
	if err != nil {
		c.LogFailure(err, "error resolving effective view settings", "viewID", view.ID)
		httputil.InternalServerError(w)
		return true
	}
	// The View in the page's language: its title and meta description.
	localized := entity.LocalizeView(i18n.FromContext(ctx), state.View)
	view = &localized

	page := c.GetPageNumber(r)
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest"
	queryParams := queryFilterValues(r.URL.Query())

	nodes, total, err := c.viewService.RunConfig(ctx, state, views.RunContext{QueryParams: queryParams, Page: page, CreatedByID: contextUserID})
	if err != nil {
		c.LogFailure(err, "error running view", "viewID", view.ID)
		httputil.InternalServerError(w)
		return true
	}
	// The list - the View's fields and each node's values for them
	// (views.BuildContentList, as for its block and its preview); a page
	// shows the nodes' titles, not their alternative titles.
	viewFields := state.Fields
	listFields, items, err := views.BuildContentList(ctx, c.contentListSources(), view.ID, viewFields, nodes, views.ContentListOptions{})
	if err != nil {
		c.LogFailure(err, "error building the view's list", "viewID", view.ID)
		httputil.InternalServerError(w)
		return true
	}
	// filters and sorts: needed only to build the exposed-filters form;
	// the run applies them.
	filters, sorts := state.Filters, state.Sorts

	// Exposed-filters form (and the visitors' choice of sort, last);
	// skipped for AJAX requests since the form is outside the re-rendered
	// list body.
	var exposedParams []string
	var exposedFields []exposedFilterFormField
	for _, f := range filters {
		if f.Exposed && f.ExposedParam != "" {
			exposedParams = append(exposedParams, f.ExposedParam)
		}
	}
	sortField, sortExposed := exposedSortField(ctx, sorts.SortBySortOrder(), queryParams[entity.ViewSortParam])
	if sortExposed {
		exposedParams = append(exposedParams, entity.ViewSortParam)
	}
	if !isAjax {
		exposedFields = c.buildViewExposedFilterFields(ctx, view.ID, filters, queryParams)
		if sortExposed {
			exposedFields = append(exposedFields, sortField)
		}
	}

	label, pageSEO := viewPageSEO(view, argUser, exposedParams, r.URL.Query())
	data := c.GetCommonData(r)
	data["title"] = label
	c.setPathTrail(data, "/"+path, label)
	data["pageSEO"] = pageSEO
	data["viewUnpublished"] = unpublishedPreview
	data["exposedFilters"] = exposedFields
	data["viewURLPath"] = path
	maps.Copy(data, views.ContentListData(*view, listFields, items, c.resolveCarouselLinkURLs(ctx, view.CarouselLinkFieldStorageIDValue(), nodes)))
	data["showPagination"] = view.ShowPagination
	pager := pagination.New(page, total, view.PageSize, "/"+path, r.URL.Query())
	data["pagination"] = pager
	// paginationAjax - read by app.js's initViewPagination to decide whether
	// to intercept pager clicks.
	data["paginationAjax"] = view.ShowPagination && view.PaginationAjax

	// Glossary bar; skipped for AJAX requests since it lives outside the
	// re-rendered list (the active letter is toggled client-side).
	if view.ShowGlossary && !isAjax {
		if glossaryItems, ok := c.viewGlossaryBar(ctx, state, queryParams, "/"+path, r.URL.Query()); ok {
			data["glossaryItems"] = glossaryItems
		}
	}
	data["showGlossary"] = view.ShowGlossary
	data["activeLetter"] = strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("letter")))

	// Theme template (ViewDisplayDef.ThemeTemplate): the list body comes
	// from the theme's views/<name>.html instead of the field grid.
	if view.ThemeTemplate != "" {
		var themePager *pagination.Pagination
		if view.ShowPagination {
			themePager = pager
		}
		html, ok, err := c.renderThemeViewList(ctx, view.ThemeTemplate, nodes, themePager)
		if err != nil {
			c.LogFailure(err, "error rendering view page theme template", "viewID", view.ID)
		} else if ok {
			data["themeListHTML"] = html
		}
	}

	// AJAX pagination fetches the same "?page=N" URL with X-Requested-With;
	// respond with just the list and pager fragment.
	if isAjax {
		var buf bytes.Buffer
		withRequestContext(r.Context(), data)
		if err := c.templateService.RenderNamed(&buf, "view_list_body", data, "dynamic/list.html", "pagination.html"); err != nil {
			c.LogFailure(err, "error rendering view list fragment", "viewID", view.ID)
			httputil.InternalServerError(w)
			return true
		}
		httputil.JSON(w, map[string]any{"html": buf.String()}, http.StatusOK)
		return true
	}

	c.RenderOrFail(w, r, "dynamic/list.html", data)
	return true
}

// viewPageOf - the View display at path, and the user its path argument
// names ("articles/%": a user's alias, used by the created_by contextual
// filter; nil - none); false - none (an unknown or blocked user too, a
// 404), or path is the literal pattern itself, not a value.
func (c *DynamicContentController) viewPageOf(ctx context.Context, path string) (*entity.ViewDisplayDef, *entity.User, bool) {
	view, err := c.viewService.ViewByURLPath(ctx, path)
	if err == nil {
		return view, nil, !strings.Contains(view.URLPath, entity.ViewURLArgument)
	}
	c.LogFailure(err, "loading the View display by its address", "path", path)
	view, arg, err := c.viewService.ViewByURLArgument(ctx, path)
	if err != nil {
		return nil, nil, false
	}
	u, err := c.userService.FindBySlug(ctx, entity.UserSlugFromAlias(arg))
	if err != nil || u == nil || u.Status == entity.UserStatusBlocked {
		return nil, nil, false
	}
	return view, u, true
}

// viewStateOf - view as its page shows it (views.Service.PublishedConfig:
// once published, its published snapshot for every viewer, admins
// included - the live state only through the admin preview; never
// published, live): View - its settings resolved (identity fields
// unchanged).
func (c *DynamicContentController) viewStateOf(ctx context.Context, view entity.ViewDisplayDef) (views.Config, error) {
	return c.viewService.PublishedConfig(ctx, view)
}

// viewGlossaryBar - the glossary bar's letters with their links (base -
// the page's address, current - its query); false - it failed (logged:
// the page shows without the bar).
func (c *DynamicContentController) viewGlossaryBar(ctx context.Context, st views.Config, queryParams map[string]string, base string, current url.Values) ([]viewGlossaryItem, bool) {
	glossary, err := c.viewService.GlossaryOf(ctx, st, queryParams)
	if err != nil {
		c.LogFailure(err, "error computing view glossary", "viewID", st.View.ID)
		return nil, false
	}
	items := make([]viewGlossaryItem, len(glossary))
	for i, g := range glossary {
		items[i] = viewGlossaryItem{GlossaryItem: g, URL: glossaryLetterURL(base, current, g.Letter)}
	}
	return items, true
}

// viewPageSEO - a View page's heading and SEO: MetaTitle/MetaDescription
// override Label; %1 in the label/meta title is the URL argument's user
// name; NoIndex when an exposed filter is active (filtered URLs have no
// unique content).
func viewPageSEO(view *entity.ViewDisplayDef, argUser *entity.User, exposedParams []string, query url.Values) (string, entity.PageSEO) {
	label, metaTitle := view.Label, view.MetaTitle
	if argUser != nil {
		label = strings.ReplaceAll(label, "%1", argUser.Username)
		metaTitle = strings.ReplaceAll(metaTitle, "%1", argUser.Username)
	}
	pageSEO := entity.PageSEO{Title: label}
	if metaTitle != "" {
		pageSEO.Title = metaTitle
	}
	if view.MetaDescription != "" {
		pageSEO.Description = view.MetaDescription
	}
	if len(exposedParams) > 0 && filterParamActive(query, exposedParams...) {
		pageSEO.NoIndex = true
	}
	return label, pageSEO
}

// renderCommentViewPage is ViewPageFallback's path for comment-based Views
// (as visitors see them: views.Service.PublishedConfig). It is
// intentionally simpler: no glossary, no AJAX pagination and no
// exposed-filters form.
func (c *DynamicContentController) renderCommentViewPage(w http.ResponseWriter, r *http.Request, display *entity.ViewDisplayDef, unpublishedPreview bool) bool {
	ctx := r.Context()

	page := c.GetPageNumber(r)
	queryParams := queryFilterValues(r.URL.Query())

	cfg, err := c.viewService.PublishedConfig(ctx, *display)
	if err != nil {
		c.LogFailure(err, "error resolving comment view settings", "viewID", display.ID)
		httputil.InternalServerError(w)
		return true
	}
	view := &cfg.View
	comments, total, err := c.viewService.RunCommentConfig(ctx, cfg, views.RunContext{QueryParams: queryParams, Page: page})
	if err != nil {
		c.LogFailure(err, "error running comment view", "viewID", view.ID)
		httputil.InternalServerError(w)
		return true
	}

	// viewFields - the display's field selection, used by both list and
	// table styles in comment_list.html.
	viewFields := cfg.Fields

	pageSEO := entity.PageSEO{Title: view.Label}
	if view.MetaTitle != "" {
		pageSEO.Title = view.MetaTitle
	}
	if view.MetaDescription != "" {
		pageSEO.Description = view.MetaDescription
	}

	data := c.GetCommonData(r)
	maps.Copy(data, views.CommentListData(ctx, *view, viewFields, comments))
	data["title"] = view.Label
	c.setPathTrail(data, "/"+view.URLPath, view.Label)
	data["pageSEO"] = pageSEO
	data["viewUnpublished"] = unpublishedPreview
	data["viewURLPath"] = view.URLPath
	data["showPagination"] = view.ShowPagination
	pager := pagination.New(page, total, view.PageSize, "/"+view.URLPath, r.URL.Query())
	data["pagination"] = pager
	if view.ThemeTemplate != "" {
		var themePager *pagination.Pagination
		if view.ShowPagination {
			themePager = pager
		}
		html, ok, err := c.renderThemeCommentViewList(r.Context(), view.ThemeTemplate, comments, themePager)
		if err != nil {
			c.LogFailure(err, "error rendering comment view theme template", "viewID", view.ID)
		} else if ok {
			data["themeListHTML"] = html
		}
	}

	c.RenderOrFail(w, r, "dynamic/comment_list.html", data)
	return true
}

// isChoiceOperator - an operator an exposed choice filter has a control
// for: eq (one option), in and not_in (several).
func isChoiceOperator(op entity.ViewFilterOperator) bool {
	return op == entity.ViewFilterOperatorEq || op == entity.ViewFilterOperatorIn || op == entity.ViewFilterOperatorNotIn
}

// queryFilterValues - each query parameter's value (a View's exposed
// filters, its contextual arguments, the glossary letter); a parameter
// repeated - ticked checkboxes of an in filter - its non-empty values
// joined with ",", the form the filters and the pager take.
func queryFilterValues(q url.Values) map[string]string {
	values := make(map[string]string, len(q))
	for k, vs := range q {
		var kept []string
		for _, v := range vs {
			if v != "" {
				kept = append(kept, v)
			}
		}
		switch {
		case len(kept) > 1:
			values[k] = strings.Join(kept, ",")
		case len(kept) == 1:
			values[k] = kept[0]
		case len(vs) > 0:
			values[k] = ""
		}
	}
	return values
}
