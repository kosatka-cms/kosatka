package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// UserFilesPerPage - users per page on /admin/content/files.
const UserFilesPerPage = 50

// userFilesRow - a user's folder on /admin/content/files.
type userFilesRow struct {
	files.UserFolderUsage
	User     *entity.User
	SizeText string
}

// formatFileSize - n bytes as "@size B/KB/MB" in the request's language.
func formatFileSize(ctx context.Context, n int64) string {
	switch {
	case n < 1024:
		return i18n.T(ctx, "@size B", "@size", n)
	case n < 1024*1024:
		return i18n.T(ctx, "@size KB", "@size", fmt.Sprintf("%.1f", float64(n)/1024))
	default:
		return i18n.T(ctx, "@size MB", "@size", fmt.Sprintf("%.1f", float64(n)/1024/1024))
	}
}

// UserFilesList handles GET /admin/content/files: users with files, the
// largest folders first; ?q= narrows to matching user names.
func (c *Controller) UserFilesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if c.userFiles == nil {
		httputil.InternalServerError(w)
		return
	}
	usages, err := c.userFiles.FolderUsages(ctx)
	if err != nil {
		c.LogFailure(err, "error reading users' folders")
		httputil.InternalServerError(w)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q != "" {
		matched := map[int]bool{}
		if users, err := c.userRepo.Search(ctx, q, 200); err == nil && users != nil {
			for _, u := range *users {
				matched[u.ID] = true
			}
		}
		filtered := usages[:0]
		for _, u := range usages {
			if matched[u.UserID] {
				filtered = append(filtered, u)
			}
		}
		usages = filtered
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pager := pagination.New(page, len(usages), UserFilesPerPage, "/admin/content/files", r.URL.Query())
	start := (pager.CurrentPage - 1) * UserFilesPerPage
	if start < 0 || start > len(usages) {
		start = 0
	}
	end := min(start+UserFilesPerPage, len(usages))
	rows := make([]userFilesRow, 0, end-start)
	var totalFiles int
	var totalBytes int64
	for _, u := range usages {
		totalFiles += u.Files
		totalBytes += u.Bytes
	}
	for _, u := range usages[start:end] {
		row := userFilesRow{UserFolderUsage: u, SizeText: formatFileSize(ctx, u.Bytes)}
		if user, err := c.userRepo.ByID(ctx, u.UserID); err == nil {
			row.User = user
		}
		rows = append(rows, row)
	}

	title := i18n.T(ctx, "Users' files")
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["rows"] = rows
	data["query"] = q
	data["folders"] = len(usages)
	data["totalFiles"] = totalFiles
	data["totalSize"] = formatFileSize(ctx, totalBytes)
	data["pagination"] = pager
	c.RenderOrFail(w, r, "admin/people/user_files_list.html", data)
}

// UserFilesBrowse handles GET /admin/content/files/{id}: the file browser
// on that user's folder.
func (c *Controller) UserFilesBrowse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		c.RenderNotFound(w, r)
		return
	}
	user, err := c.userRepo.ByID(ctx, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the user", "id", id)
		return
	}
	if user == nil {
		c.RenderNotFound(w, r)
		return
	}
	title := i18n.T(ctx, "Files of @name", "@name", user.Username)
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["filesUser"] = user
	data["CSRFToken"] = csrf.EnsureToken(c.store, w, r)
	c.RenderOrFail(w, r, "admin/people/user_files.html", data)
}
