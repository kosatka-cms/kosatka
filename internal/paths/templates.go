package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// engineDir - where the engine's own files are when they aren't beside
// the site (config engine_dir / ENGINE_DIR): <dir>/web/templates and
// <dir>/web/static, one engine serving several sites, each running in its
// own directory (its config, themes and files). "" - the working
// directory's web/.
var engineDir atomic.Value

// SetEngineDir sets the engine directory ("" - the working directory).
func SetEngineDir(dir string) { engineDir.Store(strings.TrimSpace(dir)) }

// EngineDir - the engine directory SetEngineDir set ("" - none).
func EngineDir() string {
	dir, _ := engineDir.Load().(string)
	return dir
}

// StaticDir - the engine's static files, served at /static/: the engine
// directory's web/static, else the working directory's.
func StaticDir() string {
	return filepath.Join(EngineDir(), "web", "static")
}

// TemplatesDir returns the path to the engine's templates: the engine
// directory's web/templates (SetEngineDir), else web/templates relative to
// the current working directory - the project root or a cmd/* layout
// (e.g. running from cmd/app).
func TemplatesDir() (string, error) {
	if dir := EngineDir(); dir != "" {
		p := filepath.Join(dir, "web", "templates")
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return filepath.Clean(p), nil
		}
		return "", fmt.Errorf("web/templates not found in the engine directory %s", dir)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	candidates := []string{
		filepath.Join(wd, "web", "templates"),
		filepath.Join(wd, "..", "..", "web", "templates"),
	}
	for _, dir := range candidates {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return filepath.Clean(dir), nil
		}
	}
	return "", fmt.Errorf("web/templates not found (cwd=%s)", wd)
}
