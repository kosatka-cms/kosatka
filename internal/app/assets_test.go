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
