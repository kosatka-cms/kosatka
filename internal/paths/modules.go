package paths

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Modules keep their own files next to their code:
// internal/modules/<name>/templates (templates, looked up after the
// engine's and replaceable by a theme under the same relative path) and
// internal/modules/<name>/static (served at /static/modules/<name>/).
// The binary carries them too (each module's Assets, SetModuleAssets):
// a site runs from those, while local development reads the source
// directory, so an edit applies without a rebuild.

// ModulesDir - internal/modules from the current directory (the project
// root, or a cmd/* or package directory two levels down, as tests run);
// "" when missing.
func ModulesDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for _, dir := range []string{
		filepath.Join(wd, "internal", "modules"),
		filepath.Join(wd, "..", "..", "internal", "modules"),
		filepath.Join(wd, "..", "..", "..", "internal", "modules"),
	} {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return filepath.Clean(dir)
		}
	}
	return ""
}

// embeddedPrefix starts the path of a template in a module's embedded
// files: "embed:<module>/templates/<rel>" - not a file on disk
// (ReadTemplate reads it), named by its last element when parsed, as a
// file is.
const embeddedPrefix = "embed:"

var (
	moduleAssetsMu sync.RWMutex
	// moduleAssets - each module's embedded files (templates/, static/).
	moduleAssets map[string]fs.FS
	// embeddedOnly - the embedded files even where the module's source
	// directory is on disk.
	embeddedOnly bool
)

// SetModuleAssets sets the modules' embedded files by name (at startup):
// with embedded - the only ones used (a site runs the binary's own);
// without, a module's source directory on disk comes first (local
// development), its embedded files where it has none.
func SetModuleAssets(assets map[string]fs.FS, embedded bool) {
	moduleAssetsMu.Lock()
	defer moduleAssetsMu.Unlock()
	moduleAssets, embeddedOnly = assets, embedded
}

// moduleSource - where a module's files of one kind (templates, static)
// are: dir on disk, else fsys (its embedded files, rooted at that kind).
type moduleSource struct {
	module, kind string
	dir          string
	fsys         fs.FS
}

// path - the path of rel in the source: the file on disk, or the
// embedded one's ("embed:…").
func (s moduleSource) path(rel string) string {
	if s.dir != "" {
		return filepath.Join(s.dir, rel)
	}
	return embeddedPrefix + s.module + "/" + s.kind + "/" + filepath.ToSlash(rel)
}

// has - whether the source has the file rel.
func (s moduleSource) has(rel string) bool {
	if s.dir != "" {
		return fileExists(filepath.Join(s.dir, rel))
	}
	fi, err := fs.Stat(s.fsys, filepath.ToSlash(rel))
	return err == nil && !fi.IsDir()
}

// glob - the files of the source matching pattern, by relative path.
func (s moduleSource) glob(pattern string) ([]string, error) {
	if s.dir != "" {
		found, err := filepath.Glob(filepath.Join(s.dir, pattern))
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(found))
		for _, p := range found {
			if rel, err := filepath.Rel(s.dir, p); err == nil {
				out = append(out, rel)
			}
		}
		return out, nil
	}
	found, err := fs.Glob(s.fsys, filepath.ToSlash(pattern))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(found))
	for _, p := range found {
		out = append(out, filepath.FromSlash(p))
	}
	return out, nil
}

// moduleSources - every module's sources of kind ("templates",
// "static"), sorted by module name: the source directory when there is
// one and embedded files aren't the only ones used, else the embedded
// files.
func moduleSources(kind string) []moduleSource {
	moduleAssetsMu.RLock()
	assets, embedded := moduleAssets, embeddedOnly
	moduleAssetsMu.RUnlock()
	names := map[string]bool{}
	for name := range assets {
		names[name] = true
	}
	disk := map[string]string{}
	if base := ModulesDir(); base != "" && !embedded {
		if entries, err := os.ReadDir(base); err == nil {
			for _, e := range entries {
				dir := filepath.Join(base, e.Name(), kind)
				if fi, err := os.Stat(dir); e.IsDir() && err == nil && fi.IsDir() {
					disk[e.Name()] = dir
					names[e.Name()] = true
				}
			}
		}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	var out []moduleSource
	for _, name := range sorted {
		if dir, ok := disk[name]; ok {
			out = append(out, moduleSource{module: name, kind: kind, dir: dir})
			continue
		}
		if a := assets[name]; a != nil {
			if fi, err := fs.Stat(a, kind); err == nil && fi.IsDir() {
				if sub, err := fs.Sub(a, kind); err == nil {
					out = append(out, moduleSource{module: name, kind: kind, fsys: sub})
				}
			}
		}
	}
	return out
}

// ModuleStaticFS - a module's static files (nil when it has none), from
// its source directory or its embedded files (moduleSources); a missing
// minified file (the pages link <name>.min.css and .min.js) is its
// unminified one - a module from outside the engine may ship no
// minified copies.
func ModuleStaticFS(module string) fs.FS {
	for _, s := range moduleSources("static") {
		if s.module != module {
			continue
		}
		if s.dir != "" {
			return minFallbackFS{os.DirFS(s.dir)}
		}
		return minFallbackFS{s.fsys}
	}
	return nil
}

// minFallbackFS opens <name>.css (.js) for a missing <name>.min.css
// (.min.js).
type minFallbackFS struct{ fs.FS }

// Open opens name, else its unminified file for a missing minified one.
func (f minFallbackFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open(name)
	if err == nil || !errors.Is(err, fs.ErrNotExist) {
		return file, err
	}
	for _, ext := range []string{".css", ".js"} {
		if base, ok := strings.CutSuffix(name, ".min"+ext); ok {
			return f.FS.Open(base + ext)
		}
	}
	return nil, err
}

// ReadTemplate - a template file's contents: from disk, or from a
// module's embedded files for an "embed:…" path.
func ReadTemplate(p string) ([]byte, error) {
	if !strings.HasPrefix(p, embeddedPrefix) {
		return os.ReadFile(p)
	}
	module, rest, _ := strings.Cut(strings.TrimPrefix(p, embeddedPrefix), "/")
	moduleAssetsMu.RLock()
	a := moduleAssets[module]
	moduleAssetsMu.RUnlock()
	if a == nil {
		return nil, &fs.PathError{Op: "read", Path: p, Err: fs.ErrNotExist}
	}
	return fs.ReadFile(a, path.Clean(rest))
}

// TemplateExists - whether a template path (ResolveTemplate's) is a file.
func TemplateExists(p string) bool {
	if !strings.HasPrefix(p, embeddedPrefix) {
		return fileExists(p)
	}
	_, err := ReadTemplate(p)
	return err == nil
}

// ModuleTemplatesGlob - ModuleTemplatesGlobIn the active theme.
func ModuleTemplatesGlob(pattern string) ([]string, error) {
	return ModuleTemplatesGlobIn(Theme(), pattern)
}

// ModuleTemplatesGlobIn - the modules' templates matching pattern
// (relative to a module's templates directory), each the copy of theme
// name's chain (the theme, then its base themes, ThemeTemplatesDirsOf)
// when one has it: e.g. every module's "blocks/*.html" partials, parsed
// with every page - a previewed theme's and a subtheme's overrides too.
func ModuleTemplatesGlobIn(name, pattern string) ([]string, error) {
	chain := ThemeTemplatesDirsOf(name)
	var out []string
	seen := map[string]bool{}
	for _, src := range moduleSources("templates") {
		found, err := src.glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, rel := range found {
			if seen[rel] {
				continue
			}
			seen[rel] = true
			p := src.path(rel)
			for _, dir := range chain {
				if tp := filepath.Join(dir, rel); fileExists(tp) {
					p = tp
					break
				}
			}
			out = append(out, p)
		}
	}
	return out, nil
}
