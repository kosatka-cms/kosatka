package admin

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// viewRows - one of a display's lists of rows (S a list of T: filters,
// sort criteria, fields) as its card on the View page edits them. A row
// is added, edited, reordered and deleted, the list overridden and
// reverted alike for all of them (the methods below); each list says how
// its rows are kept and what its form posts.
type viewRows[S ~[]T, T any] struct {
	// what - the rows in log messages; pathKey - the path value of a
	// row's ID.
	what, pathKey string
	// canEdit - whether the display's list is its own (overridden or
	// the Default's), so editable.
	canEdit func(view *entity.ViewDisplayDef) bool
	list    func(ctx context.Context, displayID int) (S, error)
	byID    func(ctx context.Context, id int) (*T, error)
	create  func(ctx context.Context, row T) (*T, error)
	update  func(ctx context.Context, row T) (*T, error)
	delete  func(ctx context.Context, id int) error
	// override, revert - the display's list made its own, given back to
	// its View's Default.
	override func(ctx context.Context, displayID int) error
	revert   func(ctx context.Context, displayID int) error
	// render - the card's body (the AJAX response of each change).
	render func(ctx context.Context, w io.Writer, view *entity.ViewDisplayDef) error
	// fromForm - the row the form posts for display displayID: old - the
	// row edited (its ID, place and what may not change kept), nil - a
	// new one; a code - the form's error.
	fromForm func(r *http.Request, displayID int, old *T) (row T, code string)
	// errorCode - the form's error code of the service's refusal; "" -
	// not one (a failure).
	errorCode func(err error) string
	// target - the field storage the row is of, or its builtin's name.
	target func(row T) (fieldStorageID *int, builtin string)
	// place - the row's ID, display and place in the list; placed - the
	// row at place order.
	place  func(row T) (id, displayID, order int)
	placed func(row T, order int) T
}

// viewPost - a POST to a display's page: its ID and the page's address;
// false - answered (a redirect).
func (c *Controller) viewPost(w http.ResponseWriter, r *http.Request) (int, string, bool) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/views", http.StatusSeeOther)
		return 0, "", false
	}
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/views?error=invalid", http.StatusSeeOther)
		return 0, "", false
	}
	return id, "/admin/config/views/" + strconv.Itoa(id), true
}

// editableView - the display id whose list is editable; false -
// answered: failCode when it can't be read.
func (v viewRows[S, T]) editableView(c *Controller, w http.ResponseWriter, r *http.Request, id int, base, failCode string) (*entity.ViewDisplayDef, bool) {
	view, err := c.viewService.ViewByID(r.Context(), id)
	if err != nil {
		c.LogFailure(err, "loading the View display", "viewID", id)
		c.jsonOrRedirectError(w, r, base, failCode)
		return nil, false
	}
	if !v.canEdit(view) {
		c.jsonOrRedirectError(w, r, base, "section-not-overridden")
		return nil, false
	}
	return view, true
}

// ownRow - the row of the request's path, one of display id's; false -
// answered.
func (v viewRows[S, T]) ownRow(c *Controller, w http.ResponseWriter, r *http.Request, id int, base string) (*T, bool) {
	rowID, ok := pathRowID(r, v.pathKey)
	if !ok {
		c.jsonOrRedirectError(w, r, base, "invalid")
		return nil, false
	}
	row, err := v.byID(r.Context(), rowID)
	if err != nil {
		c.LogFailure(err, "loading a View row", "rows", v.what, "id", rowID)
		c.jsonOrRedirectError(w, r, base, "invalid")
		return nil, false
	}
	if _, displayID, _ := v.place(*row); displayID != id {
		c.jsonOrRedirectError(w, r, base, "invalid")
		return nil, false
	}
	return row, true
}

// saveFailed answers a refused or failed save: the form's error code,
// else failCode (logged).
func (v viewRows[S, T]) saveFailed(c *Controller, w http.ResponseWriter, r *http.Request, base string, err error, failCode string, args ...any) {
	if code := v.errorCode(err); code != "" {
		c.jsonOrRedirectError(w, r, base, code)
		return
	}
	c.LogFailure(err, "error saving a View row", append([]any{"rows", v.what}, args...)...)
	c.jsonOrRedirectError(w, r, base, failCode)
}

// respond answers a change with the card's body (jsonOrRedirectSuccess).
func (v viewRows[S, T]) respond(c *Controller, w http.ResponseWriter, r *http.Request, base string, view *entity.ViewDisplayDef) {
	ctx := r.Context()
	c.jsonOrRedirectSuccess(w, r, base, func(w io.Writer) error {
		return v.render(ctx, w, view)
	})
}

// add handles POST /admin/config/views/{id}/<rows>: a new row, last.
func (v viewRows[S, T]) add(c *Controller, w http.ResponseWriter, r *http.Request) {
	id, base, ok := c.viewPost(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	row, code := v.fromForm(r, id, nil)
	if code != "" {
		c.jsonOrRedirectError(w, r, base, code)
		return
	}
	view, ok := v.editableView(c, w, r, id, base, "create")
	if !ok {
		return
	}
	existing, err := v.list(ctx, id)
	if err != nil {
		c.LogFailure(err, "error loading the View rows before adding one", "rows", v.what, "viewID", id)
		c.jsonOrRedirectError(w, r, base, "create")
		return
	}
	if _, err := v.create(ctx, v.placed(row, len(existing)*10)); err != nil {
		v.saveFailed(c, w, r, base, err, "create", "viewID", id)
		return
	}
	v.respond(c, w, r, base, view)
}

// edit handles POST /admin/config/views/{id}/<rows>/{<row>_id}: the row
// edited in place (its place kept; only reorder changes it).
func (v viewRows[S, T]) edit(c *Controller, w http.ResponseWriter, r *http.Request) {
	id, base, ok := c.viewPost(w, r)
	if !ok {
		return
	}
	old, ok := v.ownRow(c, w, r, id, base)
	if !ok {
		return
	}
	view, ok := v.editableView(c, w, r, id, base, "update")
	if !ok {
		return
	}
	row, code := v.fromForm(r, id, old)
	if code != "" {
		c.jsonOrRedirectError(w, r, base, code)
		return
	}
	if _, err := v.update(r.Context(), row); err != nil {
		rowID, _, _ := v.place(row)
		v.saveFailed(c, w, r, base, err, "update", "id", rowID)
		return
	}
	v.respond(c, w, r, base, view)
}

// reorder handles POST /admin/config/views/{id}/<rows>/reorder (JSON,
// viewReorderRequest): one update per row, its place index*10 (add's
// step). The order of sort criteria is their precedence.
func (v viewRows[S, T]) reorder(c *Controller, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.JSONError(w, "invalid", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		httputil.JSONError(w, "invalid", http.StatusBadRequest)
		return
	}
	view, err := c.viewService.ViewByID(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading the View display", "viewID", id)
		httputil.JSONError(w, "invalid", http.StatusBadRequest)
		return
	}
	if !v.canEdit(view) {
		httputil.JSONError(w, "section-not-overridden", http.StatusBadRequest)
		return
	}
	var req viewReorderRequest
	if err := httputil.DecodeJSON(r, &req); err != nil || len(req.Order) == 0 {
		httputil.JSONError(w, "empty", http.StatusBadRequest)
		return
	}
	existing, err := v.list(ctx, id)
	if err != nil {
		c.LogFailure(err, "error loading the View rows for reorder", "rows", v.what, "viewID", id)
		httputil.JSONError(w, "update", http.StatusInternalServerError)
		return
	}
	byID := make(map[int]T, len(existing))
	for _, row := range existing {
		rowID, _, _ := v.place(row)
		byID[rowID] = row
	}
	for i, rowID := range req.Order {
		row, ok := byID[rowID]
		if !ok {
			continue
		}
		if _, err := v.update(ctx, v.placed(row, i*10)); err != nil {
			c.LogFailure(err, "error saving the View rows' order", "rows", v.what, "id", rowID)
			httputil.JSONError(w, "update", http.StatusInternalServerError)
			return
		}
	}
	c.refreshViewBlocks()
	var buf bytes.Buffer
	if err := v.render(ctx, &buf, view); err != nil {
		c.LogFailure(err, "error rendering the View rows after reorder", "rows", v.what)
		httputil.JSON(w, map[string]any{"success": true}, http.StatusOK)
		return
	}
	httputil.JSON(w, map[string]any{"success": true, "html": buf.String()}, http.StatusOK)
}

// remove handles POST /admin/config/views/{id}/<rows>/{<row>_id}/delete
// (the AJAX UI confirms with a modal, admin_views.js).
func (v viewRows[S, T]) remove(c *Controller, w http.ResponseWriter, r *http.Request) {
	id, base, ok := c.viewPost(w, r)
	if !ok {
		return
	}
	view, ok := v.editableView(c, w, r, id, base, "section-not-overridden")
	if !ok {
		return
	}
	row, ok := v.ownRow(c, w, r, id, base)
	if !ok {
		return
	}
	rowID, _, _ := v.place(*row)
	if err := v.delete(r.Context(), rowID); err != nil {
		c.LogFailure(err, "error deleting a View row", "rows", v.what, "id", rowID)
		c.jsonOrRedirectError(w, r, base, "delete")
		return
	}
	v.respond(c, w, r, base, view)
}

// deletePage renders GET /admin/config/views/{id}/<rows>/{<row>_id}/delete -
// remove's page without JS (the AJAX UI confirms with a modal,
// admin_views.js): template with the display, the row and its target's
// machine name; titles - the page's heading and browser title by the
// display's label.
func (v viewRows[S, T]) deletePage(c *Controller, w http.ResponseWriter, r *http.Request, template string, titles func(ctx context.Context, label string) (heading, title string)) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	view, err := c.viewService.ViewByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the View display", "viewID", id)
		return
	}
	rowID, ok := pathRowID(r, v.pathKey)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	row, err := v.byID(ctx, rowID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading a View row", "rows", v.what, "id", rowID)
		return
	}
	if _, displayID, _ := v.place(*row); displayID != id {
		c.RenderNotFound(w, r)
		return
	}
	fieldStorageID, target := v.target(*row)
	if fieldStorageID != nil {
		if fs, err := c.contentTypeService.FieldStorageByID(ctx, *fieldStorageID); err == nil {
			target = fs.MachineName
		} else {
			c.LogFailure(err, "loading the field storage", "fieldStorageID", *fieldStorageID)
		}
	}
	heading, title := titles(ctx, view.Label)
	data := c.getAdminData(r)
	data["title"] = heading
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["view"] = view
	data["row"] = row
	data["target"] = target
	c.RenderOrFail(w, r, template, data)
}

// fork handles POST /admin/config/views/{id}/<rows>/override and
// …/revert (code - the one): the display's list made its own (with
// change override) or given back to its View's Default (revert).
func (v viewRows[S, T]) fork(c *Controller, w http.ResponseWriter, r *http.Request, change func(ctx context.Context, displayID int) error, code string) {
	id, base, ok := c.viewPost(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if err := change(ctx, id); err != nil {
		c.LogFailure(err, "error forking the View rows", "rows", v.what, "change", code, "id", id)
		c.jsonOrRedirectError(w, r, base, code)
		return
	}
	view, err := c.viewService.ViewByID(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading the View display", "viewID", id)
		c.jsonOrRedirectError(w, r, base, code)
		return
	}
	v.respond(c, w, r, base, view)
}

// pathRowID - a row's ID, the path value key (the routes of a row carry
// the display's ID {id} and the row's).
func pathRowID(r *http.Request, key string) (int, bool) {
	id, err := strconv.Atoi(r.PathValue(key))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// parseRowTarget - a sort criterion's or field's target as the picker
// sends it: "field:<field storage>" or "builtin:<name>"; false - neither.
func parseRowTarget(target string) (fieldStorageID *int, builtin string, ok bool) {
	switch {
	case strings.HasPrefix(target, "field:"):
		id, err := strconv.Atoi(strings.TrimPrefix(target, "field:"))
		if err != nil || id <= 0 {
			return nil, "", false
		}
		return &id, "", true
	case strings.HasPrefix(target, "builtin:"):
		return nil, strings.TrimPrefix(target, "builtin:"), true
	}
	return nil, "", false
}
