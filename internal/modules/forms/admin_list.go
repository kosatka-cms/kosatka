package forms

import (
	"net/http"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// formsListPath - the admin's list of the forms.
const formsListPath = "/admin/forms"

// FormsList handles GET /admin/forms: every form with its submissions
// count and last time, links to its submissions, CSV and editor, and a
// delete button for a form the user may delete - for those who may view
// submissions.
func (c *WebformController) FormsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := c.adminFormsUser(w, r)
	if !ok {
		return
	}
	items, err := c.webforms.AdminList(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "listing the forms")
		return
	}
	canDelete := make(map[int]bool, len(items))
	for _, it := range items {
		canDelete[it.Webform.Node.ID] = c.AuthService().CanDeleteNode(ctx, &it.Webform.Node, user)
	}
	title := i18n.T(ctx, "Forms")
	data := c.AdminPageData(r)
	data["title"] = title
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title), NoIndex: true}
	data["forms"] = items
	data["canDelete"] = canDelete
	data["canCreate"] = c.AuthService().CanCreateNode(ctx, manifest.ContentType, user)
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	c.RenderOrFail(w, r, "admin/forms.html", data)
}

// adminFormsUser - the signed-in user who may view submissions; a guest is
// sent to log in, another user gets 403 (false: answered).
func (c *WebformController) adminFormsUser(w http.ResponseWriter, r *http.Request) (*modulekit.User, bool) {
	user, _ := c.GetCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return nil, false
	}
	if !c.AuthService().HasPermission(r.Context(), user, manifest.PermViewSubmissions) {
		c.RenderForbidden(w, r)
		return nil, false
	}
	return user, true
}

// deletableForm - the form of the request ({id}) the user may delete
// (false: answered - not found, forbidden).
func (c *WebformController) deletableForm(w http.ResponseWriter, r *http.Request) (*Webform, bool) {
	user, ok := c.adminFormsUser(w, r)
	if !ok {
		return nil, false
	}
	nid, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return nil, false
	}
	wf, err := c.webforms.ByNodeID(r.Context(), nid)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the form", "nodeID", nid)
		return nil, false
	}
	if !c.AuthService().CanDeleteNode(r.Context(), &wf.Node, user) {
		c.RenderForbidden(w, r)
		return nil, false
	}
	return wf, true
}

// DeleteFormConfirm handles GET /admin/forms/{id}/delete: whether to
// delete the form with its submissions.
func (c *WebformController) DeleteFormConfirm(w http.ResponseWriter, r *http.Request) {
	wf, ok := c.deletableForm(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	stats, err := c.webforms.Stats(ctx, *wf)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "the form's submissions")
		return
	}
	title := i18n.T(ctx, "Deleting a form")
	data := c.AdminPageData(r)
	data["title"] = title
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title), NoIndex: true}
	data["question"] = i18n.T(ctx, "Delete the form \"@title\"?", "@title", wf.Node.Title)
	data["form"] = wf
	data["submissions"] = stats.Count
	data["action"] = formsListPath + "/" + strconv.Itoa(wf.Node.ID) + "/delete"
	c.RenderOrFail(w, r, "admin/forms_delete.html", data)
}

// DeleteForm handles POST /admin/forms/{id}/delete: the form goes with
// its submissions and files (the regular content delete); back to the
// list.
func (c *WebformController) DeleteForm(w http.ResponseWriter, r *http.Request) {
	wf, ok := c.deletableForm(w, r)
	if !ok {
		return
	}
	if err := c.nodeDeleter.Delete(r.Context(), wf.Node.ID); err != nil {
		c.LogFailure(err, "deleting a form", "nodeID", wf.Node.ID)
		http.Redirect(w, r, formsListPath+"?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, formsListPath+"?deleted=1", http.StatusSeeOther)
}
