package interfaces

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/assets"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// validateSlugFree adds a slug error to errs when the slug typed by hand
// (the automatic one off, see PathAuto) is another node's already (slugs
// are unique across all nodes): the editor sees it at the field instead
// of the save failing. An automatic slug is made free by the server.
func (c *DynamicContentController) validateSlugFree(ctx context.Context, f *form.DynamicForm, errs map[string]string) {
	slug := strings.Trim(f.Slug, "/ ")
	if _, invalid := errs["slug"]; invalid || slug == "" || (f.PathAutoShown && f.PathAuto) {
		return
	}
	taken, err := c.nodeRepo.SlugExistsExcludingNode(ctx, slug, f.NodeID)
	if err != nil {
		c.LogFailure(err, "error checking the slug", "slug", slug, "nodeID", f.NodeID)
		return
	}
	if taken {
		errs["slug"] = i18n.T(ctx, "This URL alias is already used by other content.")
	}
}

// resolveImagesField fills f.FieldValues for an image field with several
// images: the node's own and the ones picked from the media library (the
// field may take them: pickedFiles) kept in the form's order (<field>_keep;
// a form without them keeps the node's all), less the ticked ones
// (<field>_delete), each with its alt text (<field>_alt_<file id>), then
// the new uploads (<field>, several files, when the field takes uploads)
// while the field's MaxValues allows. The value is the file IDs,
// comma-joined, as dynamicValuesFromForm parses them.
func (c *DynamicContentController) resolveImagesField(r *http.Request, contentType string, field entity.FieldInstanceDef, existing any, f *form.DynamicForm) {
	ctx := r.Context()
	fieldName := fmt.Sprintf("field_%d", field.ID)
	uploadError := func(msg string) {
		if f.FieldUploadErrors == nil {
			f.FieldUploadErrors = make(map[int]string)
		}
		f.FieldUploadErrors[field.ID] = msg
	}

	own := map[int]bool{}
	for _, id := range entity.FileIDs(existing) {
		own[id] = true
	}
	deleted := map[int]bool{}
	for _, s := range r.Form[fieldName+"_delete"] {
		if id, err := strconv.Atoi(s); err == nil {
			deleted[id] = true
		}
	}
	kept := r.Form[fieldName+"_keep"]
	if kept == nil {
		for _, id := range entity.FileIDs(existing) {
			kept = append(kept, strconv.Itoa(id))
		}
	}
	// Another's file only picked from the media library, one the field
	// may take.
	var others []int
	for _, s := range kept {
		if id, err := strconv.Atoi(s); err == nil && id > 0 && !own[id] {
			others = append(others, id)
		}
	}
	picked := c.pickedFiles(r, field, others)
	if len(picked) < len(others) {
		uploadError(i18n.T(ctx, "This file can't be used here."))
	}
	var ids []int
	for _, s := range kept {
		id, err := strconv.Atoi(s)
		if err != nil || (!own[id] && !picked[id]) || deleted[id] || slices.Contains(ids, id) {
			continue
		}
		if alt, sent := r.Form[fmt.Sprintf("%s_alt_%d", fieldName, id)]; sent {
			setFileAlt(f, id, strings.TrimSpace(alt[0]))
		}
		ids = append(ids, id)
	}

	if limit := field.MaxValues(); limit > 0 && len(ids) > limit {
		uploadError(i18n.Plural(ctx, limit, "At most @count image.", "At most @count images."))
		ids = ids[:limit]
	}
	var uploads []*multipart.FileHeader
	if r.MultipartForm != nil && field.AllowsUpload() {
		uploads = r.MultipartForm.File[fieldName]
	}
	if len(uploads) > 0 {
		user, hasUser := currentNodeUser(c.BaseController, r)
		if limit := field.MaxValues(); limit > 0 && len(ids)+len(uploads) > limit {
			uploadError(i18n.Plural(ctx, limit, "At most @count image.", "At most @count images."))
			uploads = nil
		}
		for _, header := range uploads {
			if !hasUser {
				c.logger.Error("error saving field upload: no authenticated user", "fieldID", field.ID)
				break
			}
			file, err := header.Open()
			if err != nil {
				c.LogFailure(err, "error opening field upload", "fieldID", field.ID)
				continue
			}
			fileID, err := SaveFieldImageWithPolicy(ctx, &c.fileUploadService, c.fileRepo, file, header, FieldUploadTokens(contentType, user.ID), field)
			if err != nil {
				c.LogFailure(err, "error saving field upload", "fieldID", field.ID)
				uploadError(i18n.ErrorText(ctx, err))
				continue
			}
			ids = append(ids, fileID)
		}
	}

	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	f.FieldValues[field.ID] = strings.Join(parts, ",")
}

// validateUserFields adds an error to errs for a user field naming a user
// who isn't there or hasn't one of the field's roles (UserRoles).
func (c *DynamicContentController) validateUserFields(ctx context.Context, fields entity.FieldInstanceDefs, f *form.DynamicForm, errs map[string]string) {
	for _, field := range fields {
		key := fmt.Sprintf("field_%d", field.ID)
		if field.FieldType != entity.FieldTypeUser || errs[key] != "" {
			continue
		}
		ids := form.ParseIDList(f.FieldValues[field.ID])
		if len(ids) == 0 {
			continue
		}
		users, err := c.userService.ByIDs(ctx, ids)
		if err != nil {
			c.LogFailure(err, "error loading the users of a user field", "fieldID", field.ID, "userIDs", ids)
			errs[key] = i18n.T(ctx, "Illegal value")
			continue
		}
		roles := field.UserRoles()
		for _, id := range ids {
			u, ok := users[id]
			if !ok {
				errs[key] = i18n.T(ctx, "@field: a chosen user no longer exists.", "@field", field.Label)
				break
			}
			if len(roles) > 0 && !slices.Contains(roles, u.RoleID) {
				errs[key] = i18n.T(ctx, "@field: @name can't be chosen here.", "@field", field.Label, "@name", u.Username)
				break
			}
		}
	}
}

// fieldEditorScripts - the addresses of the scripts the formatters showing
// fields on the detail page give their editing (FieldFormatter.
// EditorScript), each once: the content form loads them.
func fieldEditorScripts(fields entity.FieldInstanceDefs) []string {
	var out []string
	for _, field := range fields {
		f, ok := service.FieldFormatterFor(field.FieldType, field.DetailFormatter)
		if !ok || f.EditorScript == "" {
			continue
		}
		if src := assets.ModuleURL(f.Module, f.EditorScript, "js"); !slices.Contains(out, src) {
			out = append(out, src)
		}
	}
	return out
}

// formSection is one block of the create/edit form (node/edit_dynamic.html).
// Group non-nil: a FieldTypeGroup and its members (by FormGroupID,
// independent of the detail page's ParentGroupID), a card of its own.
// Group nil: ungrouped fields, on the card the form is filling.
type formSection struct {
	Group  *entity.FieldInstanceDef
	Fields entity.FieldInstanceDefs
}

// buildFormSections splits fields (sorted by SortOrder) into formSections,
// like buildDisplaySections but without value filtering: the form must show
// empty inputs too. HiddenInForm fields are dropped, groups with no visible
// members are skipped, and group fields themselves render no input.
func buildFormSections(fields entity.FieldInstanceDefs) []formSection {
	membersByGroup := make(map[int]entity.FieldInstanceDefs)
	for _, f := range fields {
		if f.FormGroupID == nil || f.HiddenInForm {
			continue
		}
		membersByGroup[*f.FormGroupID] = append(membersByGroup[*f.FormGroupID], f)
	}

	var sections []formSection
	var ordinary entity.FieldInstanceDefs
	flushOrdinary := func() {
		if len(ordinary) > 0 {
			sections = append(sections, formSection{Fields: ordinary})
			ordinary = nil
		}
	}
	for _, f := range fields {
		if f.FormGroupID != nil {
			continue // rendered as part of its group's section instead
		}
		if f.FieldType == entity.FieldTypeGroup {
			members := membersByGroup[f.ID]
			if len(members) == 0 {
				continue
			}
			flushOrdinary()
			group := f
			sections = append(sections, formSection{Group: &group, Fields: members})
			continue
		}
		if f.HiddenInForm {
			continue
		}
		ordinary = append(ordinary, f)
	}
	flushOrdinary()
	return sections
}

// dynamicFormFromEntity prefills the node-level form fields only; field values
// are prefilled via displayValues (see pageData).
func dynamicFormFromEntity(e *entity.DynamicContent) *form.DynamicForm {
	return &form.DynamicForm{
		NodeMetaFields: form.NodeMetaFromPageSEO(e.Node.PageSEO),
		NodeID:         e.Node.ID,
		Title:          e.Node.Title,
		Slug:           e.Node.Slug,
		PathAuto:       e.Node.PathAuto,
		Promote:        e.Node.Promote,
		Sticky:         e.Node.Sticky,
		ViewRoles:      e.Node.ViewRoles,
		Status:         e.Node.Status,
		OnModeration:   e.Node.OnModeration,
		CommentMode:    string(e.Node.CommentMode),
		PublishOn:      e.Node.PublishOn,
		UnpublishOn:    e.Node.UnpublishOn,
	}
}
