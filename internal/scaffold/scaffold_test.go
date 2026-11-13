package scaffold

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"

	"github.com/kosatka-cms/kosatka/internal/themeinstall"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/translations"
)

// TestThemeFiles - a new theme passes app theme check with no problem
// (its warnings - author, license, tags, screenshot - are the author's to
// fill in), its name in the manifest, CSS variable and stylesheet.
func TestThemeFiles(t *testing.T) {
	files, err := ThemeFiles(Theme{Name: "sea-breeze", MinVersion: "0.1.11"})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "sea-breeze")
	if _, err := Write(dir, files); err != nil {
		t.Fatal(err)
	}
	engine, err := i18n.ParseSources(translations.Sources)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := themeinstall.Installer{}.Check(context.Background(), dir, engine)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Problem {
			t.Errorf("%s", f)
		} else {
			t.Logf("%s", f)
		}
	}
	manifest := string(files["theme.yaml"])
	for _, want := range []string{"name: Sea breeze\n", "min_version: 0.1.11\n", "css_var: --sea-breeze-accent\n", "stylesheets:\n  - sea-breeze\n"} {
		if !strings.Contains(manifest, want) {
			t.Errorf("no %q in\n%s", want, manifest)
		}
	}
	if _, ok := files["static/css/sea-breeze.css"]; !ok {
		t.Errorf("no stylesheet: %v", keys(files))
	}

	for _, bad := range []Theme{{Name: "Sea"}, {Name: "sea breeze"}, {Name: "sea", Label: "Sea: breeze"}} {
		if _, err := ThemeFiles(bad); err == nil {
			t.Errorf("%+v: no error", bad)
		}
	}
}

// TestModuleFiles - a new module builds on this engine and passes its own
// test; its name comes from its path, its package from its name.
func TestModuleFiles(t *testing.T) {
	if ModuleName("github.com/you/kosatka-module-sea-notes") != "sea_notes" || ModuleName("example.com/notes") != "notes" {
		t.Fatal("names from paths")
	}
	files, err := ModuleFiles(Module{Path: "example.com/kosatka-module-sea-notes", GoVersion: "1.27", EngineVersion: "v0.1.11"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sea_notes.go", "manifest.go", "templates/sea_notes/page.html", "static/sea_notes.css", "translations/ru.json", "page_render_test.go", "README.md"} {
		if _, ok := files[want]; !ok {
			t.Errorf("no %s: %v", want, keys(files))
		}
	}
	if !strings.Contains(string(files["go.mod"]), "require github.com/kosatka-cms/kosatka v0.1.11\n") ||
		!strings.Contains(string(files["sea_notes.go"]), "package seanotes\n") || !strings.Contains(string(files["wire.go"]), `"GET /sea-notes"`) {
		t.Errorf("go.mod, package or route:\n%s\n%s", files["go.mod"], files["wire.go"])
	}
	for _, bad := range []Module{{Path: "notes", GoVersion: "1.27"}, {Path: "example.com/notes", Name: "Notes", GoVersion: "1.27"}} {
		if _, err := ModuleFiles(bad); err == nil {
			t.Errorf("%+v: no error", bad)
		}
	}

	// Built and tested as a module of its own, on this repository's engine.
	if testing.Short() {
		t.Skip("building the module: not with -short")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go tool")
	}
	_, self, _, _ := runtime.Caller(0)
	root, err := filepath.Abs(filepath.Join(filepath.Dir(self), "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	files, err = ModuleFiles(Module{Path: "example.com/kosatka-module-sea-notes", GoVersion: "1.27"})
	if err != nil {
		t.Fatal(err)
	}
	files["go.mod"] = withEngine(t, files["go.mod"], root)
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	files["go.sum"] = sum
	dir := filepath.Join(t.TempDir(), "kosatka-module-sea-notes")
	if _, err := Write(dir, files); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"vet", "./..."}, {"test", "-count=1", "./..."}} {
		cmd := exec.Command(goBin, args...)
		cmd.Dir = dir
		// Only what the engine's own build has: no network, no workspace.
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

// withEngine - the module's go.mod (mod) on the engine in root: required
// and replaced by it, with the engine's requirements as its indirect ones,
// as go mod tidy would list them. Without them the go command would
// read the go.mod files of versions the engine's build never needs (an
// old dependency's own requirements), absent from a cache without the
// network (CI).
func withEngine(t *testing.T, mod []byte, root string) []byte {
	t.Helper()
	engineMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := modfile.Parse("go.mod", engineMod, nil)
	if err != nil {
		t.Fatal(err)
	}
	f, err := modfile.Parse("go.mod", mod, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.AddNewRequire(engine.Module.Mod.Path, "v0.0.0", false)
	for _, r := range engine.Require {
		f.AddNewRequire(r.Mod.Path, r.Mod.Version, true)
	}
	if err := f.AddReplace(engine.Module.Mod.Path, "", root, ""); err != nil {
		t.Fatal(err)
	}
	out, err := f.Format()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestWriteRefusesFull - nothing is written over a directory with files.
func TestWriteRefusesFull(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, map[string][]byte{"a.txt": []byte("a")}); err == nil {
		t.Fatal("wrote into a directory with files")
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err == nil {
		t.Error("a.txt written")
	}
}

func keys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
