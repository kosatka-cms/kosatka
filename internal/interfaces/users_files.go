package interfaces

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"log/slog"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// UserFilesController is the JSON API for per-user files under the public files directory (u{userID}/).
type UserFilesController struct {
	svc    *files.UserFilesService
	store  *sessions.CookieStore
	logger *slog.Logger
	// auth checks the permission to work in another user's files
	// (nil - only one's own files are reachable).
	auth service.AuthService
	// usage - where files are linked from (nil - not checked).
	usage *files.UserFileUsageService
	// formats - the text formats, whose own upload rules an editor's
	// upload follows (nil - the site's rules).
	formats textFormatByID
}

// textFormatByID - a text format by its ID (service.TextFormatService).
type textFormatByID interface {
	TextFormatByID(ctx context.Context, id int) (*entity.TextFormatDef, error)
}

// mayUseFormat - whether the signed-in user may use text format id: its
// upload rules are theirs, else the site's apply.
func (c *UserFilesController) mayUseFormat(r *http.Request, id int) bool {
	if c.formats == nil || c.auth == nil {
		return false
	}
	format, err := c.formats.TextFormatByID(r.Context(), id)
	if err != nil || format == nil {
		return false
	}
	user, _ := c.auth.GetCurrentUser(r.Context(), r)
	return c.auth.CanUseTextFormat(r.Context(), user, *format)
}

// canUploadDocuments - whether the signed-in user may upload documents
// besides images (PermUploadDocuments).
func (c *UserFilesController) canUploadDocuments(r *http.Request) bool {
	if c.auth == nil {
		return false
	}
	current, err := c.auth.GetCurrentUser(r.Context(), r)
	return err == nil && current != nil && c.auth.HasPermission(r.Context(), current, service.PermUploadDocuments)
}

// targetUserID - whose files the request works on: the current user's,
// or with ?user=<id> another user's for holders of
// PermAdministerUserFiles. Writes the error response when it returns
// false.
func (c *UserFilesController) targetUserID(w http.ResponseWriter, r *http.Request) (int, bool) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		httputil.JSONError(w, i18n.T(r.Context(), "Log in to manage your files."), http.StatusUnauthorized)
		return 0, false
	}
	raw := r.URL.Query().Get("user")
	if raw == "" {
		return userID, true
	}
	target, err := strconv.Atoi(raw)
	if err != nil || target <= 0 {
		httputil.JSONError(w, i18n.T(r.Context(), "The user was not found."), http.StatusBadRequest)
		return 0, false
	}
	if target == userID {
		return userID, true
	}
	if c.auth == nil {
		httputil.JSONError(w, i18n.T(r.Context(), "You may not manage these files."), http.StatusForbidden)
		return 0, false
	}
	current, err := c.auth.GetCurrentUser(r.Context(), r)
	if err != nil || current == nil || !c.auth.HasPermission(r.Context(), current, service.PermAdministerUserFiles) {
		httputil.JSONError(w, i18n.T(r.Context(), "You may not manage these files."), http.StatusForbidden)
		return 0, false
	}
	return target, true
}

// NewUserFilesController creates a UserFilesController on the users'
// files service, the session store, the auth service, the files' usage
// and the text formats (their upload rules).
func NewUserFilesController(
	svc *files.UserFilesService,
	store *sessions.CookieStore,
	logger *slog.Logger,
	auth service.AuthService,
	usage *files.UserFileUsageService,
	formats textFormatByID,
) *UserFilesController {
	return &UserFilesController{svc: svc, store: store, logger: logger, auth: auth, usage: usage, formats: formats}
}

// List GET /api/user/files?path=
func (c *UserFilesController) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := c.targetUserID(w, r)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	entries, err := c.svc.List(r.Context(), userID, path)
	if err != nil {
		c.sendError(w, r, err)
		return
	}
	baseURL := c.svc.PublicWebPath(files.UserDirName(userID))
	imageTypes := c.svc.ImageTypes(r.Context())
	httputil.SendJSON(w, http.StatusOK, map[string]any{
		"entries":  entries,
		"base_url": baseURL,
		// can_upload_documents - whether the uploader (not the folder's
		// owner) may upload documents; without it, images only.
		"can_upload_documents": c.canUploadDocuments(r),
		// image_types / image_formats - the image formats the upload policy
		// allows: MIME types (file picker filter) and their names (hints).
		"image_types":   imageTypes,
		"image_formats": files.ImageFormatsLabel(imageTypes),
	})
}

// Tree GET /api/user/files/tree - nested folders for sidebar navigation.
func (c *UserFilesController) Tree(w http.ResponseWriter, r *http.Request) {
	userID, ok := c.targetUserID(w, r)
	if !ok {
		return
	}
	tree, err := c.svc.FolderTree(r.Context(), userID)
	if err != nil {
		c.sendError(w, r, err)
		return
	}
	httputil.SendJSON(w, http.StatusOK, map[string]any{"tree": tree})
}

// Upload POST /api/user/files/upload (multipart: file, csrf_token, optional
// path=subdir, optional text_format_id for the per-format image policy of
// FileUploadService.ValidateAndProcessTextFormatImage; invalid means 0).
// Images and the upload policy's document types are accepted.
func (c *UserFilesController) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := c.targetUserID(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The form could not be read. Reload the page and try again."), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "Choose a file."), http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	subdir := r.FormValue("path")
	formatID, _ := strconv.Atoi(r.FormValue("text_format_id"))
	// A format's upload rules only for who may use the format: a request
	// naming another (a more permissive one) gets the site's.
	if formatID > 0 && !c.mayUseFormat(r, formatID) {
		formatID = 0
	}
	url, kind, err := c.svc.SaveUploadedFile(r.Context(), userID, subdir, file, header, formatID, c.canUploadDocuments(r))
	if err != nil {
		c.logger.Warn("user file upload failed", "error", err, "user_id", userID)
		c.sendUploadError(r.Context(), w, err)
		return
	}
	httputil.SendJSON(w, http.StatusOK, map[string]string{"location": url, "kind": kind})
}

// parseUserFilesForm parses body: fetch+FormData sends multipart/form-data; ParseForm alone may not fill fields for CSRF.
func parseUserFilesForm(r *http.Request) error {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		return r.ParseMultipartForm(1 << 20)
	}
	return r.ParseForm()
}

// Mkdir POST /api/user/files/mkdir
func (c *UserFilesController) Mkdir(w http.ResponseWriter, r *http.Request) {
	userID, ok := c.targetUserID(w, r)
	if !ok {
		return
	}
	if err := parseUserFilesForm(r); err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The form could not be read. Reload the page and try again."), http.StatusBadRequest)
		return
	}
	parent := r.FormValue("path")
	name := r.FormValue("name")
	if name == "" {
		httputil.JSONError(w, i18n.T(r.Context(), "Enter a name."), http.StatusBadRequest)
		return
	}
	if err := c.svc.Mkdir(r.Context(), userID, parent, name); err != nil {
		c.sendError(w, r, err)
		return
	}
	httputil.SendJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Rmdir POST /api/user/files/rmdir
func (c *UserFilesController) Rmdir(w http.ResponseWriter, r *http.Request) {
	userID, ok := c.targetUserID(w, r)
	if !ok {
		return
	}
	if err := parseUserFilesForm(r); err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The form could not be read. Reload the page and try again."), http.StatusBadRequest)
		return
	}
	path := r.FormValue("path")
	if err := c.svc.RmdirEmpty(r.Context(), userID, path); err != nil {
		c.sendError(w, r, err)
		return
	}
	httputil.SendJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Unlink POST /api/user/files/unlink
func (c *UserFilesController) Unlink(w http.ResponseWriter, r *http.Request) {
	userID, ok := c.targetUserID(w, r)
	if !ok {
		return
	}
	if err := parseUserFilesForm(r); err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The form could not be read. Reload the page and try again."), http.StatusBadRequest)
		return
	}
	path := r.FormValue("path")
	if path == "" {
		httputil.JSONError(w, i18n.T(r.Context(), "No file or folder is given."), http.StatusBadRequest)
		return
	}
	if err := c.svc.RemoveFile(r.Context(), userID, path); err != nil {
		c.sendError(w, r, err)
		return
	}
	httputil.SendJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Usage GET /api/user/files/usage?path= - where a file is linked from
// right now (read from the texts).
func (c *UserFilesController) Usage(w http.ResponseWriter, r *http.Request) {
	userID, ok := c.targetUserID(w, r)
	if !ok {
		return
	}
	path := strings.Trim(r.URL.Query().Get("path"), "/")
	if path == "" || strings.Contains(path, "..") {
		httputil.JSONError(w, i18n.T(r.Context(), "No file or folder is given."), http.StatusBadRequest)
		return
	}
	refs := []entity.UserFileRef{}
	if c.usage != nil {
		found, err := c.usage.Refs(r.Context(), userID, path)
		if err != nil {
			logFailure(c.logger, err, "error checking file usage", "path", path)
			httputil.JSONError(w, i18n.T(r.Context(), "The operation failed. Try again."), http.StatusInternalServerError)
			return
		}
		if found != nil {
			refs = found
		}
	}
	httputil.SendJSON(w, http.StatusOK, map[string]any{"refs": refs})
}

// Rename POST /api/user/files/rename (path, name): a file or folder gets a
// new name in the same folder; returns its new path.
func (c *UserFilesController) Rename(w http.ResponseWriter, r *http.Request) {
	userID, ok := c.targetUserID(w, r)
	if !ok {
		return
	}
	if err := parseUserFilesForm(r); err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The form could not be read. Reload the page and try again."), http.StatusBadRequest)
		return
	}
	path, name := r.FormValue("path"), strings.TrimSpace(r.FormValue("name"))
	if path == "" || name == "" {
		httputil.JSONError(w, i18n.T(r.Context(), "Enter a name."), http.StatusBadRequest)
		return
	}
	newPath, err := c.svc.Rename(r.Context(), userID, path, name)
	if err != nil {
		c.sendError(w, r, err)
		return
	}
	httputil.SendJSON(w, http.StatusOK, map[string]string{"path": newPath})
}

func (c *UserFilesController) sendError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	// The service's errors as the visitor reads them (not their internal
	// text, which names paths).
	for _, e := range []struct {
		err    error
		text   string
		status int
	}{
		{files.ErrUserFilesInvalidPath, i18n.T(ctx, "The path is invalid."), http.StatusBadRequest},
		{files.ErrUserFilesNotFound, i18n.T(ctx, "The file or folder was not found."), http.StatusNotFound},
		{files.ErrUserFilesNotDir, i18n.T(ctx, "This isn't a folder."), http.StatusBadRequest},
		{files.ErrUserFilesDirNotEmpty, i18n.T(ctx, "The folder isn't empty."), http.StatusBadRequest},
		{files.ErrUserFilesIsDir, i18n.T(ctx, "This is a folder, not a file."), http.StatusBadRequest},
		{files.ErrUserFilesRootReserved, i18n.T(ctx, "Your root folder can't be changed this way."), http.StatusBadRequest},
		{files.ErrUserFilesExists, i18n.T(ctx, "A file or folder with this name already exists."), http.StatusConflict},
	} {
		if errors.Is(err, e.err) {
			httputil.JSONError(w, e.text, e.status)
			return
		}
	}
	logFailure(c.logger, err, "user files error")
	httputil.JSONError(w, i18n.T(ctx, "The operation failed. Try again."), http.StatusInternalServerError)
}

// sendUploadError - a check the upload failed is 400 with its message;
// any other failure is logged, 500.
func (c *UserFilesController) sendUploadError(ctx context.Context, w http.ResponseWriter, err error) {
	if i18n.IsUserError(err) {
		httputil.JSONError(w, i18n.ErrorText(ctx, err), http.StatusBadRequest)
		return
	}
	logFailure(c.logger, err, "uploading a user file")
	httputil.JSONError(w, i18n.T(ctx, "Upload failed"), http.StatusInternalServerError)
}
