package interfaces

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

// StylesController serves the public images in their image styles.
type StylesController struct {
	service *imagestyle.StyleService
	logger  *slog.Logger
}

// NewStylesController creates a StylesController on the image styles
// service s.
func NewStylesController(
	s *imagestyle.StyleService,
	logger *slog.Logger,
) *StylesController {
	return &StylesController{
		service: s,
		logger:  logger,
	}
}

// ServeStyledImage serves an image rendered with an image style.
func (c StylesController) ServeStyledImage(w http.ResponseWriter, r *http.Request) {
	styleName := r.PathValue("style")
	relativePath := r.PathValue("filepath")

	file, info, err := c.service.GetStyledImage(r.Context(), styleName, relativePath)
	if err != nil {
		// Unknown style or missing source is a 404 (mostly bots hitting stale
		// URLs), logged at debug level; a source that isn't an image it
		// renders - a warning; other failures are errors.
		level := slog.LevelError
		switch {
		case errors.Is(err, imagestyle.ErrStyleNotFound) || errors.Is(err, imagestyle.ErrOriginalNotFound) ||
			errors.Is(err, filestore.ErrInvalidKey):
			level = slog.LevelDebug
		case errors.Is(err, imagestyle.ErrNotRenderable):
			level = slog.LevelWarn
		}
		if c.logger != nil {
			c.logger.Log(r.Context(), level, "error getting styled image",
				"error", err,
				"style", styleName,
				"path", relativePath,
			)
		}
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	defer func() { _ = file.Close() }()

	// A store serving its files itself (a bucket's CDN): the derivative
	// exists now, the store serves it.
	if base := paths.PublicStoreURL(); base != "" {
		RedirectToStore(w, r, base, info.Key)
		return
	}

	w.Header().Set("Content-Type", ImageMIME(relativePath))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Image-Style", styleName)

	http.ServeContent(w, r, info.Name, info.ModTime, file)
}

// RedirectToStore sends the visitor to the file key at base, the address
// the public files' store serves its files at itself
// (paths.PublicStoreURL), with the request's query (a derivative's
// ?v=…). Browsers and CDNs keep the redirect for a day.
func RedirectToStore(w http.ResponseWriter, r *http.Request, base, key string) {
	target := base + (&url.URL{Path: key}).EscapedPath()
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.Redirect(w, r, target, http.StatusFound)
}
