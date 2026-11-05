package admin

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// contentTypeFieldTypes - the available field types; used both to validate
// admin input and to render the field type select.
var contentTypeFieldTypes = entity.FieldTypes

func isValidFieldType(ft string) bool {
	return slices.Contains(contentTypeFieldTypes, entity.ContentTypeFieldType(ft))
}

// fieldWidgets - the FieldTypeText widget options. WidgetSafeHTML/
// WidgetWYSIWYG are no longer offered: the editor and rendering are decided
// by the allowed text formats (TextFormatDef.HasRichEditor, RendersInBody).
// The constants remain for already-saved fields (isValidWidget,
// dynamicValuesFromForm) and Views' synthetic fields.
var fieldWidgets = []entity.FieldWidget{
	entity.WidgetInput,
	entity.WidgetTextarea,
}

func isValidWidget(w string) bool {
	return slices.Contains(fieldWidgets, entity.FieldWidget(w))
}

// listWidgets - the FieldTypeList widget options (a separate allow-list
// from fieldWidgets).
var listWidgets = []entity.FieldWidget{
	entity.WidgetSelect,
	entity.WidgetRadio,
	entity.WidgetCheckboxes,
}

func isValidListWidget(w string) bool {
	return slices.Contains(listWidgets, entity.FieldWidget(w))
}

// taxonomyWidgets - the FieldTypeTaxonomy widget options (see
// entity.WidgetAutocomplete). Shown regardless of Multiple(); widget and
// cardinality mismatches are rejected on submit (ContentTypeFieldUpdate).
var taxonomyWidgets = []entity.FieldWidget{
	entity.WidgetAutocomplete,
	entity.WidgetSelect,
	entity.WidgetRadio,
	entity.WidgetCheckboxes,
}

func isValidTaxonomyWidget(w string) bool {
	return slices.Contains(taxonomyWidgets, entity.FieldWidget(w))
}

// fieldConfigFromForm builds FieldStorageDef.Config from form values -
// settings shared by every content type the field is attached to: the
// field type's (adminFieldType.storage) and, the create form's, "multiple"
// (changed later on the field's own page). Per-type widget settings are
// in fieldWidgetConfigFromForm.
func fieldConfigFromForm(r *http.Request, fieldType entity.ContentTypeFieldType) map[string]any {
	config := map[string]any{}
	if fieldType.AllowsMultiple() && r.FormValue("multiple") == "1" {
		config["multiple"] = true
	}
	if storage := adminFieldTypes[fieldType].storage; storage != nil {
		storage(r, config)
	}
	return config
}

// fieldWidgetConfigFromForm builds FieldInstanceDef's own config
// (WidgetConfig) - per-content-type form display settings, not shared with
// other types using the same FieldStorageDef (see
// entity.FieldInstanceDef.WidgetConfig): the field type's
// (adminFieldType.instance).
func fieldWidgetConfigFromForm(r *http.Request, fieldType entity.ContentTypeFieldType) map[string]any {
	config := map[string]any{}
	if instance := adminFieldTypes[fieldType].instance; instance != nil {
		instance(r, config)
	}
	return config
}

// parseResolutionForm parses the "WIDTHxHEIGHT" resolution input
// (e.g. "1920x1080"). Returns 0,0 on anything that doesn't parse cleanly.
func parseResolutionForm(raw string) (width, height int) {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(raw)), "x", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}

// parseAllowedValuesForm parses the "allowed_values" textarea - one
// "value|Label" pair per line (empty lines skipped; no "|" means the line
// is both value and label) - into the []any (JSON array, order-preserving)
// shape entity.FieldInstanceDef.AllowedValues expects.
func parseAllowedValuesForm(raw string) []any {
	var out []any
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		value, label := line, line
		if before, after, ok := strings.Cut(line, "|"); ok {
			value = strings.TrimSpace(before)
			label = strings.TrimSpace(after)
			if label == "" {
				label = value
			}
		}
		if value == "" {
			continue
		}
		out = append(out, map[string]any{"value": value, "label": label})
	}
	return out
}

// parentGroupIDFromFormKey reads the given form field: "0"/empty/missing
// (the "no group" option) means nil, any other positive int is the group's
// FieldInstanceDef.ID. Used by both bulk drag-and-drop actions:
// ContentTypeFieldsDetailDisplayUpdate ("parent_group_id_<fieldID>") and
// ContentTypeFieldsSortOrderUpdate ("form_group_id_<fieldID>").
func parentGroupIDFromFormKey(r *http.Request, key string) *int {
	id, err := strconv.Atoi(r.FormValue(key))
	if err != nil || id <= 0 {
		return nil
	}
	return &id
}

// formGroupIDFromForm reads the unkeyed "form_group_id" field like
// parentGroupIDFromFormKey; used only by ContentTypeFieldCreate to set a
// new field's initial FormGroupID.
func formGroupIDFromForm(r *http.Request) *int {
	return parentGroupIDFromFormKey(r, "form_group_id")
}

// linkFormatterFromForm - the link field formatter named by the form's
// input name: one of the enabled modules' (service.FieldFormatters), else
// "" - links.
func linkFormatterFromForm(r *http.Request, name string) string {
	f, ok := service.FieldFormatterFor(entity.FieldTypeLink, strings.TrimSpace(r.FormValue(name)))
	if !ok {
		return ""
	}
	return f.Name
}

// fieldDateFormatFromForm - the display format of a date field (a date
// format machine name, "" for the default); other field types keep theirs.
func fieldDateFormatFromForm(r *http.Request, existing entity.FieldInstanceDef) string {
	if existing.FieldType != entity.FieldTypeDate {
		return existing.DateFormat
	}
	return strings.TrimSpace(r.FormValue("date_format"))
}
