package forms

import (
	"net/http"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// FormPreview handles POST /create/form/preview and POST
// /node/{id}/edit/preview (modulekit.NodeFormPreviewer, the editor's
// Preview button): the form as its editor would save it, on its page as
// visitors see it - its questions, their conditions and prefills working
// - with sending off; nothing saved. What saving would refuse is in the
// strip over the page.
func (c *WebformController) FormPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := c.GetCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	f := &WebformForm{}
	f.ParseWebformForm(r)
	var existing *Webform
	if r.PathValue("id") != "" {
		node, ok := modulekit.RequestNode(r)
		if !ok {
			c.RenderNotFound(w, r)
			return
		}
		var err error
		if existing, err = c.webforms.ByNodeID(ctx, node.ID); err != nil {
			c.RenderLoadFailure(w, r, err, "loading the form to preview", "nodeID", node.ID)
			return
		}
		f.NodeID = node.ID
	}
	preview := &modulekit.UnsavedPreview{}
	for _, msg := range f.Validate(ctx) {
		if !slices.Contains(preview.Errors, msg) {
			preview.Errors = append(preview.Errors, msg)
		}
	}
	slices.Sort(preview.Errors)

	wf := c.webformFromForm(r, f, user, existing)
	wf.Node.Type = manifest.ContentType
	var saved WebformElements
	if existing != nil {
		saved = existing.Elements
	}
	previewElements(wf.Elements, saved)

	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	data := c.GetCommonData(r)
	c.formData(ctx, data, &wf, user, nil, nil)
	data["item"] = &wf
	data["unsavedPreview"] = preview
	data["pageSEO"] = modulekit.PageSEO{Title: wf.Node.Title, Description: modulekit.TruncateMeta(wf.Description, 160)}
	c.SetNodeRefTrail(ctx, data, wf.Node)
	c.RenderOrFail(w, r, "webform/detail.html", data)
}

// previewElements readies the editor's elements for the page as storing
// them would (prepareWebformElements; saved - the form's saved ones) and
// numbers the new ones after every other: their inputs and the
// conditions on them work on the page.
func previewElements(elements, saved WebformElements) {
	prepareWebformElements(elements, saved)
	next := 0
	for _, e := range slices.Concat(elements, saved) {
		next = max(next, e.ID)
	}
	for i := range elements {
		if elements[i].ID == 0 {
			next++
			elements[i].ID = next
		}
	}
}
