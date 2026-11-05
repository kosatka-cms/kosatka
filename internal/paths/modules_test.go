package paths

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// TestModuleTemplates - a template the engine doesn't have is found in a
// module's directory; the theme's copy under the same relative path wins;
// the modules' block partials are listed with the theme's copies.
func TestModuleTemplates(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"web/templates/layout.html",
		"themes/site/templates/demo/page.html",
		"internal/modules/demo/templates/demo/page.html",
		"internal/modules/demo/templates/demo/list.html",
		"internal/modules/demo/templates/blocks/demo.html",
		"internal/modules/other/templates/blocks/other.html",
		"themes/site/templates/blocks/other.html",
	} {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	SetTheme("")
	t.Cleanup(func() { SetTheme("") })
	rel := func(p string) string {
		r, _ := filepath.Rel(root, p)
		return filepath.ToSlash(r)
	}
	if p, _ := ResolveTemplate("demo/list.html"); rel(p) != "internal/modules/demo/templates/demo/list.html" {
		t.Errorf("module template: %s", p)
	}
	if p, _ := ResolveTemplate("layout.html"); rel(p) != "web/templates/layout.html" {
		t.Errorf("engine template: %s", p)
	}
	SetTheme("site")
	if p, _ := ResolveTemplate("demo/page.html"); rel(p) != "themes/site/templates/demo/page.html" {
		t.Errorf("theme over module: %s", p)
	}
	blocks, err := ModuleTemplatesGlob(filepath.Join("blocks", "*.html"))
	if err != nil || len(blocks) != 2 || rel(blocks[0]) != "internal/modules/demo/templates/blocks/demo.html" || rel(blocks[1]) != "themes/site/templates/blocks/other.html" {
		t.Errorf("module blocks: %v %v", blocks, err)
	}
}

// TestModuleTemplatePathsUnique - no two modules ship a template under the
// same relative path: the first module's would hide the other's (a theme
// overrides by that path; ModuleTemplatesGlob and ResolveTemplate take
// one file per path), as two partials/hooks.html did.
func TestModuleTemplatePathsUnique(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "modules", "*", "templates"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no module templates: %v", err)
	}
	owner := map[string]string{}
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(dir, p)
			if prev, ok := owner[rel]; ok {
				t.Errorf("%s: in %s and %s", rel, prev, dir)
			}
			owner[rel] = dir
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestModuleTemplatesEmbedded - the modules' embedded files
// (SetModuleAssets): the only ones a site uses, the source directory
// ignored; in local development the source directory comes first and a
// module without one is read from its embedded files. A theme's copy wins
// over either.
func TestModuleTemplatesEmbedded(t *testing.T) {
	root := t.TempDir()
	for p, body := range map[string]string{
		"web/templates/layout.html":                      "engine",
		"internal/modules/demo/templates/demo/page.html": "disk page",
		"internal/modules/demo/static/demo.js":           "disk js",
		"themes/site/templates/blocks/extra.html":        "theme block",
		"themes/sub/templates/blocks/demo.html":          "subtheme block",
	} {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	SetTheme("")
	t.Cleanup(func() { SetTheme(""); SetModuleAssets(nil, false) })
	assets := map[string]fs.FS{
		"demo": fstest.MapFS{
			"templates/demo/page.html":   {Data: []byte("embedded page")},
			"templates/blocks/demo.html": {Data: []byte("embedded block")},
			"static/demo.js":             {Data: []byte("embedded js")},
		},
		"extra": fstest.MapFS{
			"templates/extra/list.html":   {Data: []byte("extra list")},
			"templates/blocks/extra.html": {Data: []byte("extra block")},
		},
	}
	read := func(p string) string {
		t.Helper()
		b, err := ReadTemplate(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		return string(b)
	}
	static := func(module, name string) string {
		fsys := ModuleStaticFS(module)
		if fsys == nil {
			return ""
		}
		b, _ := fs.ReadFile(fsys, name)
		return string(b)
	}

	SetModuleAssets(assets, true)
	p, err := ResolveTemplate("demo/page.html")
	if err != nil || p != "embed:demo/templates/demo/page.html" || read(p) != "embedded page" || !TemplateExists(p) || filepath.Base(p) != "page.html" {
		t.Errorf("embedded only: %q %v", p, err)
	}
	if TemplateExists("embed:demo/templates/demo/missing.html") || TemplateExists("embed:nobody/templates/x.html") {
		t.Error("a missing embedded template exists")
	}
	if got := static("demo", "demo.js"); got != "embedded js" {
		t.Errorf("embedded static: %q", got)
	}
	// No minified copy shipped: the pages' .min address gets the file.
	if got := static("demo", "demo.min.js"); got != "embedded js" {
		t.Errorf("the unminified file for a missing .min: %q", got)
	}
	if got := static("demo", "missing.min.css"); got != "" {
		t.Errorf("a missing file: %q", got)
	}
	if ModuleStaticFS("extra") != nil {
		t.Error("a module without static files has some")
	}
	if files, err := GlobTemplates(filepath.Join("extra", "*.html")); err != nil || len(files) != 1 || read(files[0]) != "extra list" {
		t.Errorf("glob of an embedded module: %v %v", files, err)
	}
	SetTheme("site")
	blocks, err := ModuleTemplatesGlob(filepath.Join("blocks", "*.html"))
	if err != nil || len(blocks) != 2 || read(blocks[0]) != "embedded block" || read(blocks[1]) != "theme block" {
		t.Errorf("blocks, the theme's copy over the embedded one: %v %v", blocks, err)
	}
	SetTheme("")
	// Another theme than the active one (a preview), and a subtheme: its
	// own copy first, then its base theme's.
	if blocks, err := ModuleTemplatesGlobIn("site", filepath.Join("blocks", "*.html")); err != nil || len(blocks) != 2 || read(blocks[1]) != "theme block" {
		t.Errorf("blocks in a theme not active: %v %v", blocks, err)
	}
	SetBaseThemes(func(name string) string {
		if name == "sub" {
			return "site"
		}
		return ""
	})
	t.Cleanup(func() { SetBaseThemes(nil) })
	if blocks, err := ModuleTemplatesGlobIn("sub", filepath.Join("blocks", "*.html")); err != nil || len(blocks) != 2 || read(blocks[0]) != "subtheme block" || read(blocks[1]) != "theme block" {
		t.Errorf("blocks in a subtheme: %v %v", blocks, err)
	}

	SetModuleAssets(assets, false)
	if p, _ := ResolveTemplate("demo/page.html"); read(p) != "disk page" {
		t.Errorf("local: the source directory first, got %q", p)
	}
	if p, _ := ResolveTemplate("extra/list.html"); read(p) != "extra list" {
		t.Errorf("local: a module without sources, embedded: %q", p)
	}
	if got := static("demo", "demo.js"); got != "disk js" {
		t.Errorf("local static: %q", got)
	}
}
