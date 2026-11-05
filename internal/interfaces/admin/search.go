package admin

import (
	"context"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Search - which node types the live search covers and how a result is
// shown.

const searchAdminURL = "/admin/config/search"

// searchTypeRow - a searched type as the list shows it; TextFields - it
// has text fields (AllFields searches something besides the title).
type searchTypeRow struct {
	Def        entity.SearchTypeDef
	TypeLabel  string
	TextFields bool
}

// typesWithTextFields - the content types that have text fields (a
// module's type has none).
func (c *Controller) typesWithTextFields(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	defs, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		c.LogFailure(err, "error loading the content types for search")
		return out
	}
	for _, d := range defs {
		fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, d.ID)
		if err != nil {
			c.LogFailure(err, "error loading the content type's fields for search", "contentTypeID", d.ID)
			continue
		}
		out[d.MachineName] = slices.ContainsFunc(fields, func(f entity.FieldInstanceDef) bool { return f.FieldType == entity.FieldTypeText })
	}
	return out
}

// searchFieldOption - a field of a node type for the type form.
type searchFieldOption struct {
	MachineName string
	Label       string
}

// searchTypeFields - the text and image fields of a node type (none for a
// built-in module's type).
func (c *Controller) searchTypeFields(ctx context.Context, nodeType string) (text, images []searchFieldOption) {
	ct, err := c.contentTypeService.ContentTypeByMachineName(ctx, nodeType)
	if err != nil || ct == nil {
		c.LogFailure(err, "loading the content type", "machineName", nodeType)
		return nil, nil
	}
	fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, ct.ID)
	if err != nil {
		c.LogFailure(err, "loading the content type's fields", "contentTypeID", ct.ID)
		return nil, nil
	}
	for _, f := range fields {
		opt := searchFieldOption{MachineName: f.MachineName, Label: f.Label}
		switch f.FieldType {
		case entity.FieldTypeText:
			text = append(text, opt)
		case entity.FieldTypeImage:
			images = append(images, opt)
		}
	}
	return text, images
}

// SearchConfigPage renders GET /admin/config/search.
func (c *Controller) SearchConfigPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	types, err := c.searchService.Types(ctx)
	if err != nil {
		c.LogFailure(err, "error loading search types")
		httputil.InternalServerError(w)
		return
	}
	settings, err := c.searchService.Settings(ctx)
	if err != nil {
		c.LogFailure(err, "error loading search settings")
		httputil.InternalServerError(w)
		return
	}
	menus, err := c.menuService.Menus(ctx)
	if err != nil {
		c.LogFailure(err, "error loading menus for search settings")
		httputil.InternalServerError(w)
		return
	}
	labels := c.nodeTypeLabels(ctx)
	withText := c.typesWithTextFields(ctx)
	rows := make([]searchTypeRow, 0, len(types))
	used := make(map[string]bool, len(types))
	for _, t := range types {
		rows = append(rows, searchTypeRow{Def: t, TypeLabel: labels[t.NodeType], TextFields: withText[t.NodeType]})
		used[t.NodeType] = true
	}
	var available []searchFieldOption
	for t, l := range labels {
		if !used[t] {
			available = append(available, searchFieldOption{MachineName: t, Label: l})
		}
	}
	sort.Slice(available, func(i, j int) bool { return available[i].Label < available[j].Label })

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Search")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Search"))}
	data["types"] = rows
	data["available"] = available
	data["settings"] = settings
	data["searchEnabled"] = service.FeatureEnabled(entity.FeatureSearch)
	data["menus"] = menus
	data["success"] = r.URL.Query().Get("success") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/search/index.html", data)
}

// SearchSettingsSave handles POST /admin/config/search/settings.
func (c *Controller) SearchSettingsSave(w http.ResponseWriter, r *http.Request) {
	settings := entity.SearchSettings{
		PlaceholderImage: strings.TrimSpace(r.FormValue("placeholder_image")),
		CategoriesMenu:   strings.TrimSpace(r.FormValue("categories_menu")),
	}
	if err := c.searchService.SaveSettings(r.Context(), settings); err != nil {
		c.LogFailure(err, "error saving search settings")
		http.Redirect(w, r, searchAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	if err := c.saveFeature(r.Context(), entity.FeatureSearch, r.FormValue("search_enabled") == "1"); err != nil {
		c.LogFailure(err, "error saving the search switch")
		http.Redirect(w, r, searchAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, searchAdminURL+"?success=1", http.StatusSeeOther)
}

// SearchTypeAdd handles POST /admin/config/search/types: the type goes
// last with defaults and opens its form.
func (c *Controller) SearchTypeAdd(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodeType := strings.TrimSpace(r.FormValue("node_type"))
	labels := c.nodeTypeLabels(ctx)
	if _, ok := labels[nodeType]; !ok {
		http.Redirect(w, r, searchAdminURL+"?error=type", http.StatusSeeOther)
		return
	}
	def := entity.SearchTypeDef{NodeType: nodeType, Limit: entity.DefaultSearchLimit, AllFields: true, Label: labels[nodeType]}
	if err := c.searchService.SaveType(ctx, def, true); err != nil {
		c.LogFailure(err, "error adding search type", "type", nodeType)
		http.Redirect(w, r, searchAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, searchAdminURL+"/types/"+nodeType+"/edit", http.StatusSeeOther)
}

// searchTypeByPath - the searched type of {type} in the path.
func (c *Controller) searchTypeByPath(r *http.Request) (*entity.SearchTypeDef, bool) {
	nodeType := r.PathValue("type")
	types, err := c.searchService.Types(r.Context())
	if err != nil {
		return nil, false
	}
	for i := range types {
		if types[i].NodeType == nodeType {
			return &types[i], true
		}
	}
	return nil, false
}

// SearchTypeEdit renders GET /admin/config/search/types/{type}/edit.
func (c *Controller) SearchTypeEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	def, ok := c.searchTypeByPath(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	textFields, imageFields := c.searchTypeFields(ctx, def.NodeType)
	styles, err := c.imageStyleService.ImageStyles(ctx)
	if err != nil {
		c.LogFailure(err, "error loading image styles for search")
		httputil.InternalServerError(w)
		return
	}
	vocabularies, err := c.taxonomyService.Vocabularies(ctx)
	if err != nil {
		c.LogFailure(err, "error loading vocabularies for search")
		httputil.InternalServerError(w)
		return
	}
	match := make(map[string]bool, len(def.MatchFields))
	for _, f := range def.MatchFields {
		match[f] = true
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Search: @type", "@type", c.nodeTypeLabels(ctx)[def.NodeType])
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Search"))}
	data["def"] = def
	data["match"] = match
	data["textFields"] = textFields
	data["imageFields"] = imageFields
	data["imageStyles"] = styles
	data["vocabularies"] = vocabularies
	data["success"] = r.URL.Query().Get("success") == "1"

	c.RenderOrFail(w, r, "admin/search/type_edit.html", data)
}

// SearchTypeSave handles POST /admin/config/search/types/{type}.
func (c *Controller) SearchTypeSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	existing, ok := c.searchTypeByPath(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, searchAdminURL, http.StatusSeeOther)
		return
	}
	def := *existing
	def.Limit, _ = strconv.Atoi(r.FormValue("result_limit"))
	if def.Limit < 0 {
		def.Limit = 0
	}
	def.Label = strings.TrimSpace(r.FormValue("label"))
	// All the text fields, or the ones ticked (kept for the form either
	// way). A type without text fields has no choice: it stays as it is.
	if mode := r.FormValue("match_mode"); mode != "" {
		def.AllFields = mode == "all"
	}
	def.MatchFields = nil
	for _, f := range r.Form["match_fields"] {
		if f = strings.TrimSpace(f); f != "" {
			def.MatchFields = append(def.MatchFields, f)
		}
	}
	def.TitleField = strings.TrimSpace(r.FormValue("title_field"))
	def.SubtitleField = strings.TrimSpace(r.FormValue("subtitle_field"))
	def.ImageField = strings.TrimSpace(r.FormValue("image_field"))
	def.ImageStyle = strings.TrimSpace(r.FormValue("image_style"))
	def.BadgeVocabularyID, _ = strconv.Atoi(r.FormValue("badge_vocabulary_id"))
	editURL := searchAdminURL + "/types/" + def.NodeType + "/edit"
	if err := c.searchService.SaveType(ctx, def, false); err != nil {
		c.LogFailure(err, "error saving search type", "type", def.NodeType)
		http.Redirect(w, r, editURL+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, editURL+"?success=1", http.StatusSeeOther)
}

// SearchTypeDelete handles POST /admin/config/search/types/{type}/delete -
// the type is left out of the search (switched off, its settings kept).
func (c *Controller) SearchTypeDelete(w http.ResponseWriter, r *http.Request) {
	if err := c.searchService.RemoveType(r.Context(), r.PathValue("type")); err != nil {
		c.LogFailure(err, "error deleting search type")
		http.Redirect(w, r, searchAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, searchAdminURL+"?success=1", http.StatusSeeOther)
}

// SearchTypesReorder handles POST /admin/config/search/types/reorder -
// the drag-and-drop order as a CSV of node types in "order".
func (c *Controller) SearchTypesReorder(w http.ResponseWriter, r *http.Request) {
	var order []string
	for _, t := range strings.Split(r.FormValue("order"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			order = append(order, t)
		}
	}
	if len(order) == 0 {
		http.Redirect(w, r, searchAdminURL, http.StatusSeeOther)
		return
	}
	if err := c.searchService.ReorderTypes(r.Context(), order); err != nil {
		c.LogFailure(err, "error reordering search types")
		http.Redirect(w, r, searchAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, searchAdminURL+"?success=1", http.StatusSeeOther)
}
