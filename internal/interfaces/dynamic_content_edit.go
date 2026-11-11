package interfaces

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
	"github.com/kosatka-cms/kosatka/pkg/textcut"
)

// maxNodeFormSize - the memory a node form's upload is parsed in (the
// rest goes to temporary files).
const maxNodeFormSize = 10 << 20

// contentForm - one content type's create and edit form: the type and
// its fields (resolved per request, resolveByMachineName) and the
// service storing its nodes.
type contentForm struct {
	def    entity.ContentTypeDef
	fields entity.FieldInstanceDefs
	store  service.DynamicContentService
}

// contentForm - the form of the type named name; false - answered (the
// type's load failed).
func (c *DynamicContentController) contentForm(w http.ResponseWriter, r *http.Request, name string) (contentForm, bool) {
	def, fields, err := c.resolveByMachineName(r.Context(), name)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the content type", "type", name)
		return contentForm{}, false
	}
	return contentForm{def: *def, fields: fields, store: c.dynamicService(*def, fields)}, true
}

// createForm - the form of a new node of the request's type; false -
// answered: the type's load failed, or it is disabled
// (ContentTypeDef.Creatable) or a field set (its nodes are blocks) and,
// like an unknown type, not found.
func (c *DynamicContentController) createForm(w http.ResponseWriter, r *http.Request) (contentForm, bool) {
	cf, ok := c.contentForm(w, r, r.PathValue("type"))
	if !ok {
		return contentForm{}, false
	}
	if !cf.def.Creatable() || cf.def.Embedded {
		c.RenderNotFound(w, r)
		return contentForm{}, false
	}
	return cf, true
}

// CreateForm renders GET /create/{type} for an admin-created type.
func (c *DynamicContentController) CreateForm(w http.ResponseWriter, r *http.Request) {
	cf, ok := c.createForm(w, r)
	if !ok {
		return
	}
	c.renderContentForm(w, r, cf, cf.emptyForm(), entity.DynamicContent{}, nil, nil)
}

// CreateSubmit handles POST /create/{type} for an admin-created type:
// the form checked, the node created with its slug (nodeSlug), its SEO,
// terms and the modules' settings saved (saveContentExtras) in one
// transaction; to the new node.
func (c *DynamicContentController) CreateSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cf, ok := c.createForm(w, r)
	if !ok {
		return
	}
	user, ok := currentNodeUser(c.BaseController, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseMultipartForm(maxNodeFormSize); err != nil {
		c.LogFailure(err, "error parsing the node form")
		c.RenderBadForm(w, r)
		return
	}
	f, err := c.parseContentForm(r, cf, 0, nil)
	if err != nil {
		c.LogFailure(err, "error parsing the node form", "type", cf.def.MachineName)
		c.RenderBadForm(w, r)
		return
	}
	errs := c.validateContentForm(ctx, cf, f)
	c.validateWorkflow(r, cf, nil, errs)
	if len(errs) > 0 {
		c.renderContentForm(w, r, cf, *f, sentBack(ctx, cf, f, entity.DynamicContent{}), nil, errs)
		return
	}

	// newNode - the content f describes, with its address (a path
	// pattern's may take its terms).
	newNode := func(ctx context.Context) (*entity.DynamicContent, error) {
		e := cf.toEntity(f)
		slug, err := c.nodeSlug(ctx, cf.def, f, e)
		if err != nil {
			return nil, fmt.Errorf("making the slug (title %q): %w", f.Title, err)
		}
		e.Node.Type = entity.ContentType(cf.def.MachineName)
		e.Node.Slug, f.Slug = slug, slug
		e.Node.CreatedBy = *user
		// A new node gets the type's comment mode unless the form gives one.
		if !entity.IsValidCommentMode(string(e.Node.CommentMode)) {
			e.Node.CommentMode = cf.def.DefaultCommentMode
		}
		return e, nil
	}
	e, err := newNode(ctx)
	if err != nil {
		c.LogFailure(err, "error making the slug", "type", cf.def.MachineName)
		httputil.InternalServerError(w)
		return
	}
	if !e.Node.PathAuto {
		if msg, err := c.slugTaken(ctx, e.Node.Slug); err != nil || msg != "" {
			if err != nil {
				c.LogFailure(err, "checking the typed address", "slug", e.Node.Slug)
				httputil.InternalServerError(w)
				return
			}
			c.renderContentForm(w, r, cf, *f, sentBack(ctx, cf, f, entity.DynamicContent{}), nil, map[string]string{"slug": msg})
			return
		}
	}
	// The node and what goes with it in one transaction: a failure leaves
	// no half-saved node. What reading the form only noted (the alt texts,
	// the new terms) is written first, and the node made again with the
	// terms.
	var created *entity.DynamicContent
	err = c.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		if err := c.writeFormChanges(ctx, f, cf.fields, user.ID); err != nil {
			return err
		}
		if e, err = newNode(ctx); err != nil {
			return err
		}
		if created, err = cf.store.Create(ctx, e); err != nil {
			return err
		}
		if err := c.saveContentExtras(ctx, r, cf, created.Node.ID, f, true); err != nil {
			return err
		}
		if c.revisions == nil {
			return nil
		}
		return c.revisions.Created(ctx, created.Node.ID, user)
	})
	if err != nil {
		c.LogFailure(err, "error creating the node", "type", cf.def.MachineName)
		httputil.InternalServerError(w)
		return
	}
	if len(e.Node.ViewRoles) > 0 {
		c.viewRolesChanged()
	}
	c.logger.Info("node created", "type", cf.def.MachineName, "nodeID", created.Node.ID, "slug", e.Node.Slug)
	draftSaved(w, r, draftKey(user.ID, cf.def.MachineName, 0))
	http.Redirect(w, r, "/"+entity.NodeRelPath(e.Node.Type, e.Node.Slug), http.StatusFound)
}

// EditForm renders GET /node/{id}/edit for a node of an admin-created type.
func (c *DynamicContentController) EditForm(w http.ResponseWriter, r *http.Request) {
	node, ok := middleware.GetNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	cf, ok := c.contentForm(w, r, node.Type.ToString())
	if !ok {
		return
	}
	existing, err := cf.store.ByNodeID(r.Context(), node.ID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the node to edit", "nodeID", node.ID)
		return
	}
	c.renderContentForm(w, r, cf, *dynamicFormFromEntity(existing), *existing, node, nil)
}

// Save handles POST /node/{id}/edit for a node of an admin-created type:
// as CreateSubmit, over the node's own values (applyEdit); the old
// address redirects to a changed slug.
func (c *DynamicContentController) Save(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, ok := middleware.GetNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	cf, ok := c.contentForm(w, r, node.Type.ToString())
	if !ok {
		return
	}
	user, ok := currentNodeUser(c.BaseController, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	existing, err := cf.store.ByNodeID(ctx, node.ID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the node to save", "nodeID", node.ID)
		return
	}
	if err := r.ParseMultipartForm(maxNodeFormSize); err != nil {
		c.LogFailure(err, "error parsing the node form")
		c.RenderBadForm(w, r)
		return
	}
	f, err := c.parseContentForm(r, cf, node.ID, existing)
	if err != nil {
		c.LogFailure(err, "error parsing the node form", "type", cf.def.MachineName)
		c.RenderBadForm(w, r)
		return
	}
	errs := c.validateContentForm(ctx, cf, f)
	c.validateWorkflow(r, cf, node, errs)
	if len(errs) > 0 {
		c.renderContentForm(w, r, cf, *f, sentBack(ctx, cf, f, *existing), node, errs)
		return
	}

	// updatedNode - the node with f's values, with its address (a path
	// pattern's may take its terms).
	updatedNode := func(ctx context.Context) (*entity.DynamicContent, error) {
		updated := cf.toEntity(f)
		applyEdit(updated, existing, f)
		slug, err := c.nodeSlug(ctx, cf.def, f, updated)
		if err != nil {
			return nil, fmt.Errorf("making the slug (node %d): %w", node.ID, err)
		}
		updated.Node.Slug, f.Slug = slug, slug
		return updated, nil
	}
	updated, err := updatedNode(ctx)
	if err != nil {
		c.LogFailure(err, "error making the slug", "type", cf.def.MachineName)
		httputil.InternalServerError(w)
		return
	}
	slug := updated.Node.Slug
	if !updated.Node.PathAuto && slug != existing.Node.Slug {
		if msg, err := c.slugTaken(ctx, slug); err != nil || msg != "" {
			if err != nil {
				c.LogFailure(err, "checking the typed address", "slug", slug)
				httputil.InternalServerError(w)
				return
			}
			c.renderContentForm(w, r, cf, *f, sentBack(ctx, cf, f, *existing), node, map[string]string{"slug": msg})
			return
		}
	}
	// The node, what goes with it and the old address's redirect in one
	// transaction, recorded in its history: a failure leaves the node as
	// it was.
	err = c.trackSave(ctx, node.ID, user, func(ctx context.Context) error {
		// What reading the form only noted (the alt texts, the new terms)
		// first; the node again with the terms.
		if err := c.writeFormChanges(ctx, f, cf.fields, user.ID); err != nil {
			return err
		}
		var err error
		if updated, err = updatedNode(ctx); err != nil {
			return err
		}
		slug = updated.Node.Slug
		if _, err := cf.store.Update(ctx, updated); err != nil {
			return err
		}
		if err := c.saveContentExtras(ctx, r, cf, node.ID, f, false); err != nil {
			return err
		}
		if c.redirects != nil && slug != node.Slug {
			if err := c.redirects.MoveSlug(ctx, node.Slug, slug); err != nil {
				return fmt.Errorf("redirecting the old slug %q: %w", node.Slug, err)
			}
		}
		return nil
	})
	if err != nil {
		c.LogFailure(err, "error updating the node", "nodeID", node.ID, "type", cf.def.MachineName)
		httputil.InternalServerError(w)
		return
	}
	if !slices.Equal(updated.Node.ViewRoles, existing.Node.ViewRoles) {
		c.viewRolesChanged()
	}
	c.logger.Info("node updated", "type", cf.def.MachineName, "nodeID", node.ID, "slug", slug)
	draftSaved(w, r, draftKey(user.ID, cf.def.MachineName, node.ID))
	http.Redirect(w, r, "/"+entity.NodeRelPath(node.Type, slug), http.StatusSeeOther)
}

// Delete handles POST /node/{id}/delete for a node of an admin-created
// type (the node - the access middleware's, NodeAccessByIDMiddleware):
// it goes to the trash.
func (c *DynamicContentController) Delete(w http.ResponseWriter, r *http.Request) {
	node, ok := middleware.GetNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	user, _ := c.GetCurrentUser(r)
	// To the trash (NodeDeleteService.Remove), restored from there or
	// deleted for good later.
	if _, err := c.nodeDeleter.Remove(r.Context(), node.ID, user); err != nil {
		c.LogFailure(err, "error deleting the node", "nodeID", node.ID)
		httputil.InternalServerError(w)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// Clone handles POST /node/{id}/clone: a copy of the node (who may copy
// content, edit it and create its type's content: mayCopy) - its fields' values, the blocks of
// its blocks fields copied too, its flags, comment mode and view roles -
// titled "Copy of …", unpublished, by the user, with an address of its
// own; its form opens.
func (c *DynamicContentController) Clone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	node, ok := middleware.GetNode(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	user, ok := currentNodeUser(c.BaseController, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	cf, ok := c.contentForm(w, r, node.Type.ToString())
	if !ok {
		return
	}
	if cf.def.Embedded || !c.mayCopy(ctx, node.Type, user) {
		c.RenderForbidden(w, r)
		return
	}
	id, err := c.copyNode(ctx, cf, *node, user, "")
	if err != nil {
		c.LogFailure(err, "error copying the node", "nodeID", node.ID)
		httputil.InternalServerError(w)
		return
	}
	c.logger.Info("node copied", "from", node.ID, "nodeID", id)
	http.Redirect(w, r, fmt.Sprintf("/node/%d/edit", id), http.StatusSeeOther)
}

// mayCopy - whether user may "Make a copy" of content of contentType
// they may edit: the copying permission and creating the type's content.
func (c *DynamicContentController) mayCopy(ctx context.Context, contentType entity.ContentType, user *entity.User) bool {
	return c.authService.HasPermission(ctx, user, service.PermCopyContent) && c.authService.CanCreateNode(ctx, contentType, user)
}

// copyNode creates a copy of node (an admin-created type's, cf its form)
// by user: its fields' values, the blocks of its blocks fields copied
// too, its flags, comment mode and view roles - titled title ("" - "Copy
// of …"), unpublished, with an address of its own, in ctx's language
// (the node's language follows the request's).
func (c *DynamicContentController) copyNode(ctx context.Context, cf contentForm, node entity.Node, user *entity.User, title string) (int, error) {
	source, err := cf.store.ByNodeID(ctx, node.ID)
	if err != nil {
		return 0, fmt.Errorf("loading node %d to copy: %w", node.ID, err)
	}
	if title == "" {
		title = textcut.Runes(i18n.T(ctx, "Copy of @title", "@title", source.Node.Title), 255)
	}
	e := &entity.DynamicContent{
		Node: entity.Node{
			Type:        node.Type,
			Title:       title,
			CommentMode: source.Node.CommentMode,
			Promote:     source.Node.Promote,
			Sticky:      source.Node.Sticky,
			ViewRoles:   source.Node.ViewRoles,
			CreatedBy:   *user,
		},
		ContentType: cf.def,
		Fields:      cf.fields,
		Values:      source.Values,
		TextFormats: source.TextFormats,
	}
	slug, err := c.nodeSlug(ctx, cf.def, &form.DynamicForm{Title: e.Node.Title}, e)
	if err != nil {
		return 0, fmt.Errorf("making the copy's slug: %w", err)
	}
	e.Node.Slug = slug
	var created *entity.DynamicContent
	err = c.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		if created, err = cf.store.Create(ctx, e); err != nil {
			return err
		}
		if err := saveContentTaxonomy(ctx, c.taxonomyService, created.Node.ID); err != nil {
			return err
		}
		if c.revisions == nil {
			return nil
		}
		return c.revisions.Created(ctx, created.Node.ID, user)
	})
	if err != nil {
		return 0, err
	}
	return created.Node.ID, nil
}

// CopyNode - a copy of the node nodeID of an admin-created type by user,
// titled title ("" - "Copy of …"), unpublished, in ctx's language: its
// id (ErrNodeNotCopyable - a field set's node or a module's type's).
func (c *DynamicContentController) CopyNode(ctx context.Context, nodeID int, user *entity.User, title string) (int, error) {
	node, err := c.nodeService.ByID(ctx, nodeID)
	if err != nil {
		return 0, fmt.Errorf("loading node %d to copy: %w", nodeID, err)
	}
	def, fields, err := c.resolveByMachineName(ctx, node.Type.ToString())
	if err != nil {
		return 0, fmt.Errorf("loading the type of node %d: %w", nodeID, err)
	}
	if def.Embedded {
		return 0, ErrNodeNotCopyable
	}
	return c.copyNode(ctx, contentForm{def: *def, fields: fields, store: c.dynamicService(*def, fields)}, *node, user, title)
}

// ErrNodeNotCopyable - CopyNode's node can't be copied.
var ErrNodeNotCopyable = errors.New("the node can't be copied")

// viewRoleOption - a role a node's view may be given to: its name and
// label.
type viewRoleOption struct {
	Name, Label string
}

// viewRoleOptions - a guest, then the site's roles.
func (c *DynamicContentController) viewRoleOptions(ctx context.Context) []viewRoleOption {
	opts := []viewRoleOption{{Name: entity.RoleAnonymous, Label: i18n.T(ctx, "Guest (not signed in)")}}
	if c.roles == nil {
		return opts
	}
	roles, err := c.roles(ctx)
	if err != nil {
		c.LogFailure(err, "listing the roles for a node's view")
		return opts
	}
	for _, r := range roles {
		opts = append(opts, viewRoleOption{Name: r.Name, Label: r.Label()})
	}
	return opts
}

// viewRolesChanged - a node's view roles changed: who views what is read
// again, the cached blocks rendered again (DynamicContentDeps.AccessChanged).
func (c *DynamicContentController) viewRolesChanged() {
	if c.accessChanged != nil {
		c.accessChanged()
	}
}

// trackSave runs save of node nodeID by user in one transaction,
// recorded in the node's history (without one wired, the transaction
// alone).
func (c *DynamicContentController) trackSave(ctx context.Context, nodeID int, user *entity.User, save func(ctx context.Context) error) error {
	if c.revisions == nil {
		return c.uow.Do(ctx, save)
	}
	return c.revisions.Track(ctx, nodeID, user, save)
}

// renderContentForm renders the node form: the create form (node nil) or
// the edit form of node, e - the saved node (empty on create), errs -
// what to correct. A service page: no sidebar.
func (c *DynamicContentController) renderContentForm(w http.ResponseWriter, r *http.Request, cf contentForm, f form.DynamicForm, e entity.DynamicContent, node *entity.Node, errs map[string]string) {
	ctx := r.Context()
	data := c.GetCommonData(r)
	data["servicePage"] = true
	maps.Copy(data, c.pageData(ctx, cf.def, cf.fields, f, e))
	if user, ok := currentNodeUser(c.BaseController, r); ok {
		nodeID := 0
		if node != nil {
			nodeID = node.ID
		}
		data["autosaveKey"] = draftKey(user.ID, cf.def.MachineName, nodeID)
		if !cf.def.NoPreview {
			data["formPreviewURL"] = FormPreviewPath(cf.def.MachineName, nodeID)
		}
	}
	// A workflow's type: its part of the form (node_workflow_<module>).
	if wf := NodeWorkflowFor(ctx, cf.def.MachineName); wf != nil {
		user, _ := currentNodeUser(c.BaseController, r)
		state, err := wf.Load(ctx, cf.def.MachineName, node, user)
		if err != nil {
			c.LogFailure(err, "loading the workflow of a node's form", "type", cf.def.MachineName)
		} else {
			data["nodeWorkflow"] = state
		}
	}
	if errs != nil {
		data["errors"] = errs
	}
	if node == nil {
		title := i18n.T(ctx, "Create: @type", "@type", cf.def.Label)
		data["isCreateMode"] = true
		data["title"] = title
		c.setCreateFormTrail(data, title)
	} else {
		data["title"] = i18n.T(ctx, "Editing: @title", "@title", e.Node.Title)
		c.setNodeSubpageTrail(ctx, data, *node, i18n.T(ctx, "Editing"))
	}
	c.ApplyEditorPageSEOFromTitle(data, r)
	c.RenderOrFail(w, r, "node/edit_dynamic.html", data)
}

// draftSavedCookie - set when a content form is saved: the autosave
// script (55-autosave.js) drops the form's draft on the next page and
// clears it. Its value - the form's key (draftKey).
const draftSavedCookie = "kosatka_draft_saved"

// draftKey - the key of the draft the autosave script keeps of a content
// form (its data-autosave): of the user, of the node (nodeID) or, 0, of a
// new one of the type.
func draftKey(userID int, contentType string, nodeID int) string {
	if nodeID == 0 {
		return fmt.Sprintf("%d:create:%s", userID, contentType)
	}
	return fmt.Sprintf("%d:node:%d", userID, nodeID)
}

// draftSaved tells the autosave script that the form of key is saved: its
// draft isn't dropped before (a refused submit - an expired session, a
// stale CSRF token - would lose it).
func draftSaved(w http.ResponseWriter, r *http.Request, key string) {
	http.SetCookie(w, &http.Cookie{Name: draftSavedCookie, Value: key, Path: "/", MaxAge: 300,
		Secure: httputil.IsHTTPS(r), SameSite: http.SameSiteLaxMode})
}

// saveContentExtras saves what goes with a saved node: its SEO, its terms
// index (saveContentTaxonomy), the modules' settings (saveNodeSettings)
// and calls (AddNodeFormSaved) - in the caller's transaction (ctx).
func (c *DynamicContentController) saveContentExtras(ctx context.Context, r *http.Request, cf contentForm, nodeID int, f *form.DynamicForm, isNew bool) error {
	// The meta tags only from who may set them: the others' are kept.
	if f.SettingsAllowed {
		if err := c.nodeService.UpsertPageSEO(ctx, nodeID, f.MetaTitle, f.MetaDescription); err != nil {
			return fmt.Errorf("saving the node's SEO: %w", err)
		}
	}
	// Every classification is a taxonomy field: no terms besides them.
	if err := saveContentTaxonomy(ctx, c.taxonomyService, nodeID); err != nil {
		return err
	}
	// The modules' sections: in the form for "administer content" only.
	user, _ := currentNodeUser(c.BaseController, r)
	shown := c.authService.HasPermission(ctx, user, service.PermAdministerContent)
	node := entity.Node{ID: nodeID, Type: entity.ContentType(cf.def.MachineName)}
	if err := saveNodeSettings(ctx, node, r, shown, isNew); err != nil {
		return fmt.Errorf("saving the node's module settings: %w", err)
	}
	// A workflow's type: the transition submitted (NodeWorkflow.Apply).
	if wf := NodeWorkflowFor(ctx, cf.def.MachineName); wf != nil {
		if err := wf.Apply(ctx, node, user, r, isNew); err != nil {
			return fmt.Errorf("the workflow's transition: %w", err)
		}
	}
	if err := runNodeFormSaved(ctx, node, isNew); err != nil {
		return fmt.Errorf("the modules' calls of the saved node: %w", err)
	}
	return nil
}

// currentNodeUser - the logged-in user of a node form: the access
// middleware's (/create/{type}, /node/{id}/edit), else the session's.
func currentNodeUser(c *BaseController, r *http.Request) (*entity.User, bool) {
	if user, ok := middleware.GetUser(r); ok && user != nil {
		return user, true
	}
	if user, err := c.GetCurrentUser(r); err == nil && user != nil {
		return user, true
	}
	return nil, false
}

// parseContentForm reads the submitted form: nodeID 0 on create, the
// edited node's otherwise, existing the edited node (nil on create) -
// what the form doesn't resubmit (a file input, a hidden field) is
// carried forward from it.
func (c *DynamicContentController) parseContentForm(r *http.Request, cf contentForm, nodeID int, existing *entity.DynamicContent) (*form.DynamicForm, error) {
	fieldIDs := make([]int, len(cf.fields))
	for i, f := range cf.fields {
		fieldIDs[i] = f.ID
	}
	f, err := (&form.DynamicForm{}).ParseDynamicForm(r, fieldIDs)
	if err != nil {
		return nil, err
	}
	f.NodeID = nodeID
	// The publishing, the author, the date, the front page / sticky
	// flags, the address, the meta tags and the comments' mode only from
	// who may set them: the others' form hasn't them
	// (edit_node_settings.html), a request with them is ignored (a typed
	// address could take a View's or a term's page).
	f.SettingsAllowed = c.mayAdministerContent(r)
	if !f.SettingsAllowed {
		f.NodeFlagsShown = false
		f.CreatedByID, f.CreatedAt = 0, time.Time{}
		f.ScheduleShown, f.PublishOn, f.UnpublishOn = false, time.Time{}, time.Time{}
		f.ViewRolesShown, f.ViewRoles = false, nil
		f.Slug, f.PathAutoShown = "", false
		f.MetaTitle, f.MetaDescription = "", ""
		f.CommentMode = ""
	}
	// A workflow's type: its transitions publish the item (NodeWorkflow).
	if NodeWorkflowFor(r.Context(), cf.def.MachineName) != nil {
		f.WorkflowManaged = true
		f.ScheduleShown, f.PublishOn, f.UnpublishOn = false, time.Time{}, time.Time{}
	}
	var existingValues map[int]any
	if existing != nil {
		existingValues = existing.Values
	}
	// What the request gives the fields beyond their raw values (uploads,
	// rows, tags, formats, blocks); a hidden field keeps its saved value.
	if err := c.resolveFormFields(r, cf.def.MachineName, cf.fields, existingValues, f); err != nil {
		return nil, err
	}
	return f, nil
}

// validateContentForm - what to correct in f, by field; the slug free
// among the nodes.
func (c *DynamicContentController) validateContentForm(ctx context.Context, cf contentForm, f *form.DynamicForm) map[string]string {
	errs := f.Validate(ctx, cf.fields)
	c.validateSlugFree(ctx, f, errs)
	c.validateUserFields(ctx, cf.fields, f, errs)
	return errs
}

// validateWorkflow adds to errs why the workflow refuses the transition
// submitted (NodeWorkflow.Validate) for a workflow's type: node - the
// edited item, nil on create.
func (c *DynamicContentController) validateWorkflow(r *http.Request, cf contentForm, node *entity.Node, errs map[string]string) {
	wf := NodeWorkflowFor(r.Context(), cf.def.MachineName)
	if wf == nil || wf.Validate == nil {
		return
	}
	user, _ := currentNodeUser(c.BaseController, r)
	if msg := wf.Validate(r.Context(), cf.def.MachineName, node, user, r); msg != "" {
		errs["workflow"] = msg
	}
}

// emptyForm - the create form as it opens: the URL by the type's
// pattern, the type's front page / sticky flags, published unless the
// type's publishing is "unpublished" (not on moderation: who sees the
// options is a moderator).
func (cf contentForm) emptyForm() form.DynamicForm {
	status, _ := cf.def.DefaultPublishing.Flags()
	return form.DynamicForm{PathAuto: true, Promote: cf.def.DefaultPromote, Sticky: cf.def.DefaultSticky, Status: status}
}

// sentBack - what the form shows when sent back with errors: saved (the
// saved content; empty on create) with f's values, what was typed - not
// the saved ones. An image or file field's file sent and not saved yet is
// kept for the next submit (f.SentFiles, its multiple images' kept rows)
// where the field takes files from the media library - the user's own
// upload passes its check; elsewhere the saved files show and the field
// asks for the file again. A saved file ticked for deleting shows ticked
// (f.FileDeletes).
func sentBack(ctx context.Context, cf contentForm, f *form.DynamicForm, saved entity.DynamicContent) entity.DynamicContent {
	values, formats := dynamicValuesFromForm(f, cf.fields)
	for _, field := range cf.fields {
		if field.FieldType != entity.FieldTypeImage && field.FieldType != entity.FieldTypeFile {
			continue
		}
		before, sent := entity.FileIDs(saved.Values[field.ID]), entity.FileIDs(values[field.ID])
		var added, kept []int
		for _, id := range sent {
			if slices.Contains(before, id) {
				kept = append(kept, id)
			} else {
				added = append(added, id)
			}
		}
		if !field.Multiple() && len(sent) == 0 && len(before) > 0 {
			// Ticked for deleting: the saved file, ticked.
			values[field.ID] = saved.Values[field.ID]
			if f.FileDeletes == nil {
				f.FileDeletes = map[int]bool{}
			}
			f.FileDeletes[field.ID] = true
			continue
		}
		if len(added) == 0 {
			continue
		}
		if field.AllowsLibrary() {
			if !field.Multiple() {
				if f.SentFiles == nil {
					f.SentFiles = map[int]int{}
				}
				f.SentFiles[field.ID] = added[0]
			}
			continue
		}
		// The next submit couldn't keep the new file.
		if field.Multiple() {
			values[field.ID] = kept
		} else if v, ok := saved.Values[field.ID]; ok {
			values[field.ID] = v
		} else {
			delete(values, field.ID)
		}
		if f.FieldUploadErrors == nil {
			f.FieldUploadErrors = map[int]string{}
		}
		if _, ok := f.FieldUploadErrors[field.ID]; !ok {
			f.FieldUploadErrors[field.ID] = i18n.T(ctx, "Choose the file again: the form was sent back.")
		}
	}
	saved.Values, saved.TextFormats = values, formats
	return saved
}

// toEntity - the node f describes (without its type, slug and author:
// CreateSubmit's, applyEdit's).
func (cf contentForm) toEntity(f *form.DynamicForm) *entity.DynamicContent {
	values, textFormats := dynamicValuesFromForm(f, cf.fields)
	// Without the flags in the form: the type's defaults (a new node; an
	// edited one keeps its own, applyEdit).
	promote, sticky := cf.def.DefaultPromote, cf.def.DefaultSticky
	if f.NodeFlagsShown {
		promote, sticky = f.Promote, f.Sticky
	}
	// Without the right to set it: the type's publishing (a new node; an
	// edited one keeps its own, applyEdit).
	status, onModeration := f.Status, f.OnModeration
	switch {
	case f.WorkflowManaged:
		// Unpublished until the workflow's Apply says otherwise.
		status, onModeration = 0, 0
	case !f.SettingsAllowed:
		status, onModeration = cf.def.DefaultPublishing.Flags()
	}
	// Scheduled to be published later: not published until then.
	var publishOn, unpublishOn time.Time
	if f.ScheduleShown {
		publishOn, unpublishOn = f.PublishOn, f.UnpublishOn
		if publishOn.After(time.Now()) {
			status = 0
		}
	}
	return &entity.DynamicContent{
		Node: entity.Node{
			Title:        f.Title,
			Status:       status,
			OnModeration: onModeration,
			CommentMode:  entity.CommentMode(f.CommentMode),
			Promote:      promote,
			Sticky:       sticky,
			PublishOn:    publishOn,
			UnpublishOn:  unpublishOn,
			ViewRoles:    f.ViewRoles,
		},
		ContentType: cf.def,
		Fields:      cf.fields,
		Values:      values,
		TextFormats: textFormats,
	}
}

// applyEdit gives the edited node e what the form leaves as it was in
// existing: its ID, its path setting, the flags, the publishing and the
// schedule the form hasn't (or its user may not set), the author and the date unless
// set, the comment mode unless valid; an emptied slug keeps the current
// one (nodeSlug makes the final one).
func applyEdit(e, existing *entity.DynamicContent, f *form.DynamicForm) {
	e.Node.ID = existing.Node.ID
	// The node's own setting, unless the form changes it (nodeSlug).
	e.Node.PathAuto = existing.Node.PathAuto
	if !f.NodeFlagsShown {
		e.Node.Promote, e.Node.Sticky = existing.Node.Promote, existing.Node.Sticky
	}
	if !f.SettingsAllowed || f.WorkflowManaged {
		e.Node.Status, e.Node.OnModeration = existing.Node.Status, existing.Node.OnModeration
	}
	if !f.ScheduleShown {
		e.Node.PublishOn, e.Node.UnpublishOn = existing.Node.PublishOn, existing.Node.UnpublishOn
	}
	if !f.ViewRolesShown {
		e.Node.ViewRoles = existing.Node.ViewRoles
	}
	f.Slug = strings.Trim(f.Slug, "/ ")
	if f.Slug == "" {
		f.Slug = existing.Node.Slug
	}
	e.Node.Slug = f.Slug
	// Author/date overrides of who may set them (form.DynamicForm.
	// CreatedByID; cleared in parseContentForm otherwise).
	if f.CreatedByID > 0 {
		e.Node.CreatedBy = entity.User{ID: f.CreatedByID}
	} else {
		e.Node.CreatedBy = existing.Node.CreatedBy
	}
	if !f.CreatedAt.IsZero() {
		e.Node.CreatedAt = f.CreatedAt
	} else {
		e.Node.CreatedAt = existing.Node.CreatedAt
	}
	if !entity.IsValidCommentMode(f.CommentMode) {
		e.Node.CommentMode = existing.Node.CommentMode
	}
}

// pageData supplies the edit form's template data: the field schema, current
// values and term options per taxonomy field.
func (c *DynamicContentController) pageData(ctx context.Context, def entity.ContentTypeDef, fields entity.FieldInstanceDefs, f form.DynamicForm, e entity.DynamicContent) map[string]any {
	data := make(map[string]any)
	data["form"] = f
	// node - the saved node (edit form): Cancel goes back to it.
	data["node"] = e.Node
	data["errors"] = map[string]string{}
	data["contentType"] = def
	// timeZone - the zone the form's dates are in (the schedule).
	data["timeZone"] = sitetime.FromContext(ctx).String()
	data["fields"] = fields
	data["formSections"] = buildFormSections(fields)
	// What the fields' inputs show (values, terms, images…); the blocks
	// fields' blocks, each with what its inputs show.
	maps.Copy(data, c.fieldInputData(ctx, fields, e.Values, e.TextFormats, f))
	blocks := c.blockFieldForms(ctx, fields, e.Values, f)
	data["blockFields"] = blocks
	data["fieldEditorScripts"] = fieldEditorScripts(fields)
	// textFormats - the format dropdown options, fetched only if some field
	// (a block's too) uses text formats.
	for _, field := range append(slices.Clone(fields), blockSetFields(blocks)...) {
		if len(field.AllowedTextFormatIDs()) > 0 {
			if allFormats, tfErr := c.textFormatService.TextFormats(ctx); tfErr != nil {
				c.LogFailure(tfErr, "error listing text formats for edit form")
			} else {
				data["textFormats"] = allFormats
			}
			break
		}
	}
	// dc: the author/publishing settings read Node.CreatedBy/CreatedAt from
	// it.
	data["dc"] = e
	// defaultCommentMode: the comment mode select falls back to the type's
	// default when the form has none (create).
	data["defaultCommentMode"] = string(def.DefaultCommentMode)
	// viewRoleOptions - the roles a node's view may be given to.
	data["viewRoleOptions"] = c.viewRoleOptions(ctx)
	// previewLinkExpires - when the node's preview link stops working (none
	// while it has no working link).
	if e.Node.ID > 0 && c.previewLinks != nil {
		if link, ok, err := c.previewLinks.Active(ctx, e.Node.ID); err != nil {
			c.LogFailure(err, "loading the node's preview link", "nodeID", e.Node.ID)
		} else if ok {
			data["previewLinkExpires"] = link.ExpiresAt
		}
	}
	// moduleSettings: the modules' sections of the form (AddNodeSettings);
	// a new node has no ID yet.
	var node *entity.Node
	if e.Node.ID > 0 {
		node = &e.Node
	}
	data["moduleSettings"] = c.loadNodeSettings(ctx, def.MachineName, node, f.Submitted)

	// pathPattern - the type's URL pattern, next to "generate
	// automatically" (edit_node_settings.html).
	data["pathPattern"] = def.PathPattern
	if data["pathPattern"] == "" {
		data["pathPattern"] = service.DefaultNodePathPattern
	}

	// Taxonomy and reference fields need no per-type data: options come from
	// the loop above, widgets from edit_dynamic.html/edit_scripts.html.

	return data
}
