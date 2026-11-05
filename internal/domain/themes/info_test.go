package themes

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// TestInfoDeclaresTemplateRegions - every region a template prints
// ((region "x"), regionHasBlocks "x") is declared in theme.yaml, or its
// blocks could never be placed; the engine's file and, with each theme
// active, the theme's resolution parse with a valid default region.
func TestInfoDeclaresTemplateRegions(t *testing.T) {
	zzChdirToProjectRoot(t)
	base, err := paths.TemplatesDir()
	if err != nil {
		t.Fatal(err)
	}
	tmplComment := regexp.MustCompile(`(?s)\{\{-?\s*/\*.*?\*/\s*-?\}\}`)
	re := regexp.MustCompile(`(?:\(region|regionHasBlocks) "([a-z0-9_]+)"`)
	// htmlFiles - relative path → file under dir ("" - none).
	htmlFiles := func(dir string) map[string]string {
		out := map[string]string{}
		if dir == "" {
			return out
		}
		err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && filepath.Ext(p) == ".html" {
				rel, _ := filepath.Rel(dir, p)
				out[rel] = p
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	engine := htmlFiles(base)
	themes, err := Available()
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range themes {
		theme := option.Dir
		// The files this theme renders: its own, and the engine's it
		// doesn't replace (paths.ResolveTemplate).
		files := map[string]string{}
		for rel, p := range engine {
			files[rel] = p
		}
		if theme != "" {
			for rel, p := range htmlFiles(paths.ThemeTemplatesDirOf(theme)) {
				files[rel] = p
			}
		}
		zzUseTheme(t, theme)
		info, err := LoadInfo()
		if err != nil {
			t.Fatalf("theme %q: %v", theme, err)
		}
		if !info.Has(info.DefaultRegion) {
			t.Fatalf("theme %q: default region %q not declared", theme, info.DefaultRegion)
		}
		used := 0
		for _, file := range files {
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range re.FindAllStringSubmatch(tmplComment.ReplaceAllString(string(b), ""), -1) {
				used++
				if !info.Has(entity.Region(m[1])) {
					t.Errorf("theme %q: region %q (printed by %s) is not in theme.yaml", theme, m[1], file)
				}
			}
		}
		if used == 0 {
			t.Fatalf("theme %q: no regions found in templates", theme)
		}
	}
}

// TestLoadInfoDefaultRegionFallback - a missing or undeclared
// default_region falls back to the first region; no regions is an error.
func TestLoadInfoDefaultRegionFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "web", "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(dir, "web", "templates", InfoFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("default_region: nope\nregions:\n  - id: top\n    label: Top\n  - id: side\n    label: Side\n")
	info, err := LoadInfo()
	if err != nil || info.DefaultRegion != "top" || len(info.Regions) != 2 || info.Regions[1].Label != "Side" {
		t.Fatalf("info = %+v, %v", info, err)
	}
	write("name: empty\n")
	if _, err := LoadInfo(); err == nil {
		t.Fatal("no regions must be an error")
	}
}

// TestAvailable - the engine's templates plus every theme directory
// with a theme.yaml, each under its own name (the admin lists them by it).
func TestAvailable(t *testing.T) {
	zzChdirToProjectRoot(t)
	themes, err := Available()
	if err != nil {
		t.Fatal(err)
	}
	dirs := map[string]bool{}
	names := map[string]string{}
	for _, th := range themes {
		dirs[th.Dir] = true
		if th.Regions == 0 || th.Name == "" {
			t.Errorf("theme %+v", th)
		}
		if other, ok := names[th.Name]; ok {
			t.Errorf("themes %q and %q are both named %q", other, th.Dir, th.Name)
		}
		names[th.Name] = th.Dir
	}
	if !dirs[""] || !dirs[zzBundledTheme] {
		t.Errorf("themes %+v", themes)
	}
	if IsAvailable("no-such-theme") || IsAvailable("../templates") || !IsAvailable(zzBundledTheme) {
		t.Error("IsAvailable")
	}
}

func TestLoadInfoFor(t *testing.T) {
	zzChdirToProjectRoot(t)
	engine, err := LoadInfoFor("")
	if err != nil || len(engine.Regions) == 0 {
		t.Fatalf("engine theme: %v", err)
	}
	theme, err := LoadInfoFor(zzBundledTheme)
	if err != nil || len(theme.Regions) == 0 || theme.Name == engine.Name {
		t.Fatalf("a bundled theme: %+v %v", theme.Name, err)
	}
	if _, err := LoadInfoFor("no-such-theme"); err == nil {
		t.Error("missing theme must fail")
	}
}
