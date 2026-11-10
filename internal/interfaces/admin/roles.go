package admin

import (
	"errors"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Roles - create, rename and delete roles (/admin/users/roles); their
// permissions are edited on /admin/users/permissions.

const rolesAdminURL = "/admin/users/roles"

// roleRow - a role as the list shows it.
type roleRow struct {
	Role       entity.Role
	Users      int
	FullAccess bool
	Base       bool
}

// rolesErrorCodes - the ?error= code per service error.
var rolesErrorCodes = map[error]string{
	service.ErrRoleName:      "name",
	service.ErrRoleLabel:     "label",
	service.ErrRoleExists:    "exists",
	service.ErrRoleProtected: "protected",
	service.ErrRoleInUse:     "in-use",
}

func rolesErrorCode(err error) string {
	for e, code := range rolesErrorCodes {
		if errors.Is(err, e) {
			return code
		}
	}
	return "save"
}

// RolesPage renders GET /admin/users/roles.
func (c *Controller) RolesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	roles, err := c.permissions.Roles(ctx)
	if err != nil {
		c.LogFailure(err, "error loading roles")
		httputil.InternalServerError(w)
		return
	}
	counts := c.permissions.RoleUserCounts(ctx, roles)
	rows := make([]roleRow, 0, len(roles))
	for _, role := range roles {
		rows = append(rows, roleRow{
			Role:       role,
			Users:      counts[role.ID],
			FullAccess: service.HasGrant(&role, service.PermFullAccess),
			Base:       role.Name == entity.RoleUser,
		})
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Roles")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Roles"))}
	data["roles"] = rows
	data["success"] = r.URL.Query().Get("success")
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/people/roles.html", data)
}

// RoleCreate handles POST /admin/users/roles.
func (c *Controller) RoleCreate(w http.ResponseWriter, r *http.Request) {
	if _, err := c.permissions.CreateRole(r.Context(), r.FormValue("name"), r.FormValue("label")); err != nil {
		c.logger.Warn("error creating role", "error", err)
		http.Redirect(w, r, rolesAdminURL+"?error="+rolesErrorCode(err), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, rolesAdminURL+"?success=created", http.StatusSeeOther)
}

// RoleRename handles POST /admin/users/roles/{id}/rename.
func (c *Controller) RoleRename(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, rolesAdminURL, http.StatusSeeOther)
		return
	}
	if err := c.permissions.RenameRole(r.Context(), id, r.FormValue("label")); err != nil {
		c.logger.Warn("error renaming role", "error", err, "roleID", id)
		http.Redirect(w, r, rolesAdminURL+"?error="+rolesErrorCode(err), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, rolesAdminURL+"?success=saved", http.StatusSeeOther)
}

// RoleDelete handles POST /admin/users/roles/{id}/delete.
func (c *Controller) RoleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, rolesAdminURL, http.StatusSeeOther)
		return
	}
	if err := c.permissions.DeleteRole(r.Context(), id); err != nil {
		c.logger.Warn("error deleting role", "error", err, "roleID", id)
		http.Redirect(w, r, rolesAdminURL+"?error="+rolesErrorCode(err), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, rolesAdminURL+"?success=deleted", http.StatusSeeOther)
}
