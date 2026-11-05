package paths

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
)

// The active theme (config theme / THEME) - a directory of its own under
// themes/ (ThemesDir), self-contained (one repository can hold it):
//
//	themes/<name>/theme.yaml     its manifest (themes.LoadInfoFor)
//	themes/<name>/templates/     its templates
//	themes/<name>/static/        its assets, served at /static/themes/<name>/
//	themes/<name>/translations/  its strings
//
// A theme template with the same relative path as an engine one is used
// instead of it; an unset theme leaves the engine's own.
var activeTheme atomic.Value

// SetTheme sets the active theme; called once at startup from the config.
func SetTheme(name string) {
	activeTheme.Store(strings.Trim(strings.TrimSpace(name), "/"))
}

// Theme - the active theme's name ("" - none).
func Theme() string {
	name, _ := activeTheme.Load().(string)
	return name
}

// previewThemeKey - the context key of a request's own theme (WithTheme).
type previewThemeKey struct{}

// WithTheme - ctx rendering in theme name instead of the active one (a
// theme's preview); "" - the engine's own templates.
func WithTheme(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, previewThemeKey{}, strings.Trim(strings.TrimSpace(name), "/"))
}

// PreviewTheme - the theme WithTheme gave ctx, if any.
func PreviewTheme(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(previewThemeKey{}).(string)
	return name, ok
}

// ThemeFor - the theme ctx renders in: its preview theme, else the
// active one.
func ThemeFor(ctx context.Context) string {
	if name, ok := PreviewTheme(ctx); ok {
		return name
	}
	return Theme()
}

// baseThemes - a theme's base theme (theme.yaml "base", a subtheme's;
// "" - none), set by the service reading the manifests (SetBaseThemes).
var baseThemes atomic.Value

// SetBaseThemes - how a theme's base theme is found.
func SetBaseThemes(f func(name string) string) {
	baseThemes.Store(f)
}

// MaxThemeChain - the deepest chain of base themes followed, the theme
// itself included; a deeper one doesn't load (themes.LoadInfoFor).
const MaxThemeChain = 8

// ThemeChain - theme name and its base themes, the theme first (a
// subtheme, its base, the base's base…; empty - the engine's templates).
// A loop or a chain deeper than MaxThemeChain stops there.
func ThemeChain(name string) []string {
	f, _ := baseThemes.Load().(func(string) string)
	var out []string
	seen := map[string]bool{}
	for name != "" && !seen[name] && len(out) < MaxThemeChain {
		seen[name] = true
		out = append(out, name)
		if f == nil {
			break
		}
		name = f(name)
	}
	return out
}

// ThemeTemplatesDirsOf - the templates directories of theme name's chain
// (ThemeChain) that exist, the theme's first.
func ThemeTemplatesDirsOf(name string) []string {
	var out []string
	for _, t := range ThemeChain(name) {
		if dir := ThemeTemplatesDirOf(t); dir != "" {
			out = append(out, dir)
		}
	}
	return out
}

// ThemeWithStatic - the first theme of name's chain with the static file
// rel (under its static/), name itself when none has it.
func ThemeWithStatic(name, rel string) string {
	for _, t := range ThemeChain(name) {
		if dir := ThemeStaticDir(t); dir != "" && fileExists(filepath.Join(dir, rel)) {
			return t
		}
	}
	return name
}

// themesDir - the themes' directory SetThemesDir set ("" - themes/ beside
// web/).
var themesDir atomic.Value

// SetThemesDir points ThemesDir at dir ("" - back to themes/ beside web/):
// the tests' own themes (testdata/themes), which the engine's tests use
// rather than a site's theme.
func SetThemesDir(dir string) { themesDir.Store(dir) }

// ThemesDir - the directory of the themes, themes/ beside web/ (the
// project's or the installation's root), or the one SetThemesDir set. With
// an engine directory (SetEngineDir) the themes are the site's own:
// themes/ in its working directory.
func ThemesDir() (string, error) {
	if dir, _ := themesDir.Load().(string); dir != "" {
		return dir, nil
	}
	if EngineDir() != "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		return filepath.Join(wd, "themes"), nil
	}
	base, err := TemplatesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(filepath.Dir(base)), "themes"), nil
}

// ThemesLockFile - the record of the themes installed from their
// repositories (app theme add): themes/themes.lock, deployed with them.
func ThemesLockFile() (string, error) {
	root, err := ThemesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "themes.lock"), nil
}

// ThemesDirs - where themes are looked for, first first: ThemesDir and,
// with an engine directory (SetEngineDir), the themes it ships (its
// themes/), which every site sharing it has; a site's own theme of the
// same name comes first.
func ThemesDirs() []string {
	var dirs []string
	if root, err := ThemesDir(); err == nil {
		dirs = append(dirs, root)
	}
	if engine := EngineDir(); engine != "" {
		if shipped := filepath.Join(engine, "themes"); !slices.Contains(dirs, shipped) {
			dirs = append(dirs, shipped)
		}
	}
	return dirs
}

// ThemeDir - theme name's directory (<themes>/<name>, the first of
// ThemesDirs having it), "" for none, a missing one or a name that isn't
// a directory's.
func ThemeDir(name string) string {
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return ""
	}
	for _, root := range ThemesDirs() {
		dir := filepath.Join(root, name)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	return ""
}

// themeSubdir - part of theme name's directory, "" when either is missing.
func themeSubdir(name, part string) string {
	dir := ThemeDir(name)
	if dir == "" {
		return ""
	}
	sub := filepath.Join(dir, part)
	if fi, err := os.Stat(sub); err != nil || !fi.IsDir() {
		return ""
	}
	return sub
}

// ThemeTemplatesDirOf - theme name's templates directory, "" for none or
// a missing one.
func ThemeTemplatesDirOf(name string) string {
	return themeSubdir(name, "templates")
}

// ThemeStaticDir - theme name's static directory (served at
// /static/themes/<name>/), "" for none or a missing one.
func ThemeStaticDir(name string) string {
	return themeSubdir(name, "static")
}

// ThemeTranslationsDirsOf - the translations directories of theme name's
// chain (ThemeChain) that exist, the theme's first.
func ThemeTranslationsDirsOf(name string) []string {
	var out []string
	for _, t := range ThemeChain(name) {
		if dir := themeSubdir(t, "translations"); dir != "" {
			out = append(out, dir)
		}
	}
	return out
}

// ResolveTemplate - the absolute path of a template given relative to the
// templates root: the active theme's copy when it has one, else the
// engine's, else a module's (moduleSources: its file on disk, or an
// "embed:…" path of its embedded files - ReadTemplate reads either).
func ResolveTemplate(rel string) (string, error) {
	return ResolveTemplateIn(Theme(), rel)
}

// ResolveTemplateIn - ResolveTemplate with theme name's copy: its own,
// else its base themes' (ThemeChain).
func ResolveTemplateIn(theme, rel string) (string, error) {
	for _, dir := range ThemeTemplatesDirsOf(theme) {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	base, err := TemplatesDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(base, rel)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	for _, src := range moduleSources("templates") {
		if src.has(rel) {
			return src.path(rel), nil
		}
	}
	return p, nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// GlobTemplates - files matching pattern (relative to the templates
// root) from the engine, the modules and the active theme: a theme file
// replaces the engine's or a module's with the same relative path (a
// module's never replaces the engine's); theme-only files come last.
func GlobTemplates(pattern string) ([]string, error) {
	return GlobTemplatesIn(Theme(), pattern)
}

// GlobTemplatesIn - GlobTemplates with theme name's files and its base
// themes' (ThemeChain): a theme's file replaces its base's, both the
// engine's.
func GlobTemplatesIn(theme, pattern string) ([]string, error) {
	base, err := TemplatesDir()
	if err != nil {
		return nil, err
	}
	engine, err := filepath.Glob(filepath.Join(base, pattern))
	if err != nil {
		return nil, err
	}
	// The modules' files, as if in the engine root (by relative path).
	type sourced struct{ path, rel string }
	files := make([]sourced, 0, len(engine))
	seen := map[string]bool{}
	for _, p := range engine {
		rel, err := filepath.Rel(base, p)
		if err != nil {
			rel = p
		}
		files = append(files, sourced{p, rel})
		seen[rel] = true
	}
	for _, src := range moduleSources("templates") {
		found, err := src.glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, rel := range found {
			if !seen[rel] {
				files = append(files, sourced{src.path(rel), rel})
				seen[rel] = true
			}
		}
	}
	// The chain's files by relative path, the deepest base first, so a
	// subtheme's replace its base's; themedOrder - their first appearance.
	dirs := ThemeTemplatesDirsOf(theme)
	themedByRel := map[string]string{}
	var themedOrder []string
	for i := len(dirs) - 1; i >= 0; i-- {
		found, err := filepath.Glob(filepath.Join(dirs[i], pattern))
		if err != nil {
			return nil, err
		}
		for _, p := range found {
			rel, err := filepath.Rel(dirs[i], p)
			if err != nil {
				continue
			}
			if _, ok := themedByRel[rel]; !ok {
				themedOrder = append(themedOrder, rel)
			}
			themedByRel[rel] = p
		}
	}
	out := make([]string, 0, len(files)+len(themedByRel))
	for _, f := range files {
		if t, ok := themedByRel[f.rel]; ok {
			out = append(out, t)
			delete(themedByRel, f.rel)
			continue
		}
		out = append(out, f.path)
	}
	for _, rel := range themedOrder {
		if p, ok := themedByRel[rel]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}
