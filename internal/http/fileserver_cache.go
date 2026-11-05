package http

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

const (
	// CacheStatic is for versioned assets under /static/ (CSS/JS use ?v= query).
	CacheStatic = "public, max-age=31536000, immutable"
	// CacheUploads is for uploaded files under /sites/ (no immutable - same path may be replaced).
	CacheUploads = "public, max-age=31536000"
)

// FileServerWithCache serves the files under root (not its folders'
// listings, not dotfiles) and sets Cache-Control on every response.
func FileServerWithCache(root, cacheControl string) http.Handler {
	fs := http.FileServer(filesOnly{http.Dir(root)})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cacheControl)
		fs.ServeHTTP(w, r)
	})
}

// filesOnly - the regular files of a file system: a folder or a dotfile
// (.git, .env) is not found.
type filesOnly struct{ http.FileSystem }

func (f filesOnly) Open(name string) (http.File, error) {
	if strings.Contains(name, "/.") {
		return nil, os.ErrNotExist
	}
	file, err := f.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	if st, err := file.Stat(); err != nil || st.IsDir() {
		_ = file.Close()
		return nil, os.ErrNotExist
	}
	return file, nil
}

// uploadsInline - extensions of uploaded files a browser may show in
// place; everything else is served as a download.
var uploadsInline = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".pdf": true,
}

// uploadsCSP - files under /sites/ are data, never pages: no scripts,
// no subresources, sandboxed even if an HTML or SVG file gets there.
const uploadsCSP = "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; sandbox"

// UploadsFileServer serves uploaded files from store: files only (no
// folder listings, no dotfiles like .htaccess), with nosniff and a
// sandboxing CSP; anything but images and PDF is a download. When the
// store serves its files itself (storeURL, e.g. a bucket's CDN), it
// redirects there instead.
func UploadsFileServer(store filestore.Store, storeURL func() string, cacheControl string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		rel := path.Clean("/" + r.URL.Path)
		for _, seg := range strings.Split(rel, "/") {
			if strings.HasPrefix(seg, ".") {
				http.NotFound(w, r)
				return
			}
		}
		if base := storeURL(); base != "" {
			interfaces.RedirectToStore(w, r, base, strings.TrimPrefix(rel, "/"))
			return
		}
		f, info, err := store.Open(r.Context(), strings.TrimPrefix(rel, "/"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = f.Close() }()
		h := w.Header()
		h.Set("Cache-Control", cacheControl)
		h.Set("X-Content-Type-Options", "nosniff")
		ext := strings.ToLower(filepath.Ext(info.Name))
		// A sandboxing CSP stops browsers' PDF viewers; PDFs get nosniff only.
		if ext != ".pdf" {
			h.Set("Content-Security-Policy", uploadsCSP)
		}
		if !uploadsInline[ext] {
			h.Set("Content-Disposition", "attachment")
		}
		http.ServeContent(w, r, info.Name, info.ModTime, f)
	})
}

// moduleStatic serves /static/modules/<name>/<file> from the module's own
// static files (paths.ModuleStaticFS: its source directory, or the ones
// built into the binary) when it has the file, else from fallback
// (web/static).
func moduleStatic(fallback http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fsys := paths.ModuleStaticFS(r.PathValue("module"))
		rel := path.Clean("/" + r.PathValue("filepath"))
		if fsys != nil && !strings.Contains(rel, "/.") {
			if fi, err := fs.Stat(fsys, strings.TrimPrefix(rel, "/")); err == nil && !fi.IsDir() {
				w.Header().Set("Cache-Control", CacheStatic)
				http.ServeFileFS(w, r, fsys, strings.TrimPrefix(rel, "/"))
				return
			}
		}
		http.StripPrefix("/static/", fallback).ServeHTTP(w, r)
	})
}

// themeStatic - a theme's own files (themes/<theme>/static) at
// /static/themes/<theme>/…, else fallback (web/static: the deploy also
// copies them there, where the web server serves /static/ itself). A
// missing minified file (the pages link <name>.min.css and .min.js) is
// its unminified one, as a module's: a theme not minified yet still works.
func themeStatic(fallback http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dir := paths.ThemeStaticDir(r.PathValue("theme"))
		rel := path.Clean("/" + r.PathValue("filepath"))
		if dir != "" && !strings.Contains(rel, "/.") {
			for _, name := range []string{rel, unminified(rel)} {
				if name == "" {
					continue
				}
				file := filepath.Join(dir, filepath.FromSlash(name))
				if fi, err := os.Stat(file); err == nil && !fi.IsDir() {
					w.Header().Set("Cache-Control", CacheStatic)
					http.ServeFile(w, r, file)
					return
				}
			}
		}
		http.StripPrefix("/static/", fallback).ServeHTTP(w, r)
	})
}

// unminified - <name>.css (.js) for <name>.min.css (.min.js), "" for
// another file.
func unminified(name string) string {
	for _, ext := range []string{".css", ".js"} {
		if base, ok := strings.CutSuffix(name, ".min"+ext); ok {
			return base + ext
		}
	}
	return ""
}
