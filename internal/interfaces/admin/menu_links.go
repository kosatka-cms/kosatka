package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// menuLinkRow - one row of a menu's tree flattened in pre-order: Depth
// drives indentation; DescendantIDs are excluded from the row's "Parent"
// select (picking a descendant as parent would create a cycle).
type menuLinkRow struct {
	entity.MenuLinkDef
	Depth         int
	ResolvedURL   string
	URLOK         bool
	DescendantIDs map[int]bool
	// IndentedLabel - Title prefixed with "- " per Depth level, for the
	// "Parent" select options.
	IndentedLabel string
	// ParentIDValue - ParentID dereferenced (0 = top level), so the template
	// can compare it with another row's plain int ID.
	ParentIDValue int
}

// buildMenuLinkRows walks links (already weight-sorted per parent by
// MenuLinksByMenuID) depth-first. A row's descendants are exactly the
// rows immediately following it with a greater Depth, up to the next row
// at Depth <= its own.
func buildMenuLinkRows(ctx context.Context, links entity.MenuLinkDefs, resolve func(context.Context, entity.MenuLinkDef) (string, bool)) []menuLinkRow {
	children := make(map[int]entity.MenuLinkDefs)
	for _, l := range links {
		parent := 0
		if l.ParentID != nil {
			parent = *l.ParentID
		}
		children[parent] = append(children[parent], l)
	}

	var rows []menuLinkRow
	var walk func(parentID, depth int)
	walk = func(parentID, depth int) {
		for _, l := range children[parentID] {
			url, ok := resolve(ctx, l)
			rows = append(rows, menuLinkRow{
				MenuLinkDef:   l,
				Depth:         depth,
				ResolvedURL:   url,
				URLOK:         ok,
				IndentedLabel: strings.Repeat("- ", depth) + l.Title,
				ParentIDValue: parentID,
			})
			walk(l.ID, depth+1)
		}
	}
	walk(0, 0)

	for i := range rows {
		descendants := make(map[int]bool)
		for j := i + 1; j < len(rows) && rows[j].Depth > rows[i].Depth; j++ {
			descendants[rows[j].ID] = true
		}
		rows[i].DescendantIDs = descendants
	}
	return rows
}

// MenuLinksList renders GET /admin/config/menus/{id}/links - the menu's
// drag-and-drop tree plus a quick-add form for a new top-level link.
// Nesting a new link is a separate step (the row's "Parent" select or drag).
func (c *Controller) MenuLinksList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	menu, err := c.menuService.MenuByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the menu", "id", id)
		return
	}
	links, err := c.menuService.MenuLinksByMenuID(ctx, id)
	if err != nil {
		c.LogFailure(err, "error listing menu links", "menuID", id)
		httputil.InternalServerError(w)
		return
	}
	views, err := c.viewService.PublicPageViews(ctx)
	if err != nil {
		c.LogFailure(err, "error listing views for menu link picker")
	}
	views = pickableViews(views, 0)
	vocabularies, err := c.taxonomyService.Vocabularies(ctx)
	if err != nil {
		c.LogFailure(err, "error listing vocabularies for menu link picker")
	}

	rows := buildMenuLinkRows(ctx, links, c.menuService.ResolveMenuLinkURL)

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Menu links \"@name\"", "@name", menu.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Menu links @name - Admin panel", "@name", menu.Label)}
	data["menu"] = menu
	data["rows"] = rows
	data["views"] = views
	data["vocabularies"] = vocabularies
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"

	c.RenderOrFail(w, r, "admin/menus/links.html", data)
}

// menuLinkFromForm parses the title/link-type/target fields shared by
// the quick-add form and the per-link edit form.
func menuLinkFromForm(r *http.Request) (title string, linkType entity.MenuLinkType, linkURL string, nodeID, termID, viewID *int, ok bool) {
	title = strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		return "", "", "", nil, nil, nil, false
	}

	switch entity.MenuLinkType(r.FormValue("link_type")) {
	case entity.MenuLinkNode:
		linkType = entity.MenuLinkNode
		if v, err := strconv.Atoi(r.FormValue("link_node_id")); err == nil && v > 0 {
			nodeID = &v
		}
	case entity.MenuLinkTerm:
		linkType = entity.MenuLinkTerm
		if v, err := strconv.Atoi(r.FormValue("link_term_id")); err == nil && v > 0 {
			termID = &v
		}
	case entity.MenuLinkView:
		linkType = entity.MenuLinkView
		if v, err := strconv.Atoi(r.FormValue("link_view_id")); err == nil && v > 0 {
			viewID = &v
		}
	default:
		linkType = entity.MenuLinkURL
		linkURL = strings.TrimSpace(r.FormValue("link_url"))
	}
	return title, linkType, linkURL, nodeID, termID, viewID, true
}

// MenuLinkCreate handles POST /admin/config/menus/{id}/links - always
// added at the end of the top level (max weight + 10); see MenuLinksList.
func (c *Controller) MenuLinkCreate(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	redirectBase := fmt.Sprintf("/admin/config/menus/%d/links", id)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, redirectBase, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	title, linkType, linkURL, nodeID, termID, viewID, ok := menuLinkFromForm(r)
	if !ok {
		http.Redirect(w, r, redirectBase+"?error=empty", http.StatusSeeOther)
		return
	}

	existing, err := c.menuService.MenuLinksByMenuID(ctx, id)
	if err != nil {
		c.LogFailure(err, "error listing menu links before create", "menuID", id)
		httputil.InternalServerError(w)
		return
	}
	weight := 0
	for _, l := range existing {
		if l.ParentID == nil && l.Weight >= weight {
			weight = l.Weight + 10
		}
	}

	_, err = c.menuService.CreateMenuLink(ctx, entity.MenuLinkDef{
		MenuID:     id,
		Title:      title,
		LinkType:   linkType,
		LinkURL:    linkURL,
		LinkNodeID: nodeID,
		LinkTermID: termID,
		LinkViewID: viewID,
		Weight:     weight,
		Enabled:    true,
	})
	if err != nil {
		c.LogFailure(err, "error creating menu link", "menuID", id)
		http.Redirect(w, r, redirectBase+"?error=create", http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, redirectBase+"?success=1", http.StatusSeeOther)
}

// MenuLinkEdit renders GET
// /admin/config/menus/{id}/links/{link_id}/edit.
func (c *Controller) MenuLinkEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	menuID, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	linkID, err := strconv.Atoi(r.PathValue("link_id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	menu, err := c.menuService.MenuByID(ctx, menuID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the menu", "menuID", menuID)
		return
	}
	link, err := c.menuService.MenuLinkByID(ctx, linkID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the menu link", "linkID", linkID)
		return
	}
	if link.MenuID != menuID {
		c.RenderNotFound(w, r)
		return
	}
	views, err := c.viewService.PublicPageViews(ctx)
	if err != nil {
		c.LogFailure(err, "error listing views for menu link picker")
	}
	views = pickableViews(views, link.LinkViewIDValue())
	vocabularies, err := c.taxonomyService.Vocabularies(ctx)
	if err != nil {
		c.LogFailure(err, "error listing vocabularies for menu link picker")
	}
	url, ok := c.menuService.ResolveMenuLinkURL(ctx, *link)

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Menu link \"@title\"", "@title", link.Title)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Menu link @title - Admin panel", "@title", link.Title)}
	data["menu"] = menu
	data["link"] = link
	data["resolvedURL"] = url
	data["urlOK"] = ok
	data["views"] = views
	data["vocabularies"] = vocabularies
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"
	c.AddFormSectionsData(ctx, data, interfaces.FormMenuLink, strconv.Itoa(linkID))

	c.RenderOrFail(w, r, "admin/menus/link_edit.html", data)
}

// MenuLinkUpdate handles POST /admin/config/menus/{id}/links/{link_id}.
// Weight/ParentID are deliberately untouched: they are set only via
// MenuLinksList's drag-and-drop/"Parent" select.
func (c *Controller) MenuLinkUpdate(w http.ResponseWriter, r *http.Request) {
	menuID, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	linkID, err := strconv.Atoi(r.PathValue("link_id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	editRedirect := fmt.Sprintf("/admin/config/menus/%d/links/%d/edit", menuID, linkID)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, editRedirect, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	existing, err := c.menuService.MenuLinkByID(ctx, linkID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the menu link", "linkID", linkID)
		return
	}
	if existing.MenuID != menuID {
		c.RenderNotFound(w, r)
		return
	}

	title, linkType, linkURL, nodeID, termID, viewID, ok := menuLinkFromForm(r)
	if !ok {
		http.Redirect(w, r, editRedirect+"?error=empty", http.StatusSeeOther)
		return
	}

	_, err = c.menuService.UpdateMenuLink(ctx, entity.MenuLinkDef{
		ID:         linkID,
		MenuID:     menuID,
		ParentID:   existing.ParentID,
		Title:      title,
		LinkType:   linkType,
		LinkURL:    linkURL,
		LinkNodeID: nodeID,
		LinkTermID: termID,
		LinkViewID: viewID,
		Weight:     existing.Weight,
		Enabled:    r.FormValue("enabled") == "1",
	})
	if err == nil {
		err = interfaces.SaveFormSections(ctx, interfaces.FormMenuLink, strconv.Itoa(linkID), r)
	}
	if err != nil {
		c.LogFailure(err, "error updating menu link", "id", linkID)
		http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, editRedirect+"?success=1", http.StatusSeeOther)
}

// MenuLinkDeleteConfirm renders GET
// /admin/config/menus/{id}/links/{link_id}/delete.
func (c *Controller) MenuLinkDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	menuID, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	linkID, err := strconv.Atoi(r.PathValue("link_id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	menu, err := c.menuService.MenuByID(ctx, menuID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the menu", "menuID", menuID)
		return
	}
	link, err := c.menuService.MenuLinkByID(ctx, linkID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the menu link", "linkID", linkID)
		return
	}
	if link.MenuID != menuID {
		c.RenderNotFound(w, r)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Delete menu link \"@title\"", "@title", link.Title)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Delete menu link @title - Admin panel", "@title", link.Title)}
	data["menu"] = menu
	data["link"] = link

	c.RenderOrFail(w, r, "admin/menus/link_delete.html", data)
}

// MenuLinkDelete handles POST
// /admin/config/menus/{id}/links/{link_id}/delete - the link's children
// are reparented (see entity.MenuRepository.DeleteMenuLink), not deleted.
func (c *Controller) MenuLinkDelete(w http.ResponseWriter, r *http.Request) {
	menuID, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	linkID, err := strconv.Atoi(r.PathValue("link_id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	redirectBase := fmt.Sprintf("/admin/config/menus/%d/links", menuID)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, redirectBase, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	if err := c.menuService.DeleteMenuLink(ctx, linkID); err != nil {
		c.LogFailure(err, "error deleting menu link", "id", linkID)
		http.Redirect(w, r, redirectBase+"?error=delete", http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, redirectBase+"?deleted=1", http.StatusSeeOther)
}

// MenuLinksReorder handles POST /admin/config/menus/{id}/links/reorder -
// bulk Weight/ParentID update from the drag-and-drop tree: "order" is a
// CSV of IDs plus a "parent_<id>" value per row (each row's parent select,
// kept in sync by admin.js's tree-sortable).
func (c *Controller) MenuLinksReorder(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	redirectBase := fmt.Sprintf("/admin/config/menus/%d/links", id)
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

	updates := make([]entity.MenuLinkOrderUpdate, 0, len(order))
	for i, idStr := range order {
		linkID, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			continue
		}
		var parentID *int
		if p := r.FormValue(fmt.Sprintf("parent_%d", linkID)); p != "" {
			if pv, err := strconv.Atoi(p); err == nil && pv > 0 {
				parentID = &pv
			}
		}
		updates = append(updates, entity.MenuLinkOrderUpdate{ID: linkID, ParentID: parentID, Weight: i * 10})
	}

	if err := c.menuService.ReorderMenuLinks(ctx, updates); err != nil {
		c.LogFailure(err, "error reordering menu links", "menuID", id)
		http.Redirect(w, r, redirectBase+"?error=reorder", http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, redirectBase+"?success=1", http.StatusSeeOther)
}

// pickableViews - the Views pages a menu link may point to: the enabled
// ones, and current (the link's own, 0 - none) even if disabled, so that
// saving the link keeps it.
func pickableViews(views entity.ViewDisplayDefs, current int) entity.ViewDisplayDefs {
	out := make(entity.ViewDisplayDefs, 0, len(views))
	for _, v := range views {
		if v.Status == 1 || v.ID == current {
			out = append(out, v)
		}
	}
	return out
}
