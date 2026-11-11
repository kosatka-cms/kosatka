package admin

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// ViewsPerPage - page size of the Views list, like
// ContentPerPage/PeoplePerPage (admin/content_list.go,
// admin/people.go).
const ViewsPerPage = 25

// wantsJSON reports whether the request comes from the single-page Views
// admin JS (the only AJAX-driven admin page, see admin/views/tabs.html),
// so mutation handlers answer with JSON and a re-rendered fragment instead
// of a redirect.
func (c *Controller) wantsJSON(r *http.Request) bool {
	return r.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// jsonOrRedirectError answers a failed mutation with a redirect carrying
// ?error=<code> (non-JS submit) or, when wantsJSON, JSON {"error": code} -
// the same code either way.
func (c *Controller) jsonOrRedirectError(w http.ResponseWriter, r *http.Request, redirectURL, code string) {
	if c.wantsJSON(r) {
		httputil.JSONError(w, code, http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, redirectURL+"?error="+code, http.StatusSeeOther)
}

// jsonOrRedirectSuccess answers a successful mutation with a redirect
// carrying ?success=1 or, when wantsJSON, JSON {"success": true, "html":
// "<fragment>"}; the JS swaps the fragment into the affected card. The
// View blocks' cache is rebuilt (CacheService.ForceUpdate): a View never
// published shows the change at once (the site shows its live version),
// a published one its published version, new on publishing or
// discarding.
func (c *Controller) jsonOrRedirectSuccess(w http.ResponseWriter, r *http.Request, redirectURL string, renderFragment func(io.Writer) error) {
	c.refreshViewBlocks()
	if !c.wantsJSON(r) {
		http.Redirect(w, r, redirectURL+"?success=1", http.StatusSeeOther)
		return
	}
	var buf bytes.Buffer
	if err := renderFragment(&buf); err != nil {
		c.LogFailure(err, "error rendering view ajax fragment")
		httputil.JSON(w, map[string]any{"success": true}, http.StatusOK)
		return
	}
	httputil.JSON(w, map[string]any{"success": true, "html": buf.String()}, http.StatusOK)
}

// refreshViewBlocks rebuilds the cached View blocks after a View changed
// (asynchronous, one rebuild for many calls).
func (c *Controller) refreshViewBlocks() {
	if c.cacheService != nil {
		c.cacheService.ForceUpdate()
	}
}

// ViewList renders GET /admin/config/views - the list of Views.
func (c *Controller) ViewList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	views, err := c.viewService.Views(ctx)
	if err != nil {
		c.LogFailure(err, "error listing views")
		httputil.InternalServerError(w)
		return
	}

	// Pagination slices the already-loaded list in memory (Views() has no
	// LIMIT/OFFSET; the list is small).
	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p >= 1 {
		page = p
	}
	pager := pagination.New(page, len(views), ViewsPerPage, "/admin/config/views", r.URL.Query())
	end := min(pager.Offset+pager.ItemsPerPage, len(views))
	views = views[pager.Offset:end]

	// A grouped row is its View's Default; the " - Default" suffix is dropped
	// from the label in the list (local copy only, IDs unchanged).
	// displaysByRow - the non-Default displays of each grouped row's View,
	// keyed by the row ID and shown as "name · type" under the label. One query
	// per grouped row is fine at this list's size.
	displaysByRow := make(map[int]entity.ViewDisplayDefs, len(views))
	for i, v := range views {
		if !v.HasGroup() {
			continue
		}
		views[i].Label = strings.TrimSuffix(v.Label, " - Default")
		siblings, err := c.viewService.DisplaysOfView(ctx, v.ViewIDValue())
		if err != nil {
			c.LogFailure(err, "error loading group displays for views list row", "viewID", v.ViewIDValue())
			continue
		}
		displaysByRow[v.ID] = siblings
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Views (lists)")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["views"] = views
	data["displaysByRow"] = displaysByRow
	data["pagination"] = pager
	data["success"] = r.URL.Query().Get("success") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/views/index.html", data)
}

// parseViewDisplayType maps a form value to a known display type; unknown
// values fall back to page.
func parseViewDisplayType(v string) entity.ViewDisplayType {
	switch t := entity.ViewDisplayType(v); t {
	case entity.ViewDisplayTypeBlock, entity.ViewDisplayTypeAttachment, entity.ViewDisplayTypeTerm, entity.ViewDisplayTypeUserTab:
		return t
	default:
		return entity.ViewDisplayTypePage
	}
}

// ViewCreate handles POST /admin/config/views. Collects only what's needed
// to create the row, then redirects to ViewEdit for the rest. An empty
// url_path is generated from the label (resolveSlug) and editable later.
func (c *Controller) ViewCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/views", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	currentUser, err := c.GetCurrentUser(r)
	if err != nil || currentUser == nil {
		http.Redirect(w, r, "/admin/config/views?error=auth", http.StatusSeeOther)
		return
	}

	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		http.Redirect(w, r, "/admin/config/views?error=empty", http.StatusSeeOther)
		return
	}
	displayType := parseViewDisplayType(r.FormValue("display_type"))
	// A block has no URL, so url_path isn't generated; a page's is generated
	// from the label when left empty.
	var urlPath string
	if displayType.HasURL() {
		urlPath = resolveSlug(r.FormValue("url_path"), label)
	}
	// baseEntity is immutable after creation (see entity.ViewBaseEntity).
	baseEntity := entity.ViewBaseEntity(r.FormValue("base_entity"))
	if baseEntity != entity.ViewBaseEntityComment {
		baseEntity = entity.ViewBaseEntityContent
	}

	// MachineName is left unset: CreateView assigns "view_"+id once the ID is
	// known (see createDisplay).
	created, err := c.viewService.CreateView(ctx, entity.ViewDisplayDef{
		Label:           label,
		BaseEntity:      baseEntity,
		URLPath:         urlPath,
		DisplayType:     displayType,
		AttachShowTitle: true,
		DisplayStyle:    entity.ViewDisplayStyleGrid,
		GridColumns:     5,
		// Carousel* are seeded with sane defaults even though they matter only for
		// the carousel style, so the form shows sensible values when switched.
		CarouselSlidesPerView:  1,
		CarouselShowNavigation: true,
		CarouselShowPagination: true,
		CarouselEffect:         "slide",
		// A block shows its first items; it pages when asked.
		ShowPagination: displayType != entity.ViewDisplayTypeBlock,
		PageSize:       30,
		// Unpublished by default, like CloneDisplay/AddDisplay.
		Status:    0,
		CreatedBy: currentUser.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, views.ErrDuplicate):
			http.Redirect(w, r, "/admin/config/views?error=duplicate", http.StatusSeeOther)
		default:
			c.LogFailure(err, "error creating view")
			http.Redirect(w, r, "/admin/config/views?error=create", http.StatusSeeOther)
		}
		return
	}

	http.Redirect(w, r, "/admin/config/views/"+strconv.Itoa(created.ID), http.StatusSeeOther)
}

// ViewDeleteConfirm renders GET /admin/config/views/{id}/delete - a
// confirmation page.
func (c *Controller) ViewDeleteConfirm(w http.ResponseWriter, r *http.Request) {
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

	filters, err := c.viewService.FiltersByViewID(ctx, id)
	if err != nil {
		c.LogFailure(err, "error counting view filters for delete page", "id", id)
	}
	fields, err := c.viewService.FieldsByViewID(ctx, id)
	if err != nil {
		c.LogFailure(err, "error counting view fields for delete page", "id", id)
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Delete view \"@label\"", "@label", view.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", i18n.T(ctx, "Delete view @label", "@label", view.Label))}
	data["view"] = view
	data["filterCount"] = len(filters)
	data["fieldCount"] = len(fields)

	c.RenderOrFail(w, r, "admin/views/delete.html", data)
}

// ViewDelete handles POST /admin/config/views/{id}/delete.
func (c *Controller) ViewDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/views", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/views?error=invalid", http.StatusSeeOther)
		return
	}
	view, err := c.viewService.ViewByID(ctx, id)
	if err != nil {
		c.LogFailure(err, "loading the View display", "viewID", id)
		http.Redirect(w, r, "/admin/config/views?error=invalid", http.StatusSeeOther)
		return
	}
	// Default displays aren't reachable via the UI, but guard anyway:
	// deleting one would orphan every sibling inheriting from it.
	if view.IsDefault {
		http.Redirect(w, r, "/admin/config/views?error=cannot-delete-default", http.StatusSeeOther)
		return
	}
	if err := c.viewService.DeleteView(ctx, id); err != nil {
		c.LogFailure(err, "error deleting view", "id", id)
		http.Redirect(w, r, "/admin/config/views?error=delete", http.StatusSeeOther)
		return
	}
	c.refreshViewBlocks()
	// After deleting a grouped display, stay within the View on its Default
	// (which always exists and can't be deleted); a lone display returns to
	// the list.
	if view.ViewID != nil {
		def, err := c.viewService.DefaultDisplayOfView(ctx, *view.ViewID)
		if err == nil {
			http.Redirect(w, r, "/admin/config/views/"+strconv.Itoa(def.ID)+"?success=1", http.StatusSeeOther)
			return
		}
		c.LogFailure(err, "loading the View's default display", "viewID", *view.ViewID)
	}
	http.Redirect(w, r, "/admin/config/views?success=1", http.StatusSeeOther)
}

// ViewGroupDeleteConfirm renders GET /admin/config/view-groups/{id}/delete -
// id is the View container's (entity.View) ID. Unlike ViewDelete (one
// display), this removes the whole View: the container, its Default and
// every display, with their filters, sorts and fields.
func (c *Controller) ViewGroupDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	container, err := c.viewService.ViewContainerByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the View", "viewID", id)
		return
	}
	def, err := c.viewService.DefaultDisplayOfView(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the View's default display", "id", id)
		return
	}
	displays, err := c.viewService.DisplaysOfView(ctx, id)
	if err != nil {
		c.LogFailure(err, "error loading displays for group-delete page", "id", id)
	}

	// filterCount/fieldCount/sortCount - summed across the Default and every
	// display: what will actually be deleted.
	displayIDs := make([]int, 0, len(displays)+1)
	displayIDs = append(displayIDs, def.ID)
	for _, d := range displays {
		displayIDs = append(displayIDs, d.ID)
	}
	var filterCount, sortCount, fieldCount int
	for _, displayID := range displayIDs {
		if filters, err := c.viewService.FiltersByViewID(ctx, displayID); err == nil {
			filterCount += len(filters)
		} else {
			c.LogFailure(err, "loading the View display's filters", "viewID", displayID)
		}
		if sorts, err := c.viewService.SortsByViewID(ctx, displayID); err == nil {
			sortCount += len(sorts)
		} else {
			c.LogFailure(err, "loading the View display's sorts", "viewID", displayID)
		}
		if fields, err := c.viewService.FieldsByViewID(ctx, displayID); err == nil {
			fieldCount += len(fields)
		} else {
			c.LogFailure(err, "loading the View display's fields", "viewID", displayID)
		}
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Delete view \"@label\"", "@label", container.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", i18n.T(ctx, "Delete view @label", "@label", container.Label))}
	data["container"] = container
	data["defaultDisplay"] = def
	data["displays"] = displays
	data["filterCount"] = filterCount
	data["sortCount"] = sortCount
	data["fieldCount"] = fieldCount

	c.RenderOrFail(w, r, "admin/views/group_delete.html", data)
}

// ViewGroupDelete handles POST /admin/config/view-groups/{id}/delete -
// id is the View container's ID. One DeleteViewContainer call; ON DELETE
// CASCADE removes the rest.
func (c *Controller) ViewGroupDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/views", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/views?error=invalid", http.StatusSeeOther)
		return
	}
	if _, err := c.viewService.ViewContainerByID(ctx, id); err != nil {
		c.LogFailure(err, "loading the View", "viewID", id)
		http.Redirect(w, r, "/admin/config/views?error=invalid", http.StatusSeeOther)
		return
	}
	if err := c.viewService.DeleteViewContainer(ctx, id); err != nil {
		c.LogFailure(err, "error deleting view container", "id", id)
		http.Redirect(w, r, "/admin/config/views?error=delete", http.StatusSeeOther)
		return
	}
	c.refreshViewBlocks()
	http.Redirect(w, r, "/admin/config/views?success=1", http.StatusSeeOther)
}

// --- Grouped displays ---

// ViewClone handles POST /admin/config/views/{id}/clone - the only way to
// create a grouped display (see views.DefaultService.CloneDisplay).
// Redirects to the new sibling's edit page.
func (c *Controller) ViewClone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/views", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/views?error=invalid", http.StatusSeeOther)
		return
	}
	currentUser, err := c.GetCurrentUser(r)
	if err != nil || currentUser == nil {
		http.Redirect(w, r, "/admin/config/views?error=auth", http.StatusSeeOther)
		return
	}
	clone, err := c.viewService.CloneDisplay(ctx, id, currentUser.ID)
	if err != nil {
		switch {
		case errors.Is(err, views.ErrCannotEditDefault):
			http.Redirect(w, r, "/admin/config/views?error=cannot-clone-default", http.StatusSeeOther)
		default:
			c.LogFailure(err, "error cloning view", "id", id)
			http.Redirect(w, r, "/admin/config/views/"+strconv.Itoa(id)+"?error=clone", http.StatusSeeOther)
		}
		return
	}
	http.Redirect(w, r, "/admin/config/views/"+strconv.Itoa(clone.ID), http.StatusSeeOther)
}

// ViewAddDisplay handles POST /admin/config/views/{id}/add-display - the
// "Add display" button (see views.DefaultService.AddDisplay). {id} is the
// display it was clicked from (any display in the group, including
// Default); the new sibling joins that display's View.
func (c *Controller) ViewAddDisplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/views", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/views?error=invalid", http.StatusSeeOther)
		return
	}
	currentUser, err := c.GetCurrentUser(r)
	if err != nil || currentUser == nil {
		http.Redirect(w, r, "/admin/config/views?error=auth", http.StatusSeeOther)
		return
	}
	view, err := c.viewService.ViewByID(ctx, id)
	if err != nil || view.ViewID == nil {
		c.LogFailure(err, "loading the View display", "viewID", id)
		http.Redirect(w, r, "/admin/config/views?error=invalid", http.StatusSeeOther)
		return
	}
	created, err := c.viewService.AddDisplay(ctx, *view.ViewID, currentUser.ID)
	if err != nil {
		c.LogFailure(err, "error adding display", "viewID", *view.ViewID)
		http.Redirect(w, r, "/admin/config/views/"+strconv.Itoa(id)+"?error=create", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/views/"+strconv.Itoa(created.ID), http.StatusSeeOther)
}

// viewReorderRequest - JSON body of viewRows.reorder:
// the row IDs in their new order. AJAX-only (dragging requires JS anyway).
type viewReorderRequest struct {
	Order []int `json:"order"`
}
