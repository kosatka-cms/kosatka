package app

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// TestExportModuleStatic - each module's static files go to
// <dir>/modules/<name>/, replacing what was there; a .css or .js without
// its .min copy gets one, a module's own minified file is kept as it is;
// a module without static files gets nothing.
func TestExportModuleStatic(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "modules", "demo", "old.css")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	assets := map[string]fs.FS{
		"demo": fstest.MapFS{
			"static/demo.css":     {Data: []byte("a { }")},
			"static/demo.js":      {Data: []byte("full()")},
			"static/demo.min.js":  {Data: []byte("min()")},
			"static/img/logo.svg": {Data: []byte("<svg/>")},
			"templates/x.html":    {Data: []byte("x")},
		},
		"plain": fstest.MapFS{"templates/y.html": {Data: []byte("y")}},
	}
	if err := exportModuleStatic(assets, dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[string]string{
		"demo/demo.css": "a { }", "demo/demo.min.css": "a { }", "demo/demo.js": "full()", "demo/demo.min.js": "min()", "demo/img/logo.svg": "<svg/>",
	} {
		if b, err := os.ReadFile(filepath.Join(dir, "modules", filepath.FromSlash(f))); err != nil || string(b) != want {
			t.Errorf("%s: %q %v, want %q", f, b, err, want)
		}
	}
	for _, f := range []string{"demo/old.css", "demo/x.html", "plain"} {
		if _, err := os.Stat(filepath.Join(dir, "modules", f)); err == nil {
			t.Errorf("%s exported", f)
		}
	}
}

// TestExportThemeStatic - each theme's static files (themes/<name>/static)
// go to <dir>/themes/<name>/, replacing what was there, with .min copies
// as a module's; a theme in two themes directories is the first one's
// (as the application serves it); a theme without static files and a
// hidden directory get nothing.
func TestExportThemeStatic(t *testing.T) {
	site, engine, dir := t.TempDir(), t.TempDir(), t.TempDir()
	write := func(root, rel, data string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(site, "sand/theme.yaml", "name: Sand")
	write(site, "sand/static/css/sand.css", "a { }")
	write(site, "sand/static/js/sand.js", "full()")
	write(site, "sand/static/js/sand.min.js", "min()")
	write(site, "sand/static/screenshot.jpg", "jpg")
	write(site, "plain/theme.yaml", "name: Plain")
	write(site, ".git/static/x.css", "x")
	write(engine, "sand/static/css/sand.css", "the engine's copy")
	write(engine, "sea/static/css/sea.css", "b { }")
	write(dir, "themes/sand/css/old.min.css", "stale")

	if err := exportThemeStatic([]string{site, engine, filepath.Join(site, "missing")}, dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[string]string{
		"sand/css/sand.css": "a { }", "sand/css/sand.min.css": "a { }", "sand/js/sand.js": "full()", "sand/js/sand.min.js": "min()",
		"sand/screenshot.jpg": "jpg", "sea/css/sea.css": "b { }", "sea/css/sea.min.css": "b { }",
	} {
		if b, err := os.ReadFile(filepath.Join(dir, "themes", filepath.FromSlash(f))); err != nil || string(b) != want {
			t.Errorf("%s: %q %v, want %q", f, b, err, want)
		}
	}
	for _, f := range []string{"sand/css/old.min.css", "sand/theme.yaml", "plain", ".git"} {
		if _, err := os.Stat(filepath.Join(dir, "themes", f)); err == nil {
			t.Errorf("%s exported", f)
		}
	}
}
