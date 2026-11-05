package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// MenuList renders GET /admin/config/menus - the list of menus with a
// label-only quick-add; machine name and description are set on the
// menu's edit form.
func (c *Controller) MenuList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	menus, err := c.menuService.Menus(ctx)
	if err != nil {
		c.LogFailure(err, "error listing menus")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Menus")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Menus"))}
	data["menus"] = menus
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"

	c.RenderOrFail(w, r, "admin/menus/index.html", data)
}

// MenuCreate handles POST /admin/config/menus - the machine name is
// transliterated from the label, as in TextFormatCreate.
func (c *Controller) MenuCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/menus", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		http.Redirect(w, r, "/admin/config/menus?error=empty", http.StatusSeeOther)
		return
	}

	machineName := strings.ReplaceAll(slug.Generate(label), "-", "_")
	if machineName == "" {
		machineName = "menu"
	}

	_, err := c.menuService.CreateMenu(ctx, entity.MenuDef{
		MachineName: machineName,
		Label:       label,
	})
	if err != nil {
		if errors.Is(err, service.ErrMenuDuplicate) {
			http.Redirect(w, r, "/admin/config/menus?error=duplicate", http.StatusSeeOther)
			return
		}
		c.LogFailure(err, "error creating menu", "machineName", machineName)
		http.Redirect(w, r, "/admin/config/menus?error=create", http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, "/admin/config/menus?success=1", http.StatusSeeOther)
}

// MenuEdit renders GET /admin/config/menus/{id}/edit - label and description.
func (c *Controller) MenuEdit(w http.ResponseWriter, r *http.Request) {
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

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Menu \"@name\"", "@name", menu.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Menu @name - Admin panel", "@name", menu.Label)}
	data["menu"] = menu
	data["error"] = r.URL.Query().Get("error")
	data["success"] = r.URL.Query().Get("success") == "1"

	c.RenderOrFail(w, r, "admin/menus/edit.html", data)
}

// MenuUpdate handles POST /admin/config/menus/{id}. The machine name is
// an identity string and is immutable.
func (c *Controller) MenuUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/menus", http.StatusSeeOther)
		return
	}
	editRedirect := fmt.Sprintf("/admin/config/menus/%d/edit", id)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, editRedirect, http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		http.Redirect(w, r, editRedirect+"?error=empty", http.StatusSeeOther)
		return
	}

	_, err := c.menuService.UpdateMenu(ctx, entity.MenuDef{
		ID:          id,
		Label:       label,
		Description: strings.TrimSpace(r.FormValue("description")),
		InSitemap:   r.FormValue("in_sitemap") == "1",
	})
	if err != nil {
		c.LogFailure(err, "error updating menu", "id", id)
		http.Redirect(w, r, editRedirect+"?error=update", http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, editRedirect+"?success=1", http.StatusSeeOther)
}

// MenuDeleteConfirm renders GET /admin/config/menus/{id}/delete - a
// confirmation page warning that every link in the menu is deleted too.
func (c *Controller) MenuDeleteConfirm(w http.ResponseWriter, r *http.Request) {
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
		c.LogFailure(err, "error listing menu links before delete confirm", "menuID", id)
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Delete menu \"@name\"", "@name", menu.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "Delete menu @name - Admin panel", "@name", menu.Label)}
	data["menu"] = menu
	data["linkCount"] = len(links)

	c.RenderOrFail(w, r, "admin/menus/delete.html", data)
}

// MenuDelete handles POST /admin/config/menus/{id}/delete.
func (c *Controller) MenuDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/menus", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/menus?error=invalid", http.StatusSeeOther)
		return
	}

	if err := c.menuService.DeleteMenu(ctx, id); err != nil {
		c.LogFailure(err, "error deleting menu", "id", id)
		http.Redirect(w, r, "/admin/config/menus?error=delete", http.StatusSeeOther)
		return
	}
	c.cacheService.ForceUpdate()
	http.Redirect(w, r, "/admin/config/menus?deleted=1", http.StatusSeeOther)
}
