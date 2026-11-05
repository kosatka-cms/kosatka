package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// contentTypeFieldsPageContext loads the ContentTypeDef and its fields
// (SortOrder-ordered) - shared by the content type tabs (see
// admin/content-types/tabs.html): settings, fields, detail display and list
// display.
func (c *Controller) contentTypeFieldsPageContext(ctx context.Context, id int) (*entity.ContentTypeDef, entity.FieldInstanceDefs, error) {
	contentType, err := c.contentTypeService.ContentTypeByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return contentType, fields, nil
}

// ContentTypeFieldList renders GET /admin/config/content-types/{id}/fields -
// the "Fields" tab.
func (c *Controller) ContentTypeFieldList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}

	contentType, fields, err := c.contentTypeFieldsPageContext(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the content type and its fields", "contentTypeID", id)
		return
	}

	// availableFieldStorages/fieldStorageUsedBy - for the "Add existing field"
	// picker: every reusable field not yet attached to this type, plus which
	// other types use it (its config is shared, see UpdateField).
	availableFieldStorages, fieldStorageUsedBy, err := c.availableFieldStoragesFor(ctx, id, fields)
	if err != nil {
		c.LogFailure(err, "error building available field storages list", "contentTypeID", id)
	}

	// vocabularyNames: id -> Name, so taxonomy fields in the "Widget" column
	// show and link to their vocabulary.
	vocabularies, err := c.taxonomyService.Vocabularies(ctx)
	if err != nil {
		c.LogFailure(err, "error listing vocabularies for content type fields page")
	}
	vocabularyNames := make(map[int]string, len(vocabularies))
	for _, v := range vocabularies {
		vocabularyNames[v.ID] = v.Name
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Fields: @label", "@label", contentType.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Fields @label - Admin panel", "@label", contentType.Label)}
	data["contentType"] = contentType
	markFieldSetMenu(data, contentType.Embedded)
	data["activeTab"] = "fields"
	data["fields"] = fields
	// orderedFields: every non-group field plus groups scoped UsableInForm,
	// SortOrder-sorted with members nested after their group (see
	// nestGroupMembersForAdminForm); detail-only groups are managed on the
	// detail display tab. .fields (group selects, create form) stays
	// unfiltered.
	formEligible := make(entity.FieldInstanceDefs, 0, len(fields))
	for _, f := range fields {
		if f.FieldType == entity.FieldTypeGroup && !f.UsableInForm {
			continue
		}
		formEligible = append(formEligible, f)
	}
	data["orderedFields"] = nestGroupMembersForAdminForm(formEligible.SortBySortOrder())
	// A field set's fields: no blocks in it (ErrBlocksInFieldSet).
	fieldTypes := contentTypeFieldTypes
	if contentType.Embedded {
		fieldTypes = slices.DeleteFunc(slices.Clone(fieldTypes), func(t entity.ContentTypeFieldType) bool { return t == entity.FieldTypeBlocks })
		availableFieldStorages = slices.DeleteFunc(availableFieldStorages, func(s entity.FieldStorageDef) bool { return s.FieldType == entity.FieldTypeBlocks })
	}
	data["fieldTypes"] = fieldTypes
	data["vocabularyNames"] = vocabularyNames
	data["fieldStorages"] = availableFieldStorages
	data["fieldStorageUsedBy"] = fieldStorageUsedBy
	data["success"] = r.URL.Query().Get("success") == "1"
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/content-types/fields.html", data)
}

// ContentTypeFieldsDetailDisplay renders GET
// /admin/config/content-types/{id}/fields/display-detail - the "Detail
// page" tab; its form posts to ContentTypeFieldsDetailDisplayUpdate.
func (c *Controller) ContentTypeFieldsDetailDisplay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}

	contentType, fields, err := c.contentTypeFieldsPageContext(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the content type and its fields", "contentTypeID", id)
		return
	}

	// Image styles for the per-image-field style select.
	imageStyles, err := c.imageStyleService.ImageStyles(ctx)
	if err != nil {
		c.LogFailure(err, "error listing image styles for field detail display")
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Display: on the full page - @label", "@label", contentType.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Full page display @label - Admin panel", "@label", contentType.Label)}
	data["contentType"] = contentType
	markFieldSetMenu(data, contentType.Embedded)
	data["activeTab"] = "detail-display"
	// linkFormatters - the modules' formatters a link field may show with.
	data["linkFormatters"] = service.FieldFormatters(entity.FieldTypeLink)
	// fields: for the "Parent group" select (every FieldTypeGroup field of
	// this type).
	data["fields"] = fields
	// displayFields: every non-group field plus only groups scoped
	// UsableInDetail - a form-only group can never get members here (the
	// parent group select lists only UsableInDetail groups), so it isn't shown.
	detailEligible := make(entity.FieldInstanceDefs, 0, len(fields))
	for _, f := range fields {
		if f.FieldType == entity.FieldTypeGroup && !f.UsableInDetail {
			continue
		}
		detailEligible = append(detailEligible, f)
	}
	data["displayFields"] = nestGroupMembersForAdminDisplay(detailEligible.SortByDisplayOrder())
	data["imageStyles"] = imageStyles
	data["success"] = r.URL.Query().Get("success") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/content-types/detail_display.html", data)
}

// availableFieldStoragesFor builds the "Add existing field" options: every
// FieldStorageDef not attached to contentTypeID, plus a per-storage label
// naming the other content types using it, since its config is shared
// (see UpdateField).
func (c *Controller) availableFieldStoragesFor(ctx context.Context, contentTypeID int, currentFields entity.FieldInstanceDefs) (entity.FieldStorageDefs, map[int]string, error) {
	allStorages, err := c.contentTypeService.FieldStorages(ctx)
	if err != nil {
		return nil, nil, err
	}

	attached := make(map[int]bool, len(currentFields))
	for _, f := range currentFields {
		attached[f.FieldStorageID] = true
	}

	allContentTypes, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		return nil, nil, err
	}
	sets, err := c.contentTypeService.FieldSets(ctx)
	if err != nil {
		return nil, nil, err
	}
	allContentTypes = append(allContentTypes, sets...)
	usedByLabels := make(map[int][]string)
	for _, ct := range allContentTypes {
		if ct.ID == contentTypeID {
			continue
		}
		ctFields, err := c.contentTypeService.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			c.LogFailure(err, "error loading fields for used-by label", "contentTypeID", ct.ID)
			continue
		}
		for _, f := range ctFields {
			usedByLabels[f.FieldStorageID] = append(usedByLabels[f.FieldStorageID], ct.Label)
		}
	}

	usedBy := make(map[int]string, len(usedByLabels))
	for storageID, labels := range usedByLabels {
		usedBy[storageID] = i18n.T(ctx, "already on: @types", "@types", strings.Join(labels, ", "))
	}

	available := make(entity.FieldStorageDefs, 0, len(allStorages))
	for _, s := range allStorages {
		if !attached[s.ID] {
			available = append(available, s)
		}
	}
	return available, usedBy, nil
}

// nestGroupMembersForAdminDisplay reorders a DisplayOrder-sorted field
// list so each FieldTypeGroup field's members immediately follow it.
// Presentation only, for the detail display admin table: the public page
// already renders members at the group's position. Contiguous member rows
// let the table's drag-and-drop move a whole group.
func nestGroupMembersForAdminDisplay(sorted entity.FieldInstanceDefs) entity.FieldInstanceDefs {
	membersByGroup := make(map[int]entity.FieldInstanceDefs)
	for _, f := range sorted {
		if f.ParentGroupID != nil {
			membersByGroup[*f.ParentGroupID] = append(membersByGroup[*f.ParentGroupID], f)
		}
	}
	result := make(entity.FieldInstanceDefs, 0, len(sorted))
	for _, f := range sorted {
		if f.ParentGroupID != nil {
			continue
		}
		result = append(result, f)
		if f.FieldType == entity.FieldTypeGroup {
			result = append(result, membersByGroup[f.ID]...)
		}
	}
	return result
}

// nestGroupMembersForAdminForm - like nestGroupMembersForAdminDisplay, but
// for the "Fields" tab's SortOrder/FormGroupID axis, so its drag-and-drop
// can move a whole form group.
func nestGroupMembersForAdminForm(sorted entity.FieldInstanceDefs) entity.FieldInstanceDefs {
	membersByGroup := make(map[int]entity.FieldInstanceDefs)
	for _, f := range sorted {
		if f.FormGroupID != nil {
			membersByGroup[*f.FormGroupID] = append(membersByGroup[*f.FormGroupID], f)
		}
	}
	result := make(entity.FieldInstanceDefs, 0, len(sorted))
	for _, f := range sorted {
		if f.FormGroupID != nil {
			continue
		}
		result = append(result, f)
		if f.FieldType == entity.FieldTypeGroup {
			result = append(result, membersByGroup[f.ID]...)
		}
	}
	return result
}

// ContentTypeFieldAttach handles POST
// /admin/config/content-types/{id}/fields/attach - the "Add existing
// field" form: attaches an existing FieldStorageDef to this content type
// (see AttachExistingField). label/required are per-type instance
// settings; machine_name/field_type/config come from the storage.
func (c *Controller) ContentTypeFieldAttach(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	redirectBase := "/admin/config/content-types/" + strconv.Itoa(id) + "/fields"
	if r.Method != http.MethodPost {
		http.Redirect(w, r, redirectBase, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	fieldStorageID, err := strconv.Atoi(r.FormValue("field_storage_id"))
	if err != nil || fieldStorageID <= 0 {
		http.Redirect(w, r, redirectBase+"?error=invalid#fields-attach", http.StatusSeeOther)
		return
	}
	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		http.Redirect(w, r, redirectBase+"?error=empty#fields-attach", http.StatusSeeOther)
		return
	}
	required := r.FormValue("required") == "1"
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))

	_, err = c.contentTypeService.AttachExistingField(ctx, id, fieldStorageID, label, required, sortOrder)
	if err != nil {
		if errors.Is(err, service.ErrFieldAlreadyAttached) {
			http.Redirect(w, r, redirectBase+"?error=already-attached#fields-attach", http.StatusSeeOther)
			return
		}
		if errors.Is(err, service.ErrBlocksInFieldSet) {
			http.Redirect(w, r, redirectBase+"?error=blocks-in-field-set#fields-attach", http.StatusSeeOther)
			return
		}
		c.LogFailure(err, "error attaching existing field", "contentTypeID", id, "fieldStorageID", fieldStorageID)
		http.Redirect(w, r, redirectBase+"?error=create#fields-attach", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, redirectBase+"?success=1", http.StatusSeeOther)
}

// ContentTypeFieldsSortOrderUpdate handles POST
// /admin/config/content-types/{id}/fields/reorder - the "Fields" tab's
// drag-and-drop: position in "order" becomes the field's SortOrder (form
// order) and "form_group_id_<id>" its FormGroupID; one
// UpdateFieldSortOrder call per field.
func (c *Controller) ContentTypeFieldsSortOrderUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	redirectBase := "/admin/config/content-types/" + strconv.Itoa(id) + "/fields"
	if r.Method != http.MethodPost {
		http.Redirect(w, r, redirectBase, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	order := strings.Split(r.FormValue("order"), ",")
	if len(order) == 0 || strings.TrimSpace(r.FormValue("order")) == "" {
		http.Redirect(w, r, redirectBase+"?error=empty-order", http.StatusSeeOther)
		return
	}

	own, err := c.typeFields(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading the type's fields", "contentTypeID", id)
		http.Redirect(w, r, redirectBase+"?error=display-update", http.StatusSeeOther)
		return
	}
	for i, idStr := range order {
		fieldID, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			continue
		}
		formGroupID := parentGroupIDFromFormKey(r, fmt.Sprintf("form_group_id_%d", fieldID))
		if _, ok := own[fieldID]; !ok || !ownGroup(own, formGroupID) {
			http.Redirect(w, r, redirectBase+"?error=invalid", http.StatusSeeOther)
			return
		}
		if err := c.contentTypeService.UpdateFieldSortOrder(ctx, fieldID, i, formGroupID); err != nil {
			c.LogFailure(err, "error updating content type field sort order", "fieldID", fieldID)
			http.Redirect(w, r, redirectBase+"?error=display-update", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, redirectBase+"?success=1", http.StatusSeeOther)
}

// ContentTypeFieldsDetailDisplayUpdate handles POST
// /admin/config/content-types/{id}/fields/display-detail - the detail tab's
// bulk form: "order" (CSV of field IDs in final order, becoming
// DisplayOrder), per-field checkboxes (show_on_detail_<id>/show_label_<id>/
// renders_in_body_<id>), how it shows (detail_image_style_<id>,
// detail_formatter_<id>, detail_layout_<id>) and the parent group select
// (parent_group_id_<id>). One UpdateFieldDetailDisplay call per field.
func (c *Controller) ContentTypeFieldsDetailDisplayUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	redirectBase := "/admin/config/content-types/" + strconv.Itoa(id) + "/fields/display-detail"
	if r.Method != http.MethodPost {
		http.Redirect(w, r, redirectBase, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	order := strings.Split(r.FormValue("order"), ",")
	if len(order) == 0 || strings.TrimSpace(r.FormValue("order")) == "" {
		http.Redirect(w, r, redirectBase+"?error=empty-order", http.StatusSeeOther)
		return
	}

	own, err := c.typeFields(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading the type's fields", "contentTypeID", id)
		http.Redirect(w, r, redirectBase+"?error=display-update", http.StatusSeeOther)
		return
	}
	for i, idStr := range order {
		fieldID, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			continue
		}
		d := entity.FieldDetailDisplay{
			DisplayOrder:   i,
			ShowOnDetail:   r.FormValue(fmt.Sprintf("show_on_detail_%d", fieldID)) == "1",
			ShowLabel:      r.FormValue(fmt.Sprintf("show_label_%d", fieldID)) == "1",
			RendersInBody:  r.FormValue(fmt.Sprintf("renders_in_body_%d", fieldID)) == "1",
			ImageStyle:     r.FormValue(fmt.Sprintf("detail_image_style_%d", fieldID)),
			ListImageStyle: r.FormValue(fmt.Sprintf("list_image_style_%d", fieldID)),
			Formatter:      r.FormValue(fmt.Sprintf("detail_formatter_%d", fieldID)),
			Layout:         entity.ParseFieldLayout(r.FormValue(fmt.Sprintf("detail_layout_%d", fieldID))),
			ParentGroupID:  parentGroupIDFromFormKey(r, fmt.Sprintf("parent_group_id_%d", fieldID)),
		}
		field, ok := own[fieldID]
		if !ok || !ownGroup(own, d.ParentGroupID) {
			http.Redirect(w, r, redirectBase+"?error=invalid", http.StatusSeeOther)
			return
		}
		// A formatter only for a field of its type (unknown - links).
		if _, ok := service.FieldFormatterFor(field.FieldType, d.Formatter); !ok {
			d.Formatter = ""
		}
		if err := c.contentTypeService.UpdateFieldDetailDisplay(ctx, fieldID, d); err != nil {
			c.LogFailure(err, "error updating content type field detail display", "fieldID", fieldID)
			http.Redirect(w, r, redirectBase+"?error=display-update", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, redirectBase+"?success=1", http.StatusSeeOther)
}
