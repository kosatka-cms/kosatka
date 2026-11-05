package paths

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEngineDir - with an engine directory the engine's templates and
// static files are its web/, the themes the site's own (its working
// directory's themes/).
func TestEngineDir(t *testing.T) {
	engine, site := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(engine, "web", "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(site)
	SetEngineDir(engine)
	t.Cleanup(func() { SetEngineDir("") })

	if dir, err := TemplatesDir(); err != nil || dir != filepath.Join(engine, "web", "templates") {
		t.Errorf("TemplatesDir %q, %v", dir, err)
	}
	if dir := StaticDir(); dir != filepath.Join(engine, "web", "static") {
		t.Errorf("StaticDir %q", dir)
	}
	if dir, err := ThemesDir(); err != nil || filepath.Base(dir) != "themes" || filepath.Dir(dir) == engine {
		t.Errorf("ThemesDir %q, %v: want the site's themes/", dir, err)
	}

	// The engine's themes are every site's; a site's own of the same name
	// comes first.
	for _, dir := range []string{filepath.Join(engine, "themes", "shipped"), filepath.Join(engine, "themes", "both"), filepath.Join(site, "themes", "both")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if dir := ThemeDir("shipped"); dir != filepath.Join(engine, "themes", "shipped") {
		t.Errorf("ThemeDir(shipped) %q: want the engine's", dir)
	}
	if dir := ThemeDir("both"); filepath.Base(filepath.Dir(filepath.Dir(dir))) == filepath.Base(engine) {
		t.Errorf("ThemeDir(both) %q: want the site's", dir)
	}

	SetEngineDir(filepath.Join(engine, "missing"))
	if _, err := TemplatesDir(); err == nil {
		t.Error("an engine directory without web/templates isn't an error")
	}
	SetEngineDir("")
	if dir := StaticDir(); dir != filepath.Join("web", "static") {
		t.Errorf("StaticDir without an engine directory %q", dir)
	}
}
