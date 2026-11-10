package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Permissions - the role × permission matrix (/admin/users/permissions),
// backed by service.PermissionService.

const permissionsAdminURL = "/admin/users/permissions"

// permissionRoleColumn - a role as a matrix column.
type permissionRoleColumn struct {
	Role       entity.Role
	FullAccess bool
	// Base - the role every signed-in user has (entity.RoleUser).
	Base bool
}

// PermissionsPage renders GET /admin/users/permissions.
func (c *Controller) PermissionsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	catalog, err := c.permissions.Catalog(ctx)
	if err != nil {
		c.LogFailure(err, "error loading permission catalog")
		httputil.InternalServerError(w)
		return
	}
	roles, err := c.permissions.Roles(ctx)
	if err != nil {
		c.LogFailure(err, "error loading roles")
		httputil.InternalServerError(w)
		return
	}
	columns := make([]permissionRoleColumn, 0, len(roles))
	for _, role := range roles {
		columns = append(columns, permissionRoleColumn{
			Role:       role,
			FullAccess: service.HasGrant(&role, service.PermFullAccess),
			Base:       role.Name == entity.RoleUser,
		})
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Permissions")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Permissions"))}
	data["catalog"] = catalog
	data["columns"] = columns
	data["success"] = r.URL.Query().Get("success") == "1"
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/people/permissions.html", data)
}

// PermissionsSave handles POST /admin/users/permissions: each checked
// box is a "grant" value "<role id>|<permission>".
func (c *Controller) PermissionsSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, permissionsAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	granted := map[int]map[string]bool{}
	for _, v := range r.Form["grant"] {
		idStr, key, ok := strings.Cut(v, "|")
		id, err := strconv.Atoi(idStr)
		if !ok || err != nil {
			continue
		}
		if granted[id] == nil {
			granted[id] = map[string]bool{}
		}
		granted[id][key] = true
	}
	roles, err := c.permissions.Roles(ctx)
	if err != nil {
		c.LogFailure(err, "error loading roles")
		http.Redirect(w, r, permissionsAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	for _, role := range roles {
		if err := c.permissions.SaveGrants(ctx, role.ID, granted[role.ID]); err != nil {
			c.LogFailure(err, "error saving role permissions", "roleID", role.ID)
			http.Redirect(w, r, permissionsAdminURL+"?error=save", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, permissionsAdminURL+"?success=1", http.StatusSeeOther)
}
