package interfaces

import (
	"mime/multipart"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
)

// A content item's form previewed (its Preview button): the form sent to
// a page of its own (formaction, a new tab), the item it describes
// rendered as its page would be - nothing saved, no file kept. The form
// stays as it was in its own tab.

// UnsavedPreview - the strip over a previewed item's page (the page's
// data "unsavedPreview", html.html's "unsaved_preview"): Errors - what
// saving would refuse, to correct in the form; FilesLeft - files chosen
// in the form, shown once saved (a preview doesn't store them).
type UnsavedPreview struct {
	Errors    []string
	FilesLeft bool
}

// FormPreviewPath - where the form of a new item of type (nodeID 0) or
// of the item nodeID sends its preview (the Preview button's formaction).
func FormPreviewPath(contentType string, nodeID int) string {
	if nodeID > 0 {
		return "/node/" + strconv.Itoa(nodeID) + "/edit/preview"
	}
	return "/create/" + contentType + "/preview"
}

// CreatePreview handles POST /create/{type}/preview: the new item of
// the type its form describes (CreateSubmit's), unsaved. A type without
// preview (ContentTypeDef.NoPreview) - not found.
func (c *DynamicContentController) CreatePreview(w http.ResponseWriter, r *http.Request) {
	cf, ok := c.createForm(w, r)
	if !ok {
		return
	}
	if cf.def.NoPreview {
		c.RenderNotFound(w, r)
		return
	}
	user, ok := currentNodeUser(c.BaseController, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	f, preview, ok := c.previewForm(w, r, cf, 0, nil)
	if !ok {
		return
	}
	e := cf.toEntity(f)
	e.Node.Type = entity.ContentType(cf.def.MachineName)
	e.Node.CreatedBy, e.Node.CreatedAt = *user, time.Now()
	if !entity.IsValidCommentMode(string(e.Node.CommentMode)) {
		e.Node.CommentMode = cf.def.DefaultCommentMode
	}
	c.renderPreview(w, r, cf, e, user, preview)
}

// EditPreview handles POST /node/{id}/edit/preview: the item as its edit
// form would save it (Save's, applyEdit), unsaved; what the form doesn't
// touch (its address, its counts, its last change) as saved. A type
// without preview - not found.
func (c *DynamicContentController) EditPreview(w http.ResponseWriter, r *http.Request) {
	node, ok := middleware.GetNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	cf, ok := c.contentForm(w, r, node.Type.ToString())
	if !ok {
		return
	}
	if cf.def.NoPreview {
		c.RenderNotFound(w, r)
		return
	}
	user, ok := currentNodeUser(c.BaseController, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	existing, err := cf.store.ByNodeID(r.Context(), node.ID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the node to preview", "nodeID", node.ID)
		return
	}
	f, preview, ok := c.previewForm(w, r, cf, node.ID, existing)
	if !ok {
		return
	}
	e := cf.toEntity(f)
	applyEdit(e, existing, f)
	e.Node.Type, e.Node.Slug, e.Node.PathAuto = existing.Node.Type, existing.Node.Slug, existing.Node.PathAuto
	e.Node.Counter, e.Node.UpdatedAt = existing.Node.Counter, existing.Node.UpdatedAt
	e.Node.ParentID, e.Node.Langcode = existing.Node.ParentID, existing.Node.Langcode
	if e.Node.CreatedBy.ID != existing.Node.CreatedBy.ID {
		e.Node.CreatedBy = c.previewAuthor(r, e.Node.CreatedBy.ID)
	}
	c.renderPreview(w, r, cf, e, user, preview)
}

// previewForm reads the previewed form (as parseContentForm, nodeID and
// existing - its): the files chosen in it dropped first - a preview
// stores none (FilesLeft) - and what saving would refuse noted
// (validateContentForm); false - answered (a bad form).
func (c *DynamicContentController) previewForm(w http.ResponseWriter, r *http.Request, cf contentForm, nodeID int, existing *entity.DynamicContent) (*form.DynamicForm, *UnsavedPreview, bool) {
	if err := r.ParseMultipartForm(maxNodeFormSize); err != nil {
		c.LogFailure(err, "error parsing the previewed node form")
		c.RenderBadForm(w, r)
		return nil, nil, false
	}
	preview := &UnsavedPreview{FilesLeft: dropFiles(r.MultipartForm)}
	f, err := c.parseContentForm(r, cf, nodeID, existing)
	if err != nil {
		c.LogFailure(err, "error parsing the previewed node form", "type", cf.def.MachineName)
		c.RenderBadForm(w, r)
		return nil, nil, false
	}
	errs := c.validateContentForm(r.Context(), cf, f)
	for _, msg := range errs {
		if !slices.Contains(preview.Errors, msg) {
			preview.Errors = append(preview.Errors, msg)
		}
	}
	slices.Sort(preview.Errors)
	return f, preview, true
}

// dropFiles removes the files of a multipart form (their temporary
// copies too); whether one was chosen.
func dropFiles(m *multipart.Form) bool {
	if m == nil {
		return false
	}
	chosen := false
	for _, fhs := range m.File {
		for _, fh := range fhs {
			if fh.Filename != "" {
				chosen = true
			}
		}
	}
	_ = m.RemoveAll()
	m.File = map[string][]*multipart.FileHeader{}
	return chosen
}

// previewAuthor - the author the form gives (who may set it), by ID; only
// the ID when the user fails to load (logged).
func (c *DynamicContentController) previewAuthor(r *http.Request, id int) entity.User {
	u, err := c.userService.ByID(r.Context(), id)
	if err != nil || u == nil {
		if err != nil {
			c.LogFailure(err, "loading the previewed item's author", "userID", id)
		}
		return entity.User{ID: id}
	}
	return *u
}

// renderPreview renders the previewed item e (renderItem): kept by no
// cache, not indexed.
func (c *DynamicContentController) renderPreview(w http.ResponseWriter, r *http.Request, cf contentForm, e *entity.DynamicContent, user *entity.User, preview *UnsavedPreview) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	def := cf.def
	c.renderItem(w, r, &def, cf.fields, e, user, preview)
}
