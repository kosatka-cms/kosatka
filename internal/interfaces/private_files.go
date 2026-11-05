package interfaces

import (
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

// PrivateFilesController serves the private files of file and image
// fields at paths.PrivateFilesURL - an image's style derivatives too
// (imageurl.Styled) - only to those service.PrivateFileAccess
// lets download the file, from the private files' store, never cached by
// anyone but the browser.
type PrivateFilesController struct {
	*BaseController
	files  entity.FileRepository
	access service.PrivateFileAccess
	store  func() filestore.Store
	// styles - the image styles' derivatives of the private files.
	styles *imagestyle.StyleService
}

// NewPrivateFilesController creates a PrivateFilesController on the
// files, the access check, the private files' store and the image styles.
func NewPrivateFilesController(base *BaseController, files entity.FileRepository, access service.PrivateFileAccess, store func() filestore.Store, styles *imagestyle.StyleService) *PrivateFilesController {
	return &PrivateFilesController{BaseController: base, files: files, access: access, store: store, styles: styles}
}

// privateInline - the media types a browser may show in place; anything
// else is downloaded.
func privateInline(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".pdf":
		return true
	}
	return false
}

// Serve handles GET {PrivateFilesURL}{key...} - a file, or
// styles/<style>/<key>, a private image's derivative: a visitor without
// access is sent to sign in, a signed-in user without it gets 403; a
// file the site doesn't know is 404.
func (c *PrivateFilesController) Serve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := r.PathValue("filepath")
	if !fs.ValidPath(key) || key == "." || filestore.HasDotSegment(key) {
		c.RenderNotFound(w, r)
		return
	}
	f, err := c.files.ByURI(ctx, entity.PrivateScheme+key)
	style, double := "", false
	if isNotFound(err) {
		if s, d, original, ok := imageurl.ParsePrivateStyled(key); ok {
			style, double, key = s, d, original
			f, err = c.files.ByURI(ctx, entity.PrivateScheme+key)
		}
	}
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading a private file", "key", key)
		return
	}
	if !c.allowed(w, r, *f) {
		return
	}
	var file filestore.File
	var info filestore.Info
	if style != "" {
		file, info, err = c.styles.GetStyledOriginal(ctx, style, double, key)
		if errors.Is(err, imagestyle.ErrStyleNotFound) {
			c.RenderLoadFailure(w, r, entity.ErrEntityNotFound, "an unknown image style of a private image", "style", style, "key", key)
			return
		}
		if errors.Is(err, imagestyle.ErrNotRenderable) {
			c.Logger().Warn("a private file the image style can't render", "key", key, "style", style, "error", err)
			c.RenderNotFound(w, r)
			return
		}
		if errors.Is(err, imagestyle.ErrOriginalNotFound) {
			err = filestore.ErrNotExist
		}
	} else {
		file, info, err = c.store().Open(ctx, key)
	}
	if errors.Is(err, filestore.ErrNotExist) {
		// Its row is there, the file isn't: worth a look.
		c.Logger().Warn("a private file missing from its store", "key", key, "fileID", f.ID)
		c.RenderNotFound(w, r)
		return
	}
	if err != nil {
		c.LogFailure(err, "opening a private file", "key", key, "style", style)
		httputil.InternalServerError(w)
		return
	}
	defer func() { _ = file.Close() }()
	name := f.OriginalFilename
	if name == "" {
		name = path.Base(key)
	}
	contentType := f.MimeType
	if style != "" {
		// The derivative's own type (WebP or JPEG), its own name.
		contentType = ImageMIME(info.Name)
		name = strings.TrimSuffix(name, path.Ext(name)) + path.Ext(info.Name)
	}
	h := w.Header()
	h.Set("Cache-Control", "private, no-cache")
	h.Set("X-Content-Type-Options", "nosniff")
	disposition := "attachment"
	if privateInline(info.Name) {
		disposition = "inline"
	}
	h.Set("Content-Disposition", disposition+"; filename*=UTF-8''"+url.PathEscape(name))
	if strings.ToLower(path.Ext(info.Name)) != ".pdf" {
		h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; sandbox")
	}
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, info.Name, info.ModTime, file)
}

// allowed reports whether the request's user may download f, answering
// the request when not: a visitor is sent to sign in, a signed-in user
// gets 403.
func (c *PrivateFilesController) allowed(w http.ResponseWriter, r *http.Request, f entity.File) bool {
	user, err := c.GetCurrentUser(r)
	if err != nil {
		c.LogFailure(err, "loading the user for a private file", "fileID", f.ID)
		httputil.InternalServerError(w)
		return false
	}
	ok, err := c.access.CanDownload(r.Context(), user, f)
	if err != nil {
		c.LogFailure(err, "checking access to a private file", "fileID", f.ID)
		httputil.InternalServerError(w)
		return false
	}
	if ok {
		return true
	}
	if user == nil {
		http.Redirect(w, r, "/login?destination="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return false
	}
	c.RenderForbidden(w, r)
	return false
}
