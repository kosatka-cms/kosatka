package admin

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// trashPath - the trash's page.
const trashPath = "/admin/content/trash"

// TrashList handles GET /admin/content/trash: the deleted content, the
// latest first - its type, author, who deleted it and when - each
// restored or deleted for good, and the trash emptied
// (service.NodeDeleteService).
func (c *Controller) TrashList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := c.GetPageNumber(r)
	nodes, total, err := c.nodeDeleter.TrashedNodes(ctx, ContentPerPage, (page-1)*ContentPerPage)
	if err != nil {
		c.LogFailure(err, "listing the trash")
		httputil.InternalServerError(w)
		return
	}
	// deletedBy - who put each in the trash (a deleted account - none).
	deletedBy := map[int]*entity.User{}
	for _, n := range nodes {
		if n.DeletedBy == 0 || deletedBy[n.DeletedBy] != nil {
			continue
		}
		if u, err := c.userRepo.ByID(ctx, n.DeletedBy); err == nil {
			deletedBy[n.DeletedBy] = u
		}
	}
	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Trash")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["nodes"] = nodes
	data["deletedBy"] = deletedBy
	data["trashDays"] = c.nodeDeleter.TrashDays()
	data["typeLabels"] = c.nodeTypeLabels(ctx)
	data["pagination"] = pagination.New(page, total, ContentPerPage, trashPath, r.URL.Query())
	// The notices' flags (admin_alerts): ?success=restored|deleted|emptied.
	for _, flag := range []string{"restored", "deleted", "emptied"} {
		data[flag] = r.URL.Query().Get("success") == flag
	}
	data["error"] = r.URL.Query().Get("error")
	c.RenderOrFail(w, r, "admin/content/trash.html", data)
}

// TrashRestore handles POST /admin/content/trash/{id}/restore.
func (c *Controller) TrashRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	if err := c.nodeDeleter.Restore(r.Context(), id); err != nil {
		c.LogFailure(err, "restoring from the trash", "nodeID", id)
		http.Redirect(w, r, trashPath+"?error=restore", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, trashPath+"?success=restored", http.StatusSeeOther)
}

// TrashPurge handles POST /admin/content/trash/{id}/delete: deleted for
// good.
func (c *Controller) TrashPurge(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		c.RenderNotFound(w, r)
		return
	}
	if err := c.nodeDeleter.Purge(r.Context(), id); err != nil {
		c.LogFailure(err, "deleting from the trash", "nodeID", id)
		http.Redirect(w, r, trashPath+"?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, trashPath+"?success=deleted", http.StatusSeeOther)
}

// TrashEmpty handles POST /admin/content/trash/empty: everything in the
// trash deleted for good.
func (c *Controller) TrashEmpty(w http.ResponseWriter, r *http.Request) {
	if _, err := c.nodeDeleter.EmptyTrash(r.Context()); err != nil {
		c.LogFailure(err, "emptying the trash")
		http.Redirect(w, r, trashPath+"?error=empty", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, trashPath+"?success=emptied", http.StatusSeeOther)
}
