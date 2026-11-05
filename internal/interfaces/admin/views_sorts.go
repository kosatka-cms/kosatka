package admin

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

// renderSortsFragment renders the "Sort" card body, like
// renderFiltersFragment.
func (c *Controller) renderSortsFragment(ctx context.Context, w io.Writer, view *entity.ViewDisplayDef) error {
	sorts, err := c.viewService.EffectiveSortsByView(ctx, *view)
	if err != nil {
		return err
	}
	allFieldStorages, err := c.contentTypeService.FieldStorages(ctx)
	if err != nil {
		return err
	}
	data := map[string]any{
		"view":              view,
		"sorts":             sorts,
		"fieldLabels":       c.resolveFieldStorageLabels(ctx, allFieldStorages),
		"sortBuiltinLabels": translatedLabelMap(ctx, sortBuiltinLabels),
	}
	addViewModuleCriteria(ctx, data, view)
	return c.templateService.RenderNamed(w, "view_sorts_list", data, viewFragmentFiles...)
}

// ViewSortCreate handles POST /admin/config/views/{id}/sorts
// (viewRows.add).
func (c *Controller) ViewSortCreate(w http.ResponseWriter, r *http.Request) {
	c.sortRows().add(c, w, r)
}

// sortRows - the sort criteria as viewRows edits them.
func (c *Controller) sortRows() viewRows[entity.ViewSortDefs, entity.ViewSortDef] {
	return viewRows[entity.ViewSortDefs, entity.ViewSortDef]{
		what: "sorts", pathKey: "sort_id",
		canEdit: (*entity.ViewDisplayDef).CanEditSort,
		list:    c.viewService.SortsByViewID, byID: c.viewService.SortByID,
		create: c.viewService.CreateSort, update: c.viewService.UpdateSort, delete: c.viewService.DeleteSort,
		override: func(ctx context.Context, id int) error {
			_, err := c.viewService.OverrideSort(ctx, id)
			return err
		},
		revert:    c.viewService.RevertSort,
		render:    c.renderSortsFragment,
		fromForm:  sortFromForm,
		errorCode: sortErrorCode,
		target:    func(sd entity.ViewSortDef) (*int, string) { return sd.FieldStorageID, string(sd.BuiltinField) },
		place:     func(sd entity.ViewSortDef) (int, int, int) { return sd.ID, sd.ViewDisplayID, sd.SortOrder },
		placed: func(sd entity.ViewSortDef, order int) entity.ViewSortDef {
			sd.SortOrder = order
			return sd
		},
	}
}

// sortErrorCode - viewRows.errorCode of the sort criteria.
func sortErrorCode(err error) string {
	switch {
	case errors.Is(err, views.ErrSortFieldTypeUnsupported):
		return "field-type"
	case errors.Is(err, views.ErrSortExposedKeyInvalid):
		return "exposed-key"
	case errors.Is(err, views.ErrSortExposedLabelRequired):
		return "exposed-label"
	case errors.Is(err, views.ErrSortExposedKeyTaken):
		return "exposed-key-taken"
	}
	return ""
}

// sortFromForm - viewRows.fromForm of the sort criteria: the target
// (parseRowTarget) is set when a criterion is added, then kept -
// changing it means delete and re-add; edited - the direction, the offer
// to visitors and a module's criterion's settings.
func sortFromForm(r *http.Request, displayID int, old *entity.ViewSortDef) (entity.ViewSortDef, string) {
	sd := entity.ViewSortDef{ViewDisplayID: displayID, Direction: r.FormValue("direction")}
	if r.FormValue("exposed") == "1" {
		sd.ExposedKey, sd.ExposedLabel = r.FormValue("exposed_key"), r.FormValue("exposed_label")
	}
	if sd.Direction != "asc" && sd.Direction != "desc" {
		sd.Direction = "desc"
	}
	if old != nil {
		sd.ID, sd.SortOrder = old.ID, old.SortOrder
		sd.FieldStorageID, sd.BuiltinField, sd.Params = old.FieldStorageID, old.BuiltinField, old.Params
	} else {
		fieldStorageID, builtin, ok := parseRowTarget(r.FormValue("target"))
		if !ok {
			return sd, "empty"
		}
		sd.FieldStorageID, sd.BuiltinField = fieldStorageID, entity.ViewSortBuiltin(builtin)
	}
	// A module's criterion: its settings in this View (its module off -
	// kept).
	if ms, ok := views.ModuleSort(string(sd.BuiltinField)); ok {
		if sd.Params, ok = moduleParamsFromForm(r, ms.Params); !ok {
			return sd, "params"
		}
	}
	return sd, ""
}

// ViewSortUpdate handles POST /admin/config/views/{id}/sorts/{sort_id}
// (viewRows.edit).
func (c *Controller) ViewSortUpdate(w http.ResponseWriter, r *http.Request) {
	c.sortRows().edit(c, w, r)
}

// ViewSortsReorder handles POST /admin/config/views/{id}/sorts/reorder
// (viewRows.reorder): sorts[0] is the primary key, later entries are
// tie-breakers.
func (c *Controller) ViewSortsReorder(w http.ResponseWriter, r *http.Request) {
	c.sortRows().reorder(c, w, r)
}

// ViewSortDelete handles POST
// /admin/config/views/{id}/sorts/{sort_id}/delete (viewRows.remove).
func (c *Controller) ViewSortDelete(w http.ResponseWriter, r *http.Request) {
	c.sortRows().remove(c, w, r)
}
