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
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// fieldStorageMachineNames - field_storage_id → machine_name, shown next
// to a View field's label in the fields list.
func fieldStorageMachineNames(fieldStorages entity.FieldStorageDefs) map[int]string {
	names := make(map[int]string, len(fieldStorages))
	for _, fs := range fieldStorages {
		names[fs.ID] = fs.MachineName
	}
	return names
}

// resolveFieldStorageLabels - a label per FieldStorageDef, falling back
// to its machine name when the field isn't attached to any content type
// (field_storages has no label of its own).
func (c *Controller) resolveFieldStorageLabels(ctx context.Context, fieldStorages entity.FieldStorageDefs) map[int]string {
	ids := make([]int, len(fieldStorages))
	for i, fs := range fieldStorages {
		ids[i] = fs.ID
	}
	instanceLabels, err := c.contentTypeService.FieldInstanceLabelsByFieldStorageIDs(ctx, ids)
	if err != nil {
		c.LogFailure(err, "error loading field instance labels")
		instanceLabels = nil
	}
	labels := make(map[int]string, len(fieldStorages))
	for _, fs := range fieldStorages {
		if l := instanceLabels[fs.ID]; l != "" {
			labels[fs.ID] = l
		} else {
			labels[fs.ID] = fs.MachineName
		}
	}
	return labels
}

// renderFieldsFragment - like renderFiltersFragment, for the "Fields" card.
func (c *Controller) renderFieldsFragment(ctx context.Context, w io.Writer, view *entity.ViewDisplayDef) error {
	fields, err := c.viewService.EffectiveFieldsByView(ctx, *view)
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
		"view":                     view,
		"fields":                   fields,
		"fieldStorageLabels":       c.resolveFieldStorageLabels(ctx, allFieldStorages),
		"fieldStorageMachineNames": fieldStorageMachineNames(allFieldStorages),
		"fieldStorageTypes":        fieldStorageTypes,
		"fieldBuiltinLabels":       views.FieldBuiltinLabels(ctx),
	}
	addViewModuleCriteria(ctx, data, view)
	return c.templateService.RenderNamed(w, "view_fields_list", data, viewFragmentFiles...)
}

// ViewFieldCreate handles POST /admin/config/views/{id}/fields
// (viewRows.add).
func (c *Controller) ViewFieldCreate(w http.ResponseWriter, r *http.Request) {
	c.fieldRows().add(c, w, r)
}

// fieldRows - the View's fields as viewRows edits them.
func (c *Controller) fieldRows() viewRows[entity.ViewFieldDefs, entity.ViewFieldDef] {
	return viewRows[entity.ViewFieldDefs, entity.ViewFieldDef]{
		what: "fields", pathKey: "field_id",
		canEdit: (*entity.ViewDisplayDef).CanEditFields,
		list:    c.viewService.FieldsByViewID, byID: c.viewService.ViewFieldByID,
		create: c.viewService.CreateViewField, update: c.viewService.UpdateViewField, delete: c.viewService.DeleteViewField,
		override: func(ctx context.Context, id int) error {
			_, err := c.viewService.OverrideFields(ctx, id)
			return err
		},
		revert:   c.viewService.RevertFields,
		render:   c.renderFieldsFragment,
		fromForm: viewFieldFromForm,
		errorCode: func(err error) string {
			switch {
			case errors.Is(err, views.ErrFieldTypeNotDisplayable):
				return "field-type"
			case errors.Is(err, views.ErrFieldLinkTargetUnsupported):
				return "link-target"
			}
			return ""
		},
		target: func(f entity.ViewFieldDef) (*int, string) { return f.FieldStorageID, string(f.BuiltinField) },
		place:  func(f entity.ViewFieldDef) (int, int, int) { return f.ID, f.ViewDisplayID, f.SortOrder },
		placed: func(f entity.ViewFieldDef, order int) entity.ViewFieldDef {
			f.SortOrder = order
			return f
		},
	}
}

// viewFieldFromForm - viewRows.fromForm of the View's fields: the
// target (parseRowTarget) is set when a field is added, then kept -
// changing the displayed field means delete and re-add (see
// entity.ViewRepository.UpdateViewField).
func viewFieldFromForm(r *http.Request, displayID int, old *entity.ViewFieldDef) (entity.ViewFieldDef, string) {
	trimLength, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("trim_length")))
	f := entity.ViewFieldDef{
		ViewDisplayID: displayID,
		Label:         strings.TrimSpace(r.FormValue("label")),
		ShowLabel:     r.FormValue("show_label") == "1",
		ImageStyle:    strings.TrimSpace(r.FormValue("image_style")),
		DateFormat:    strings.TrimSpace(r.FormValue("date_format")),
		TagFormat:     entity.ViewTagFormat(r.FormValue("tag_format")),
		ImageDisplay:  entity.ViewImageDisplay(r.FormValue("image_display")),
		Formatter:     linkFormatterFromForm(r, "formatter"),
		Layout:        entity.ParseFieldLayout(r.FormValue("layout")),
		LinkTo:        entity.ViewFieldLinkTarget(r.FormValue("link_to")),
		CSSClass:      strings.TrimSpace(r.FormValue("css_class")),
		// rewrite_html - genuine HTML (unlike custom_css), sanitized with
		// SanitizeAdmin like HeaderText/FooterText.
		RewriteHTML:        htmlsanitize.SanitizeAdmin(r.FormValue("rewrite_html")),
		TrimLength:         max(trimLength, 0),
		ExcludeFromDisplay: r.FormValue("exclude_from_display") == "1",
		HideInOverflow:     r.FormValue("hide_in_overflow") == "1",
		FallbackFieldKey:   strings.TrimSpace(r.FormValue("fallback_field_key")),
	}
	if old != nil {
		f.ID, f.SortOrder = old.ID, old.SortOrder
		f.FieldStorageID, f.BuiltinField, f.Params = old.FieldStorageID, old.BuiltinField, old.Params
	} else {
		fieldStorageID, builtin, ok := parseRowTarget(r.FormValue("target"))
		if !ok {
			return f, "empty"
		}
		f.FieldStorageID, f.BuiltinField = fieldStorageID, entity.ViewFieldBuiltinField(builtin)
	}
	// A module's field: its settings in this View (its module off - kept).
	if mf, ok := views.ModuleField(string(f.BuiltinField)); ok {
		if f.Params, ok = moduleParamsFromForm(r, mf.Params); !ok {
			return f, "params"
		}
	}
	return f, ""
}

// ViewFieldUpdate handles POST /admin/config/views/{id}/fields/{field_id}
// (viewRows.edit).
func (c *Controller) ViewFieldUpdate(w http.ResponseWriter, r *http.Request) {
	c.fieldRows().edit(c, w, r)
}

// ViewFieldsReorder handles POST /admin/config/views/{id}/fields/reorder
// (viewRows.reorder).
func (c *Controller) ViewFieldsReorder(w http.ResponseWriter, r *http.Request) {
	c.fieldRows().reorder(c, w, r)
}

// ViewFieldDeleteConfirm renders GET
// /admin/config/views/{id}/fields/{field_id}/delete (viewRows.deletePage).
func (c *Controller) ViewFieldDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	c.fieldRows().deletePage(c, w, r, "admin/views/field_delete.html", func(ctx context.Context, label string) (string, string) {
		return i18n.T(ctx, "Delete field: @label", "@label", label), i18n.T(ctx, "Delete field - @label", "@label", label)
	})
}

// ViewFieldDelete handles POST
// /admin/config/views/{id}/fields/{field_id}/delete (viewRows.remove).
func (c *Controller) ViewFieldDelete(w http.ResponseWriter, r *http.Request) {
	c.fieldRows().remove(c, w, r)
}
