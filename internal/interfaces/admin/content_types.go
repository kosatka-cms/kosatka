package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// contentTypesPath, fieldSetsPath - the lists of the content types and of
// the field sets (ContentTypeDef.Embedded).
const (
	contentTypesPath = "/admin/config/content-types"
	fieldSetsPath    = "/admin/config/field-sets"
)

// typesListPath - the list a type is on.
func typesListPath(embedded bool) string {
	if embedded {
		return fieldSetsPath
	}
	return contentTypesPath
}

// ContentTypeList renders GET /admin/config/content-types - the list of
// admin-defined content types.
func (c *Controller) ContentTypeList(w http.ResponseWriter, r *http.Request) {
	c.renderTypesList(w, r, false)
}

// FieldSetList renders GET /admin/config/field-sets - the field sets
// (ContentTypeDef.Embedded): the content types of the blocks fields'
// items, made and listed like the content types.
func (c *Controller) FieldSetList(w http.ResponseWriter, r *http.Request) {
	c.renderTypesList(w, r, true)
}

// renderTypesList renders the content types, or the field sets.
func (c *Controller) renderTypesList(w http.ResponseWriter, r *http.Request, embedded bool) {
	ctx := r.Context()
	list, title := c.contentTypeService.ContentTypes, i18n.T(ctx, "Content types")
	if embedded {
		list, title = c.contentTypeService.FieldSets, i18n.T(ctx, "Field sets")
	}
	contentTypes, err := list(ctx)
	if err != nil {
		c.LogFailure(err, "error listing content types", "embedded", embedded)
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", title)}
	data["contentTypes"] = contentTypes
	data["fieldSets"] = embedded
	data["listPath"] = typesListPath(embedded)
	data["success"] = r.URL.Query().Get("success") == "1"
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/content-types/index.html", data)
}

// ContentTypeCreate handles POST /admin/config/content-types.
func (c *Controller) ContentTypeCreate(w http.ResponseWriter, r *http.Request) {
	c.createType(w, r, false)
}

// FieldSetCreate handles POST /admin/config/field-sets: a new field set,
// to its fields.
func (c *Controller) FieldSetCreate(w http.ResponseWriter, r *http.Request) {
	c.createType(w, r, true)
}

// createType creates a content type, or a field set (embedded); back to
// its list, a field set to its fields.
func (c *Controller) createType(w http.ResponseWriter, r *http.Request, embedded bool) {
	listPath := typesListPath(embedded)
	ctx := r.Context()

	currentUser, err := c.GetCurrentUser(r)
	if err != nil || currentUser == nil {
		http.Redirect(w, r, listPath+"?error=auth", http.StatusSeeOther)
		return
	}

	machineName := strings.ToLower(strings.TrimSpace(r.FormValue("machine_name")))
	label := strings.TrimSpace(r.FormValue("label"))
	if machineName == "" || label == "" {
		http.Redirect(w, r, listPath+"?error=empty", http.StatusSeeOther)
		return
	}
	desc := strings.TrimSpace(r.FormValue("description"))

	created, err := c.contentTypeService.CreateContentType(ctx, entity.ContentTypeDef{
		MachineName: machineName,
		Label:       label,
		Description: desc,
		Status:      1,
		Embedded:    embedded,
		CreatedBy:   currentUser.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrContentTypeMachineName):
			http.Redirect(w, r, listPath+"?error=machine-name", http.StatusSeeOther)
		case errors.Is(err, service.ErrContentTypeMachineNameReserved):
			http.Redirect(w, r, listPath+"?error=reserved", http.StatusSeeOther)
		case errors.Is(err, service.ErrContentTypeDuplicate):
			http.Redirect(w, r, listPath+"?error=duplicate", http.StatusSeeOther)
		default:
			c.LogFailure(err, "error creating content type", "machineName", machineName, "embedded", embedded)
			http.Redirect(w, r, listPath+"?error=create", http.StatusSeeOther)
		}
		return
	}
	if embedded {
		http.Redirect(w, r, contentTypesPath+"/"+strconv.Itoa(created.ID)+"/fields", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, listPath+"?success=1", http.StatusSeeOther)
}

// ContentTypeEdit handles GET /admin/config/content-types/{id} - the type's
// settings page (label, description, URL pattern, status, default comment
// mode); the form posts to ContentTypeUpdate.
func (c *Controller) ContentTypeEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	contentType, err := c.contentTypeService.ContentTypeByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the content type", "contentTypeID", id)
		return
	}

	data := c.getAdminData(r)
	data["title"] = contentType.Label
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", contentType.Label)}
	data["contentType"] = contentType
	markFieldSetMenu(data, contentType.Embedded)
	data["activeTab"] = "settings"
	// contentCount - the type's content, under the title (-1: not shown).
	data["contentCount"] = -1
	if n, err := c.nodeRepo.CountByType(ctx, contentType.MachineName); err != nil {
		c.LogFailure(err, "counting the content of a type", "type", contentType.MachineName)
	} else {
		data["contentCount"] = n
	}
	data["defaultPathPattern"] = service.DefaultNodePathPattern
	data["pathTokens"] = c.pathTokens(ctx, id)
	data["textFields"], _ = c.searchTypeFields(ctx, contentType.MachineName)
	data["moduleSettings"] = c.LoadContentTypeSettings(ctx, *contentType)
	// viewRoles - the "Who may view" choices: guests and every role.
	data["viewRoles"] = c.visibilityRoleOptions(ctx)
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"

	c.RenderOrFail(w, r, "admin/content-types/edit.html", data)
}

// ContentTypeUpdate handles POST /admin/config/content-types/{id}.
// machine_name is immutable (it is node.Type of every node of this type).
func (c *Controller) ContentTypeUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/content-types", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/content-types?error=invalid", http.StatusSeeOther)
		return
	}
	redirectBase := "/admin/config/content-types/" + strconv.Itoa(id)

	existing, err := c.contentTypeService.ContentTypeByID(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading the content type", "contentTypeID", id)
		http.Redirect(w, r, "/admin/config/content-types?error=update", http.StatusSeeOther)
		return
	}

	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		http.Redirect(w, r, redirectBase+"?error=empty", http.StatusSeeOther)
		return
	}
	desc := strings.TrimSpace(r.FormValue("description"))
	// A field set has its name and description only: its nodes have no
	// page, address or publishing of their own.
	if existing.Embedded {
		set := *existing
		set.Label, set.Description = label, desc
		if _, err := c.contentTypeService.UpdateContentType(ctx, set); err != nil {
			c.LogFailure(err, "error updating a field set", "id", id)
			http.Redirect(w, r, redirectBase+"?error=update", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, redirectBase+"?success=1", http.StatusSeeOther)
		return
	}
	status := 0
	if r.FormValue("status") == "1" {
		status = 1
	}
	hideEmptyComments := r.FormValue("hide_empty_comments") == "1"
	titleDisplay := existing.TitleDisplay
	if v := r.FormValue("title_display"); entity.IsValidTitleDisplay(v) {
		titleDisplay = entity.TitleDisplay(v)
	}
	// altTitleField/redirectField - only the type's own text fields.
	textFields, _ := c.searchTypeFields(ctx, existing.MachineName)
	ownTextField := func(name string) string {
		name = strings.TrimSpace(name)
		for _, f := range textFields {
			if f.MachineName == name {
				return name
			}
		}
		return ""
	}
	altTitleField := ownTextField(r.FormValue("alt_title_field"))
	redirectField := ownTextField(r.FormValue("redirect_field"))
	showSubmitted := r.FormValue("show_submitted") == "1"
	showViewCount := r.FormValue("show_view_count") == "1"
	noPreview := r.FormValue("preview") != "1"
	revisions := r.FormValue("revisions") == "1"
	// How many versions: empty or not a positive number - all.
	revisionsKeep, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("revisions_keep")))
	revisionsKeep = max(revisionsKeep, 0)
	defaultPromote := r.FormValue("default_promote") == "1"
	defaultSticky := r.FormValue("default_sticky") == "1"
	// view_roles - entity.ContentTypeDef.ViewRoles: none checked -
	// everyone.
	var viewRoles []string
	if v := parseFormRoles(r, "view_roles"); v != "" {
		viewRoles = strings.Split(v, ",")
	}

	commentMode := entity.CommentMode(r.FormValue("default_comment_mode"))
	if !entity.IsValidCommentMode(string(commentMode)) {
		commentMode = existing.DefaultCommentMode
	}
	publishing := entity.NodePublishing(r.FormValue("default_publishing"))
	if !publishing.Valid() {
		publishing = existing.DefaultPublishing
	}

	pathPattern := strings.TrimSpace(r.FormValue("path_pattern"))
	fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, id)
	if err != nil {
		c.LogFailure(err, "error loading the type's fields for its URL pattern", "id", id)
		http.Redirect(w, r, redirectBase+"?error=update", http.StatusSeeOther)
		return
	}
	if service.ValidateNodePathPattern(pathPattern, fields) != nil {
		http.Redirect(w, r, redirectBase+"?error=path-pattern", http.StatusSeeOther)
		return
	}

	_, err = c.contentTypeService.UpdateContentType(ctx, entity.ContentTypeDef{
		ID:                 id,
		MachineName:        existing.MachineName,
		Label:              label,
		PathPattern:        pathPattern,
		Description:        desc,
		Status:             status,
		DefaultCommentMode: commentMode,
		HideEmptyComments:  hideEmptyComments,
		ViewRoles:          viewRoles,
		DefaultPromote:     defaultPromote,
		DefaultSticky:      defaultSticky,
		DefaultPublishing:  publishing,
		TitleDisplay:       titleDisplay,
		AltTitleField:      altTitleField,
		RedirectField:      redirectField,
		ShowSubmitted:      showSubmitted,
		ShowViewCount:      showViewCount,
		Revisions:          revisions,
		RevisionsKeep:      revisionsKeep,
		NoPreview:          noPreview,
	})
	if err != nil {
		c.LogFailure(err, "error updating content type", "id", id)
		http.Redirect(w, r, redirectBase+"?error=update", http.StatusSeeOther)
		return
	}
	// The modules' sections of the form (AddContentTypeSettings).
	if err := interfaces.SaveContentTypeSettings(ctx, entity.ContentTypeDef{ID: id, MachineName: existing.MachineName}, r); err != nil {
		c.LogFailure(err, "error saving the content type's module settings", "id", id)
		http.Redirect(w, r, redirectBase+"?error=update", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, redirectBase+"?success=1", http.StatusSeeOther)
}

// ContentTypeDelete handles POST /admin/config/content-types/{id}/delete.
func (c *Controller) ContentTypeDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/content-types", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/content-types?error=invalid", http.StatusSeeOther)
		return
	}

	// Back to its list: the content types' or the field sets'.
	listPath := contentTypesPath
	if ct, err := c.contentTypeService.ContentTypeByID(ctx, id); err == nil {
		listPath = typesListPath(ct.Embedded)
	}
	if err := c.contentTypeService.DeleteContentType(ctx, id); err != nil {
		if errors.Is(err, service.ErrContentTypeHasNodes) {
			http.Redirect(w, r, listPath+"?error=has-nodes", http.StatusSeeOther)
			return
		}
		c.LogFailure(err, "error deleting content type", "id", id)
		http.Redirect(w, r, listPath+"?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, listPath+"?deleted=1", http.StatusSeeOther)
}

// nodePathTokens - the node's own tokens of every URL pattern.
var nodePathTokens = []string{"[node:type]", "[node:title]", "[node:created:Y]", "[node:created:m]", "[node:created:d]"}

// pathTokens - the tokens a type's URL pattern can take: the node's and
// its taxonomy fields' (service.ValidateNodePathPattern), for the hints.
func (c *Controller) pathTokens(ctx context.Context, contentTypeID int) []string {
	fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, contentTypeID)
	if err != nil {
		c.LogFailure(err, "error loading the type's fields for its URL pattern", "id", contentTypeID)
		return nodePathTokens
	}
	tokens := append([]string{}, nodePathTokens...)
	for _, f := range fields {
		token := "[node:field:" + f.MachineName + "]"
		if service.ValidateNodePathPattern(token, entity.FieldInstanceDefs{f}) == nil {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// markFieldSetMenu - a field set's pages (they are under
// /admin/config/content-types/{id}, as a type's) are "Field sets"' in
// the admin's sidebar and breadcrumbs.
func markFieldSetMenu(data map[string]any, embedded bool) {
	if !embedded {
		return
	}
	if menu, ok := data["adminMenu"].([]entity.AdminMenuSection); ok {
		if section, item, ok := entity.AdminMenuCurrent(menu, fieldSetsPath); ok {
			data["adminSection"], data["adminItem"] = section, item
		}
	}
}
