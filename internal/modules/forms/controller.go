package forms

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"

	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// WebformController - the forms module's pages: a form (node of type
// "form") is created, edited and deleted like other content, shown at its
// address and submitted there.
type WebformController struct {
	*modulekit.BaseController
	webforms    *WebformService
	nodes       modulekit.NodesService
	nodeDeleter modulekit.NodeDeleter
}

// NewWebformController creates a WebformController on base.
func NewWebformController(base *modulekit.BaseController, webforms *WebformService, nodes modulekit.NodesService, nodeDeleter modulekit.NodeDeleter) *WebformController {
	base.SetLogger(base.Logger().With("component", "webform.handler"))
	return &WebformController{BaseController: base, webforms: webforms, nodes: nodes, nodeDeleter: nodeDeleter}
}

// editorData - the editor page's data: a service page (no sidebar).
func (c *WebformController) editorData(r *http.Request, f *WebformForm, existing *Webform, errs map[string]string) map[string]any {
	ctx := r.Context()
	data := c.GetCommonData(r)
	data["servicePage"] = true
	data["form"] = f
	data["errors"] = errs
	data["elementTypes"] = WebformElementTypes
	if existing != nil {
		data["webform"] = existing
		data["editNode"] = existing
		data["title"] = i18n.T(ctx, "Edit form")
		c.SetNodeSubpageTrail(ctx, data, existing.Node, i18n.T(ctx, "Editing"))
	} else {
		data["webform"] = &Webform{}
		data["editNode"] = data["webform"]
		data["isCreateMode"] = true
		data["title"] = i18n.T(ctx, "Create form")
		c.SetCreateFormTrail(data, i18n.T(ctx, "Create form"))
	}
	c.ApplyEditorPageSEOFromTitle(data, r)
	return data
}

// newFormCommentMode - a new form's comments: hidden, unless the editor
// (who administers content) chooses otherwise.
const newFormCommentMode = modulekit.CommentModeHidden

// CreateForm handles GET /create/form.
func (c *WebformController) CreateForm(w http.ResponseWriter, r *http.Request) {
	f := &WebformForm{Status: 1, CommentMode: string(newFormCommentMode)}
	c.RenderOrFail(w, r, "node/edit_webform.html", c.editorData(r, f, nil, nil))
}

// Create handles POST /create/form.
func (c *WebformController) Create(w http.ResponseWriter, r *http.Request) {
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
	if errs := f.Validate(ctx); len(errs) > 0 {
		c.RenderOrFail(w, r, "node/edit_webform.html", c.editorData(r, f, nil, errs))
		return
	}
	wf := c.webformFromForm(r, f, user, nil)
	nodeID, err := c.webforms.Create(ctx, wf)
	if err != nil {
		c.LogFailure(err, "create form")
		httputil.InternalServerError(w)
		return
	}
	if c.administersContent(r, user) {
		if err := c.nodes.UpsertPageSEO(ctx, nodeID, f.MetaTitle, f.MetaDescription); err != nil {
			c.LogFailure(err, "error upserting page SEO", "nodeID", nodeID)
		}
	}
	created, err := c.webforms.ByNodeID(ctx, nodeID)
	if err != nil {
		c.LogFailure(err, "loading the form", "nodeID", nodeID)
		httputil.InternalServerError(w)
		return
	}
	http.Redirect(w, r, created.Node.URL(), http.StatusSeeOther)
}

// Edit handles GET /node/{id}/edit for a form.
func (c *WebformController) Edit(w http.ResponseWriter, r *http.Request) {
	node, ok := modulekit.RequestNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	wf, err := c.webforms.ByNodeID(r.Context(), node.ID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the form", "nodeID", node.ID)
		return
	}
	c.RenderOrFail(w, r, "node/edit_webform.html", c.editorData(r, WebformFormFromEntity(wf, sitetime.FromContext(r.Context())), wf, nil))
}

// Save handles POST /node/{id}/edit for a form.
func (c *WebformController) Save(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, ok := modulekit.RequestNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	user, ok := modulekit.RequestUser(r)
	if !ok || user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	existing, err := c.webforms.ByNodeID(ctx, node.ID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the form", "nodeID", node.ID)
		return
	}
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	f := &WebformForm{}
	f.ParseWebformForm(r)
	f.NodeID = node.ID
	if errs := f.Validate(ctx); len(errs) > 0 {
		c.RenderOrFail(w, r, "node/edit_webform.html", c.editorData(r, f, existing, errs))
		return
	}
	wf := c.webformFromForm(r, f, user, existing)
	if err := c.webforms.Update(ctx, wf); err != nil {
		c.LogFailure(err, "update form", "nodeID", node.ID)
		httputil.InternalServerError(w)
		return
	}
	if c.administersContent(r, user) {
		if err := c.nodes.UpsertPageSEO(ctx, node.ID, f.MetaTitle, f.MetaDescription); err != nil {
			c.LogFailure(err, "error upserting page SEO", "nodeID", node.ID)
		}
	}
	http.Redirect(w, r, wf.Node.URL(), http.StatusSeeOther)
}

// Delete handles POST /node/{id}/delete for a form: its submissions go
// with it.
func (c *WebformController) Delete(w http.ResponseWriter, r *http.Request) {
	node, ok := modulekit.RequestNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	if err := c.nodeDeleter.Delete(r.Context(), node.ID); err != nil {
		c.LogFailure(err, "delete form")
		httputil.InternalServerError(w)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// administersContent - whether user may set a form's node settings: the
// publishing, the author, the date, the address, the comments' mode and
// the meta tags (the editor shows them to them only).
func (c *WebformController) administersContent(r *http.Request, user *modulekit.User) bool {
	return c.AuthService().HasPermission(r.Context(), user, modulekit.PermAdministerContent)
}

// webformFromForm - the form of the editor's fields; status, moderation,
// author, date, address and comments' mode only from those who
// administer content.
func (c *WebformController) webformFromForm(r *http.Request, f *WebformForm, user *modulekit.User, existing *Webform) Webform {
	wf := Webform{
		Description:     f.Description,
		Confirmation:    f.Confirmation,
		ConfirmationURL: f.ConfirmationURL,
		SubmitLabel:     f.SubmitLabel,
		Closed:          f.Closed,
		NotifyEmail:     f.NotifyEmail,
		LimitPerUser:    f.LimitPerUser,
		Elements:        f.WebformElements(),
		Node: modulekit.Node{
			Title:        f.Title,
			Slug:         f.Slug,
			Status:       f.Status,
			OnModeration: f.OnModeration,
			CommentMode:  modulekit.CommentMode(f.CommentMode),
			CreatedBy:    *user,
		},
	}
	wf.MaxSubmissions, wf.ClosesAt = f.Limits(sitetime.FromContext(r.Context()))
	if existing != nil {
		wf.ID = existing.ID
		wf.Node.ID = existing.Node.ID
		wf.Node.CreatedBy = existing.Node.CreatedBy
		wf.Node.CreatedAt = existing.Node.CreatedAt
		if wf.Node.Slug == "" {
			wf.Node.Slug = existing.Node.Slug
		}
		if !modulekit.IsValidCommentMode(f.CommentMode) {
			wf.Node.CommentMode = existing.Node.CommentMode
		}
	} else if !modulekit.IsValidCommentMode(f.CommentMode) {
		// The comment settings are shown to who administers content: a
		// form anyone else creates gets the new form's.
		wf.Node.CommentMode = newFormCommentMode
	}
	if !c.administersContent(r, user) {
		wf.Node.Status, wf.Node.OnModeration = 1, 0
		wf.Node.Slug, wf.Node.CommentMode = "", newFormCommentMode
		if existing != nil {
			wf.Node.Status, wf.Node.OnModeration = existing.Node.Status, existing.Node.OnModeration
			wf.Node.Slug, wf.Node.CommentMode = existing.Node.Slug, existing.Node.CommentMode
		}
		return wf
	}
	if s := strings.TrimSpace(r.FormValue("created_by")); s != "" {
		if id, err := strconv.Atoi(s); err == nil && id > 0 {
			wf.Node.CreatedBy = modulekit.User{ID: id}
		}
	}
	if s := strings.TrimSpace(r.FormValue("created_at")); s != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", s, sitetime.FromContext(r.Context())); err == nil {
			wf.Node.CreatedAt = t
		}
	}
	return wf
}

// Detail handles a form's page (GET /<slug>): the form, or the
// confirmation after a submission (?sent=1).
func (c *WebformController) Detail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wf, err := c.webforms.BySlug(ctx, strings.TrimPrefix(r.URL.Path, "/"))
	if err != nil {
		if modulekit.IsNotFound(err) {
			c.RenderNotFound(w, r)
			return
		}
		if !errors.Is(err, context.Canceled) {
			c.LogFailure(err, "form by slug")
			httputil.InternalServerError(w)
		}
		return
	}
	user, _ := c.GetCurrentUser(r)
	if !c.RequireVisibleOrModerator(w, r, &wf.Node, user) {
		return
	}
	c.renderDetail(w, r, wf, user, nil, nil, r.URL.Query().Get("sent") == "1")
}

// webformValues - what was entered, by element ID, for the form shown
// again after an error.
type webformValues map[int][]string

// First - the element's first value ("" - none).
func (v webformValues) First(id int) string {
	if vals := v[id]; len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// Has - whether the element's values include option.
func (v webformValues) Has(id int, option string) bool {
	for _, s := range v[id] {
		if s == option {
			return true
		}
	}
	return false
}

// formData adds what webform_form renders: the form, what was entered and
// the errors, why it takes no submission from the user now ("Log in" with
// a link for a one-per-user form) and the CAPTCHA key.
func (c *WebformController) formData(ctx context.Context, data map[string]any, wf *Webform, user *modulekit.User, values map[int][]string, errs map[int]string) {
	data["webform"] = wf
	data["values"] = webformValues(values)
	if errs == nil {
		errs = map[int]string{}
	}
	data["fieldErrors"] = errs
	var userID *int
	if user != nil && user.ID > 0 {
		uid := user.ID
		userID = &uid
	}
	if err := c.webforms.Availability(ctx, *wf, userID, time.Now()); err != nil {
		var reason *i18n.Error
		if errors.As(err, &reason) {
			data["unavailable"] = reason.Source
			data["loginRequired"] = errors.Is(err, ErrWebformLoginRequired)
		} else {
			c.LogFailure(err, "form availability", "nodeID", wf.Node.ID)
		}
	}
	if modulekit.Captcha() != nil {
		data["recaptcha_site_key"] = modulekit.Captcha().SiteKey(manifest.CaptchaForm)
	}
	data["uploadLimit"] = int(c.webforms.UploadLimit(ctx))
}

// Embed handles GET /node/{id}/form/embed: the form alone (webform_form,
// no page around it) for the "Form" block, which loads it into any page;
// forms.js sends it and shows the answer in its place (Submit). Not cached: what it shows
// depends on the visitor.
func (c *WebformController) Embed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid, ok := c.PathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	wf, err := c.webforms.ByNodeID(ctx, nid)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the form", "nid", nid)
		return
	}
	user, _ := c.GetCurrentUser(r)
	if !c.AuthService().CanViewNode(ctx, &wf.Node, user) {
		http.NotFound(w, r)
		return
	}
	c.renderFragment(w, r, wf, user, nil, nil, "webform_form", http.StatusOK)
}

// renderFragment writes the form's part define (webform_form - the form
// with values and errs; webform_sent - the confirmation) alone, with
// status: the "Form" block's content and forms.js's answers.
func (c *WebformController) renderFragment(w http.ResponseWriter, r *http.Request, wf *Webform, user *modulekit.User, values map[int][]string, errs map[int]string, define string, status int) {
	ctx := r.Context()
	data := map[string]any{
		modulekit.RequestLocationKey: sitetime.FromContext(ctx),
		modulekit.RequestLanguageKey: i18n.FromContext(ctx),
		modulekit.RequestThemeKey:    modulekit.RequestTheme(ctx),
	}
	c.formData(ctx, data, wf, user, values, errs)
	var buf bytes.Buffer
	if err := c.TemplateService().RenderNamed(&buf, define, data, "webform/detail.html"); err != nil {
		c.LogFailure(err, "render a form's part", "nodeID", wf.Node.ID, "define", define)
		httputil.InternalServerError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// sentByScript - a submission forms.js sends (X-Requested-With): answered
// with the form's part, not a page.
func sentByScript(r *http.Request) bool {
	return r.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// showAgain shows the form again with what was entered and errs: its page,
// or, to forms.js, the form alone (422 Unprocessable Content).
func (c *WebformController) showAgain(w http.ResponseWriter, r *http.Request, wf *Webform, user *modulekit.User, values map[int][]string, errs map[int]string) {
	if sentByScript(r) {
		c.renderFragment(w, r, wf, user, values, errs, "webform_form", http.StatusUnprocessableEntity)
		return
	}
	c.renderDetail(w, r, wf, user, values, errs, false)
}

func (c *WebformController) renderDetail(w http.ResponseWriter, r *http.Request, wf *Webform, user *modulekit.User, values map[int][]string, errs map[int]string, sent bool) {
	ctx := r.Context()
	canEdit, canDelete := c.EditDeletePermissions(ctx, &wf.Node, user)
	data := c.GetCommonData(r)
	c.formData(ctx, data, wf, user, values, errs)
	data["item"] = wf
	data["sent"] = sent
	data["canEdit"] = canEdit
	data["canDelete"] = canDelete
	data["moduleNode"] = modulekit.ModuleNodeData(ctx, wf.Node)
	if c.AuthService().HasPermission(ctx, user, manifest.PermViewSubmissions) {
		if n, err := c.webforms.CountSubmissions(ctx, *wf); err == nil {
			data["canViewSubmissions"] = true
			data["submissionsCount"] = n
		}
	}
	seo := modulekit.PageSEO{Title: wf.Node.Title, Description: modulekit.TruncateMeta(wf.Description, 160)}
	if wf.Node.PageSEO.Title != "" {
		seo.Title = wf.Node.PageSEO.Title
	}
	if wf.Node.PageSEO.Description != "" {
		seo.Description = wf.Node.PageSEO.Description
	}
	data["pageSEO"] = seo
	c.SetViewTrackNID(data, wf.Node)
	c.SetNodeRefTrail(ctx, data, wf.Node)
	c.RenderOrFail(w, r, "webform/detail.html", data)
}

// Submit handles POST /node/{id}/form: the values (element_<id>, several
// for checkboxes) are checked and stored, then the page shows the
// confirmation; a field error or a failed CAPTCHA shows the form again
// with what was entered. To forms.js (sentByScript) - the form's part in
// place of the page: the form again (422) or the confirmation; with a
// page after sending, its address in X-Webform-Redirect.
func (c *WebformController) Submit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	wf, err := c.webforms.ByNodeID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the form", "nodeID", id)
		return
	}
	user, _ := c.GetCurrentUser(r)
	if !c.RequireVisibleOrModerator(w, r, &wf.Node, user) {
		return
	}
	// By what was sent, not by the form: a multipart body (a form with a
	// file, or a script's FormData) is read as one.
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		err = r.ParseMultipartForm(32 << 20)
	} else {
		err = r.ParseForm()
	}
	if err != nil {
		c.showAgain(w, r, wf, user, nil, map[int]string{0: i18n.N("The form could not be read: is a file too large?")})
		return
	}
	values := map[int][]string{}
	uploads := map[int]WebformUpload{}
	limit := c.webforms.UploadLimit(ctx)
	for _, e := range wf.Elements {
		key := "element_" + strconv.Itoa(e.ID)
		if e.Type == WebformFile {
			if up, ok := readWebformUpload(r, key, limit); ok {
				uploads[e.ID] = up
			}
			continue
		}
		if v, ok := r.Form[key]; ok {
			values[e.ID] = v
		}
	}
	if modulekit.CheckBotTrap(r) != nil {
		c.showAgain(w, r, wf, user, values, map[int]string{0: i18n.N("The form was sent too quickly, or looks sent by a program. Wait a few seconds and send it again.")})
		return
	}
	if modulekit.Captcha() != nil {
		ok, err := modulekit.Captcha().Verify(ctx, manifest.CaptchaForm, r.FormValue("g-recaptcha-response"), httputil.ClientIP(r))
		if err != nil || !ok {
			c.showAgain(w, r, wf, user, values, map[int]string{0: i18n.N("Confirm that you are not a robot.")})
			return
		}
	}
	from := WebformSender{RemoteIP: httputil.ClientIP(r), Source: webformSource(r, wf)}
	if user != nil && user.ID > 0 {
		uid := user.ID
		from.UserID = &uid
	}
	errs, err := c.webforms.Submit(ctx, *wf, values, uploads, from)
	var refusal *i18n.Error
	switch {
	case errors.As(err, &refusal):
		c.showAgain(w, r, wf, user, values, map[int]string{0: refusal.Source})
		return
	case err != nil:
		c.LogFailure(err, "submit form", "nodeID", wf.Node.ID)
		httputil.InternalServerError(w)
		return
	case len(errs) > 0:
		c.showAgain(w, r, wf, user, values, errs)
		return
	}
	if sentByScript(r) {
		if wf.ConfirmationURL != "" {
			w.Header().Set("X-Webform-Redirect", wf.ConfirmationURL)
			w.WriteHeader(http.StatusOK)
			return
		}
		c.renderFragment(w, r, wf, user, nil, nil, "webform_sent", http.StatusOK)
		return
	}
	if wf.ConfirmationURL != "" {
		http.Redirect(w, r, wf.ConfirmationURL, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, wf.Node.URL()+"?sent=1", http.StatusSeeOther)
}

// webformSource - the site's page a submission came from: what forms.js
// put in webform_source (the page the form is embedded in, or the one that
// linked to its own page), else the Referer unless it's the form's own
// page; "" - unknown.
func webformSource(r *http.Request, wf *Webform) string {
	hosts := []string{r.Host, modulekit.SiteDomain()}
	if src := CleanWebformSource(r.FormValue("webform_source"), hosts...); src != "" {
		return src
	}
	ref := CleanWebformSource(r.Referer(), hosts...)
	own := wf.Node.URL()
	if ref == "" || ref == own || strings.HasPrefix(ref, own+"?") {
		return ""
	}
	return ref
}

// submissionsAccess - the form of /node/{id}/form/submissions… for a user
// who may view its submissions; otherwise the response is written and nil
// returned.
func (c *WebformController) submissionsAccess(w http.ResponseWriter, r *http.Request) *Webform {
	user, _ := c.GetCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return nil
	}
	if !c.AuthService().HasPermission(r.Context(), user, manifest.PermViewSubmissions) {
		c.RenderForbidden(w, r)
		return nil
	}
	nid, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return nil
	}
	wf, err := c.webforms.ByNodeID(r.Context(), nid)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the form", "nodeID", nid)
		return nil
	}
	return wf
}

// Submissions handles GET /node/{id}/form/submissions: the form's
// submissions, newest first, a page at a time, with the CSV export.
func (c *WebformController) Submissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wf := c.submissionsAccess(w, r)
	if wf == nil {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	total, err := c.webforms.CountSubmissions(ctx, *wf)
	if err != nil {
		c.LogFailure(err, "count form submissions")
		httputil.InternalServerError(w)
		return
	}
	subs, err := c.webforms.Submissions(ctx, *wf, page)
	if err != nil {
		c.LogFailure(err, "form submissions")
		httputil.InternalServerError(w)
		return
	}
	user, _ := c.GetCurrentUser(r)
	canEdit, canDelete := c.EditDeletePermissions(ctx, &wf.Node, user)
	data := c.GetCommonData(r)
	data["webform"] = wf
	data["item"] = wf
	data["submissions"] = subs
	data["total"] = total
	data["page"] = page
	data["hasNext"] = page*WebformSubmissionsPerPage < total
	data["canEdit"] = canEdit
	data["canDelete"] = canDelete
	data["canDeleteSubmissions"] = c.AuthService().HasPermission(ctx, user, manifest.PermDeleteSubmissions)
	if n, err := strconv.Atoi(r.URL.Query().Get("deleted")); err == nil {
		data["deletedCount"] = n
	}
	title := i18n.T(ctx, "Submissions: @title", "@title", wf.Node.Title)
	data["title"] = title
	data["pageSEO"] = modulekit.PageSEO{Title: title, NoIndex: true}
	c.SetNodeSubpageTrail(ctx, data, wf.Node, i18n.T(ctx, "Submissions"))
	c.RenderOrFail(w, r, "webform/submissions.html", data)
}

// Submission handles GET /node/{id}/form/submissions/{sid}: one
// submission - every question with its answer, the sender, links to the
// newer and older ones and deleting it.
func (c *WebformController) Submission(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wf := c.submissionsAccess(w, r)
	if wf == nil {
		return
	}
	sid, err := strconv.Atoi(r.PathValue("sid"))
	if err != nil || sid <= 0 {
		c.RenderNotFound(w, r)
		return
	}
	sub, newer, older, err := c.webforms.Submission(ctx, *wf, sid)
	if err != nil {
		c.LogFailure(err, "form submission", "nodeID", wf.Node.ID, "submissionID", sid)
		httputil.InternalServerError(w)
		return
	}
	if sub == nil {
		c.RenderNotFound(w, r)
		return
	}
	user, _ := c.GetCurrentUser(r)
	canEdit, canDelete := c.EditDeletePermissions(ctx, &wf.Node, user)
	data := c.GetCommonData(r)
	data["webform"] = wf
	data["item"] = wf
	data["submission"] = sub
	data["newer"] = newer
	data["older"] = older
	data["canEdit"] = canEdit
	data["canDelete"] = canDelete
	data["canDeleteSubmissions"] = c.AuthService().HasPermission(ctx, user, manifest.PermDeleteSubmissions)
	title := i18n.T(ctx, "Submission #@id: @title", "@id", sid, "@title", wf.Node.Title)
	data["title"] = title
	data["pageSEO"] = modulekit.PageSEO{Title: title, NoIndex: true}
	c.SetNodeSubpageTrail(ctx, data, wf.Node, i18n.T(ctx, "Submission #@id", "@id", sid))
	c.RenderOrFail(w, r, "webform/submission.html", data)
}

// SubmissionsCSV handles GET /node/{id}/form/submissions.csv: every
// submission as CSV (UTF-8 with a BOM for spreadsheet programs): number,
// date, sender, IP, then a column per element (several values joined by
// "; ").
func (c *WebformController) SubmissionsCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wf := c.submissionsAccess(w, r)
	if wf == nil {
		return
	}
	name := wf.Node.Slug
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`-submissions.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	if err := writeWebformCSV(ctx, w, *wf, c.webforms.EachSubmission); err != nil {
		c.LogFailure(err, "form submissions CSV", "nodeID", wf.Node.ID)
	}
}

// csvCell - a text cell safe to open in a spreadsheet program: a value
// starting with =, +, -, @, a tab or a carriage return would be read as a
// formula there (CSV injection), so it gets a leading apostrophe, shown as
// text.
func csvCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// writeWebformCSV writes the form's submissions as CSV to out.
func writeWebformCSV(ctx context.Context, out io.Writer, wf Webform, each func(context.Context, Webform, func(WebformSubmission) error) error) error {
	if _, err := io.WriteString(out, "\ufeff"); err != nil {
		return err
	}
	cw := csv.NewWriter(out)
	header := []string{"#", i18n.T(ctx, "Date"), i18n.T(ctx, "User"), "IP", i18n.T(ctx, "Page")}
	for _, e := range wf.Elements {
		header = append(header, csvCell(e.Label))
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	loc := sitetime.FromContext(ctx)
	err := each(ctx, wf, func(s WebformSubmission) error {
		row := []string{strconv.Itoa(s.ID), s.CreatedAt.In(loc).Format("2006-01-02 15:04:05"), csvCell(s.Username), s.RemoteIP, csvCell(s.SourceURL)}
		for _, e := range wf.Elements {
			v := strings.Join(s.Values[e.ID], "; ")
			switch {
			case e.Type == WebformConsent && v != "":
				v = i18n.T(ctx, "Yes")
			case e.Type == WebformFile:
				var names []string
				for _, fv := range s.Values[e.ID] {
					names = append(names, e.FileName(fv))
				}
				v = strings.Join(names, "; ")
			}
			row = append(row, csvCell(v))
		}
		return cw.Write(row)
	})
	if err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// DeleteSubmissions handles POST /node/{id}/form/submissions/delete: the
// checked submissions (ids), or the one a row's button names (only), for
// those who may delete them; back to the list with how many went.
func (c *WebformController) DeleteSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wf := c.submissionsAccess(w, r)
	if wf == nil {
		return
	}
	user, _ := c.GetCurrentUser(r)
	if !c.AuthService().HasPermission(ctx, user, manifest.PermDeleteSubmissions) {
		c.RenderForbidden(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	raw := r.Form["ids"]
	if only := r.FormValue("only"); only != "" {
		raw = []string{only}
	}
	var ids []int
	for _, v := range raw {
		if id, err := strconv.Atoi(v); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	n, err := c.webforms.DeleteSubmissions(ctx, *wf, ids)
	if err != nil {
		c.LogFailure(err, "delete form submissions", "nodeID", wf.Node.ID)
		httputil.InternalServerError(w)
		return
	}
	back := "/node/" + strconv.Itoa(wf.Node.ID) + "/form/submissions?deleted=" + strconv.Itoa(n)
	if page, err := strconv.Atoi(r.FormValue("page")); err == nil && page > 1 {
		back += "&page=" + strconv.Itoa(page)
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// PurgeSubmissions handles POST /node/{id}/form/submissions/purge: deletes
// the form's submissions older than the given days (with their files) -
// for those who may delete submissions.
func (c *WebformController) PurgeSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wf := c.submissionsAccess(w, r)
	if wf == nil {
		return
	}
	user, _ := c.GetCurrentUser(r)
	if !c.AuthService().HasPermission(ctx, user, manifest.PermDeleteSubmissions) {
		c.RenderForbidden(w, r)
		return
	}
	days, err := strconv.Atoi(r.FormValue("days"))
	if err != nil || days < 1 {
		c.RenderBadForm(w, r)
		return
	}
	n, err := c.webforms.PurgeSubmissions(ctx, *wf, days, time.Now())
	if err != nil {
		c.LogFailure(err, "purge form submissions", "nodeID", wf.Node.ID, "deleted", n)
		httputil.InternalServerError(w)
		return
	}
	http.Redirect(w, r, "/node/"+strconv.Itoa(wf.Node.ID)+"/form/submissions?deleted="+strconv.Itoa(n), http.StatusSeeOther)
}

// Analysis handles GET /node/{id}/form/analysis: the form's answers summed
// up - per question those who answered, per choice each option's count.
func (c *WebformController) Analysis(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wf := c.submissionsAccess(w, r)
	if wf == nil {
		return
	}
	analysis, err := c.webforms.Analysis(ctx, *wf)
	if err != nil {
		c.LogFailure(err, "form analysis", "nodeID", wf.Node.ID)
		httputil.InternalServerError(w)
		return
	}
	user, _ := c.GetCurrentUser(r)
	canEdit, canDelete := c.EditDeletePermissions(ctx, &wf.Node, user)
	data := c.GetCommonData(r)
	data["webform"] = wf
	data["item"] = wf
	data["analysis"] = analysis
	data["canEdit"] = canEdit
	data["canDelete"] = canDelete
	title := i18n.T(ctx, "Analysis: @title", "@title", wf.Node.Title)
	data["title"] = title
	data["pageSEO"] = modulekit.PageSEO{Title: title, NoIndex: true}
	c.SetNodeSubpageTrail(ctx, data, wf.Node, i18n.T(ctx, "Analysis"))
	c.RenderOrFail(w, r, "webform/analysis.html", data)
}

// readWebformUpload - the file posted as key, read up to limit+1 bytes (so
// a larger one is seen as too large without reading all of it).
func readWebformUpload(r *http.Request, key string, limit int64) (WebformUpload, bool) {
	f, h, err := r.FormFile(key)
	if err != nil {
		return WebformUpload{}, false
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return WebformUpload{}, false
	}
	return WebformUpload{Name: h.Filename, Data: data}, true
}

// File handles GET /node/{id}/form/file?sid=&eid=&d=: a file sent with the
// form, for those who may view its submissions - always as a download
// (never shown in the browser) under its original name.
func (c *WebformController) File(w http.ResponseWriter, r *http.Request) {
	wf := c.submissionsAccess(w, r)
	if wf == nil {
		return
	}
	q := r.URL.Query()
	sid, _ := strconv.Atoi(q.Get("sid"))
	eid, _ := strconv.Atoi(q.Get("eid"))
	delta, _ := strconv.Atoi(q.Get("d"))
	file, info, name, err := c.webforms.OpenSubmissionFile(r.Context(), *wf, sid, eid, delta)
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	defer func() { _ = file.Close() }()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", info.ModTime, file)
}
