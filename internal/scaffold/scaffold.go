// Package scaffold - the starting files of a new module or theme (app
// module new, app theme new): a module of its own Go module with a page,
// its test, styles, translations and README; a theme with its manifest,
// stylesheet, translations and README. Both work as written: the module
// builds and passes its test, the theme passes app theme check.
package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

// files - the scaffolds' templates: module/ and theme/, "NAME" in a path
// standing for the module's or theme's name, ".tmpl" left out of the
// written file's name. [[ ]] delimit their own actions, so the engine's
// template syntax ({{ }}) is written as it is.
//
//go:embed all:module all:theme
var files embed.FS

// The names: a module's machine name (its tables' prefix, its admin
// addresses), a theme's (its directory, its asset URLs).
var (
	moduleNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	themeNameRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	// modulePathRe - a Go module path: a host, then its path.
	modulePathRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*\.[a-zA-Z]{2,}(/[a-zA-Z0-9._~+-]+)+$`)
)

// Module - what a new module is: its Go module path, its machine name
// ("" - from the path's last part: "kosatka-module-notes" → "notes"), the
// engine release it requires ("v0.1.11"; "" - none: go mod tidy takes
// the latest) and the Go version of its go.mod ("1.27").
type Module struct {
	Path, Name, EngineVersion, GoVersion string
}

// Theme - what a new theme is: its name (its directory's), its label
// ("" - from the name: "sea-breeze" → "Sea breeze"), the oldest engine
// release it works with ("0.1.11"; "" - none named).
type Theme struct {
	Name, Label, MinVersion string
}

// moduleData / themeData - what the templates see.
type moduleData struct {
	Module
	Package, Label, URLPath, CSSClass string
}

type themeData struct {
	Theme
}

// ModuleName - the machine name a module of Go module path gets by
// default: its last part, without a "kosatka-module-" or "kosatka-"
// prefix, "-" as "_".
func ModuleName(modulePath string) string {
	s := strings.ToLower(path.Base(modulePath))
	for _, p := range []string{"kosatka-module-", "kosatka-"} {
		s = strings.TrimPrefix(s, p)
	}
	return strings.ReplaceAll(s, "-", "_")
}

// ModuleFiles - a new module's files by their path in its directory.
func ModuleFiles(m Module) (map[string][]byte, error) {
	if !modulePathRe.MatchString(m.Path) {
		return nil, fmt.Errorf("%q: not a Go module path (host/path…, e.g. github.com/you/kosatka-module-notes)", m.Path)
	}
	if m.Name == "" {
		m.Name = ModuleName(m.Path)
	}
	if !moduleNameRe.MatchString(m.Name) {
		return nil, fmt.Errorf("module name %q: lowercase latin letters, digits and _, a letter first (-name)", m.Name)
	}
	if m.GoVersion == "" {
		return nil, errors.New("no Go version for go.mod")
	}
	d := moduleData{
		Module:   m,
		Package:  strings.ReplaceAll(m.Name, "_", ""),
		Label:    label(m.Name),
		URLPath:  strings.ReplaceAll(m.Name, "_", "-"),
		CSSClass: strings.ReplaceAll(m.Name, "_", "-"),
	}
	return render("module", m.Name, d)
}

// ThemeFiles - a new theme's files by their path in its directory.
func ThemeFiles(t Theme) (map[string][]byte, error) {
	if !themeNameRe.MatchString(t.Name) {
		return nil, fmt.Errorf("theme name %q: lowercase latin letters, digits, - and _", t.Name)
	}
	if t.Label == "" {
		t.Label = label(t.Name)
	}
	if strings.ContainsAny(t.Label, "\n\r\"\\:#") {
		return nil, fmt.Errorf("theme label %q: one line, no quotes, colons or #", t.Label)
	}
	return render("theme", t.Name, themeData{t})
}

// label - a name as words: "sea_breeze" → "Sea breeze".
func label(name string) string {
	s := strings.NewReplacer("_", " ", "-", " ").Replace(name)
	return strings.ToUpper(s[:1]) + s[1:]
}

// render - the files of the scaffold under dir, NAME in their paths as
// name, each executed with data.
func render(dir, name string, data any) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := fs.WalkDir(files, dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		src, err := files.ReadFile(p)
		if err != nil {
			return err
		}
		tpl, err := template.New(p).Delims("[[", "]]").Option("missingkey=error").Parse(string(src))
		if err != nil {
			return fmt.Errorf("scaffold %s: %w", p, err)
		}
		var b bytes.Buffer
		if err := tpl.Execute(&b, data); err != nil {
			return fmt.Errorf("scaffold %s: %w", p, err)
		}
		rel := strings.TrimSuffix(strings.TrimPrefix(p, dir+"/"), ".tmpl")
		out[strings.ReplaceAll(rel, "NAME", name)] = b.Bytes()
		return nil
	})
	return out, err
}

// Write puts files into dir, which must not exist or be empty (nothing
// is overwritten); it returns their paths, sorted.
func Write(dir string, files map[string][]byte) ([]string, error) {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("%s: not empty", dir)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, files[name], 0o644); err != nil {
			return nil, err
		}
	}
	return names, nil
}
