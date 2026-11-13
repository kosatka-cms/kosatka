package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/modules"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// assetsUsage - "app assets".
const assetsUsage = `usage:
  app assets export <dir>   the modules' static files the binary carries, into <dir>/modules/<name>/, and the themes'
                            (themes/<name>/static), into <dir>/themes/<name>/ - where a web server serving /static/ itself finds them`

// runAssets is "app assets …".
func runAssets(args []string) int {
	if len(args) != 2 || args[0] != "export" {
		_, _ = fmt.Fprintln(os.Stderr, assetsUsage)
		return 2
	}
	if err := exportModuleStatic(modules.Assets(), args[1], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "assets:", err)
		return 1
	}
	if err := exportThemeStatic(paths.ThemesDirs(), args[1], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "assets:", err)
		return 1
	}
	return 0
}

// exportModuleStatic writes every module's static files (its embedded
// static/) to dir/modules/<name>/ (exportStatic).
func exportModuleStatic(assets map[string]fs.FS, dir string, log io.Writer) error {
	names := make([]string, 0, len(assets))
	for name := range assets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		static, err := fs.Sub(assets[name], "static")
		if err != nil {
			return err
		}
		if _, err := fs.Stat(static, "."); err != nil {
			continue // no static files
		}
		dst := filepath.Join(dir, "modules", name)
		n, err := exportStatic(static, dst)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		_, _ = fmt.Fprintf(log, "%s: %d file(s) -> %s\n", name, n, dst)
	}
	return nil
}

// exportThemeStatic writes every theme's static files (themes/<name>/static
// of the themes directories roots, the first one having a theme as the
// application serves it - paths.ThemeDir) to dir/themes/<name>/
// (exportStatic).
func exportThemeStatic(roots []string, dir string, log io.Writer) error {
	done := map[string]bool{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() || strings.HasPrefix(name, ".") || done[name] {
				continue
			}
			done[name] = true
			static := filepath.Join(root, name, "static")
			if fi, err := os.Stat(static); err != nil || !fi.IsDir() {
				continue // no static files
			}
			dst := filepath.Join(dir, "themes", name)
			n, err := exportStatic(os.DirFS(static), dst)
			if err != nil {
				return fmt.Errorf("theme %s: %w", name, err)
			}
			_, _ = fmt.Fprintf(log, "theme %s: %d file(s) -> %s\n", name, n, dst)
		}
	}
	return nil
}

// exportStatic writes the files of static to dst, replacing what was
// there: a web server serving /static/ from disk (nginx) serves them at
// the addresses the app does. A .css or .js without its .min copy gets one
// (the same file) - the pages link the minified files, and a module or
// theme from outside the engine may ship none (the app serves the
// unminified one then; nginx would answer 404, or an older release's
// copy left on the server).
func exportStatic(static fs.FS, dst string) (int, error) {
	if err := os.RemoveAll(dst); err != nil {
		return 0, err
	}
	n := 0
	err := fs.WalkDir(static, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(static, p)
		if err != nil {
			return err
		}
		targets := []string{p}
		if minified := minName(p); minified != "" {
			if _, err := fs.Stat(static, minified); err != nil {
				targets = append(targets, minified)
			}
		}
		for _, t := range targets {
			out := filepath.Join(dst, filepath.FromSlash(t))
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(out, b, 0o644); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// minName - the minified copy's name of a .css or .js file ("" for
// another file or one minified already).
func minName(p string) string {
	ext := path.Ext(p)
	if (ext != ".css" && ext != ".js") || strings.HasSuffix(p, ".min"+ext) {
		return ""
	}
	return strings.TrimSuffix(p, ext) + ".min" + ext
}
