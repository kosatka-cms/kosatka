package admin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// renderFiltersFragment renders the "Filters" card body - for ViewEdit's
// initial render and every filter mutation's AJAX response. Uses
// EffectiveFiltersByView so an inherited section shows Default's rows
// (read-only, see CanEditFilters in admin/views/page.html).
func (c *Controller) renderFiltersFragment(ctx context.Context, w io.Writer, view *entity.ViewDisplayDef) error {
	filters, err := c.viewService.EffectiveFiltersByView(ctx, *view)
	if err != nil {
		return err
	}
	allFieldStorages, err := c.contentTypeService.FieldStorages(ctx)
	if err != nil {
		return err
	}
	fieldStorageTypes := make(map[int]entity.ContentTypeFieldType, len(allFieldStorages))
	for _, fs := range allFieldStorages {
		fieldStorageTypes[fs.ID] = fs.FieldType
	}
	data := map[string]any{
		"view":                view,
		"filters":             filters,
		"fieldLabels":         c.resolveFieldStorageLabels(ctx, allFieldStorages),
		"operators":           viewFilterOperators,
		"filterValueDisplays": filterValueDisplays(ctx, filters, fieldStorageTypes, c.resolveTermNames(ctx, allFieldStorages)),
	}
	addViewModuleCriteria(ctx, data, view)
	return c.templateService.RenderNamed(w, "view_filters_list", data, viewFragmentFiles...)
}

// resolveTermNames - term ID -> Name across every taxonomy field in
// fieldStorages, so the filters table shows term names instead of raw IDs.
func (c *Controller) resolveTermNames(ctx context.Context, fieldStorages entity.FieldStorageDefs) map[int]string {
	names := make(map[int]string)
	for _, fs := range fieldStorages {
		if fs.FieldType != entity.FieldTypeTaxonomy {
			continue
		}
		terms, err := c.taxonomyService.TermsByVocabularyID(ctx, fs.VocabularyID())
		if err != nil {
			c.LogFailure(err, "error loading terms for filter display", "fieldStorageID", fs.ID)
			continue
		}
		for _, t := range terms {
			names[t.ID] = t.Name
		}
	}
	return names
}

// filterValueDisplays precomputes each filter's "Value" cell: the raw
// value, except taxonomy term IDs resolved to names via termNames
// (html/template can't split strings) and relative dates shown with a
// "relative to the current date" prefix so they don't read as literal
// (invalid) dates.
func filterValueDisplays(ctx context.Context, filters entity.ViewFilterDefs, fieldStorageTypes map[int]entity.ContentTypeFieldType, termNames map[int]string) map[int]string {
	displays := make(map[int]string, len(filters))
	for _, f := range filters {
		if f.ValueIsRelative {
			expr := f.Value
			if expr == "" {
				expr = "now"
			}
			displays[f.ID] = i18n.T(ctx, "relative to the current date: @expr", "@expr", expr)
			continue
		}
		fsID := f.FieldStorageIDValue()
		if fsID == 0 || fieldStorageTypes[fsID] != entity.FieldTypeTaxonomy {
			displays[f.ID] = f.Value
			continue
		}
		parts := strings.Split(f.Value, ",")
		names := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if id, err := strconv.Atoi(p); err == nil {
				if name, ok := termNames[id]; ok {
					names = append(names, name)
					continue
				}
			}
			names = append(names, p)
		}
		displays[f.ID] = strings.Join(names, ", ")
	}
	return displays
}

// parseFilterTarget - a filter's target as the picker sends it
// (entity.ViewFilterDef.Target): "field:<storage>", "block:<blocks
// field>:<storage>" - a field of the blocks' sets - or "builtin:<name>";
// false - none of them.
func parseFilterTarget(target string) (fieldStorageID, viaFieldStorageID *int, builtin entity.ViewFilterBuiltinField, ok bool) {
	id := func(s string) (*int, bool) {
		v, err := strconv.Atoi(s)
		return &v, err == nil && v > 0
	}
	switch {
	case strings.HasPrefix(target, "field:"):
		fieldStorageID, ok = id(strings.TrimPrefix(target, "field:"))
		return fieldStorageID, nil, "", ok
	case strings.HasPrefix(target, "block:"):
		via, field, found := strings.Cut(strings.TrimPrefix(target, "block:"), ":")
		if !found {
			return nil, nil, "", false
		}
		var okVia bool
		viaFieldStorageID, okVia = id(via)
		fieldStorageID, ok = id(field)
		return fieldStorageID, viaFieldStorageID, "", ok && okVia
	case strings.HasPrefix(target, "builtin:"):
		return nil, nil, entity.ViewFilterBuiltinField(strings.TrimPrefix(target, "builtin:")), true
	}
	return nil, nil, "", false
}

// ViewFilterCreate handles POST /admin/config/views/{id}/filters
// (viewRows.add); target - parseFilterTarget.
func (c *Controller) ViewFilterCreate(w http.ResponseWriter, r *http.Request) {
	c.filterRows().add(c, w, r)
}

// filterRows - the filters as viewRows edits them.
func (c *Controller) filterRows() viewRows[entity.ViewFilterDefs, entity.ViewFilterDef] {
	return viewRows[entity.ViewFilterDefs, entity.ViewFilterDef]{
		what: "filters", pathKey: "filter_id",
		canEdit: (*entity.ViewDisplayDef).CanEditFilters,
		list:    c.viewService.FiltersByViewID, byID: c.viewService.FilterByID,
		create: c.viewService.CreateFilter, update: c.viewService.UpdateFilter, delete: c.viewService.DeleteFilter,
		override: func(ctx context.Context, id int) error {
			_, err := c.viewService.OverrideFilters(ctx, id)
			return err
		},
		revert:   c.viewService.RevertFilters,
		render:   c.renderFiltersFragment,
		fromForm: filterFromForm, errorCode: filterErrorCode,
		target: func(f entity.ViewFilterDef) (*int, string) { return f.FieldStorageID, string(f.BuiltinField) },
		place:  func(f entity.ViewFilterDef) (int, int, int) { return f.ID, f.ViewDisplayID, f.SortOrder },
		placed: func(f entity.ViewFilterDef, order int) entity.ViewFilterDef {
			f.SortOrder = order
			return f
		},
	}
}

// filterFromForm - viewRows.fromForm of the filters: all of a filter
// is edited, its target too (parseFilterTarget).
func filterFromForm(r *http.Request, displayID int, old *entity.ViewFilterDef) (entity.ViewFilterDef, string) {
	fieldStorageID, viaFieldStorageID, builtinField, ok := parseFilterTarget(r.FormValue("target"))
	if !ok {
		return entity.ViewFilterDef{}, "empty"
	}
	f := entity.ViewFilterDef{
		ViewDisplayID:     displayID,
		FieldStorageID:    fieldStorageID,
		ViaFieldStorageID: viaFieldStorageID,
		BuiltinField:      builtinField,
		Operator:          entity.ViewFilterOperator(r.FormValue("operator")),
		Value:             strings.TrimSpace(r.FormValue("value")),
		ValueIsRelative:   r.FormValue("value_is_relative") == "1",
		Exposed:           r.FormValue("exposed") == "1",
		ExposedParam:      strings.TrimSpace(r.FormValue("exposed_param")),
		ExposedLabel:      strings.TrimSpace(r.FormValue("exposed_label")),
		Contextual:        r.FormValue("contextual") == "1",
	}
	if old != nil {
		f.ID, f.SortOrder = old.ID, old.SortOrder
	}
	return f, ""
}

// filterErrorCode - viewRows.errorCode of the filters.
func filterErrorCode(err error) string {
	switch {
	case errors.Is(err, views.ErrFilterOperatorUnsupported):
		return "operator"
	case errors.Is(err, views.ErrFilterFieldTypeUnsupported):
		return "field-type"
	case errors.Is(err, views.ErrFilterExposedParamRequired):
		return "exposed-param"
	case errors.Is(err, views.ErrFilterExposedParamInvalid):
		return "exposed-param-invalid"
	case errors.Is(err, views.ErrFilterExposedParamTaken):
		return "exposed-param-taken"
	case errors.Is(err, views.ErrFilterInvalidRelativeDate):
		return "invalid-relative-date"
	case errors.Is(err, views.ErrFilterContextualExposedConflict):
		return "contextual-conflict"
	case errors.Is(err, views.ErrFilterContextualUnsupported):
		return "contextual-unsupported"
	}
	return ""
}

// ViewFilterUpdate handles POST /admin/config/views/{id}/filters/{filter_id}
// (viewRows.edit).
func (c *Controller) ViewFilterUpdate(w http.ResponseWriter, r *http.Request) {
	c.filterRows().edit(c, w, r)
}

// ViewFiltersReorder handles POST /admin/config/views/{id}/filters/reorder
// (viewRows.reorder).
func (c *Controller) ViewFiltersReorder(w http.ResponseWriter, r *http.Request) {
	c.filterRows().reorder(c, w, r)
}

// ViewFilterDeleteConfirm renders GET
// /admin/config/views/{id}/filters/{filter_id}/delete (viewRows.deletePage).
func (c *Controller) ViewFilterDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	c.filterRows().deletePage(c, w, r, "admin/views/filter_delete.html", func(ctx context.Context, label string) (string, string) {
		return i18n.T(ctx, "Delete filter: @label", "@label", label), i18n.T(ctx, "Delete filter - @label", "@label", label)
	})
}

// ViewFilterDelete handles POST
// /admin/config/views/{id}/filters/{filter_id}/delete (viewRows.remove).
func (c *Controller) ViewFilterDelete(w http.ResponseWriter, r *http.Request) {
	c.filterRows().remove(c, w, r)
}
