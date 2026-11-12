package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/paths"
)

// TestSplitVersion - "<repository or name>@<version>"; the "@" of an ssh
// address isn't a version's.
func TestSplitVersion(t *testing.T) {
	for arg, want := range map[string][2]string{
		"sand":        {"sand", ""},
		"sand@v1.2.0": {"sand", "v1.2.0"},
		"https://github.com/example/kosatka-theme-sand": {"https://github.com/example/kosatka-theme-sand", ""},
		"https://github.com/example/sand@1.2.0":         {"https://github.com/example/sand", "1.2.0"},
		"git@github.com:example/sand.git":               {"git@github.com:example/sand.git", ""},
		"git@github.com:example/sand.git@v2.0.0":        {"git@github.com:example/sand.git", "v2.0.0"},
	} {
		if source, version := splitVersion(arg); source != want[0] || version != want[1] {
			t.Errorf("splitVersion(%q) = %q, %q, want %q, %q", arg, source, version, want[0], want[1])
		}
	}
}

// TestThemeCommandUsage - an unknown or missing command prints the usage
// and exits 2.
func TestThemeCommandUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"frobnicate"}, {"add"}, {"update"}, {"remove", "a", "b"}} {
		var out, errOut bytes.Buffer
		if code := themeCommand(args, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "app theme add") {
			t.Errorf("%v: exit %d, stderr %q", args, code, errOut.String())
		}
	}
}

// TestThemeNew - app theme new writes a theme into themes/<name> (the
// flags before or after the name), never over a directory with files;
// a bad name exits 2.
func TestThemeNew(t *testing.T) {
	themesDir := t.TempDir()
	paths.SetThemesDir(themesDir)
	t.Cleanup(func() { paths.SetThemesDir("") })
	var out, errOut bytes.Buffer
	if code := themeCommand([]string{"new", "sea-breeze", "-label", "Sea Breeze"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	manifest, err := os.ReadFile(filepath.Join(themesDir, "sea-breeze", "theme.yaml"))
	if err != nil || !strings.Contains(string(manifest), "name: Sea Breeze\n") {
		t.Fatalf("theme.yaml: %v\n%s", err, manifest)
	}
	if !strings.Contains(out.String(), filepath.Join(themesDir, "sea-breeze", "static", "css", "sea-breeze.css")) {
		t.Errorf("the files written: %s", out.String())
	}
	if code := themeCommand([]string{"new", "-label", "Again", "sea-breeze"}, io.Discard, io.Discard); code != 1 {
		t.Errorf("over a theme: exit %d", code)
	}
	dir := filepath.Join(t.TempDir(), "kosatka-theme-dune")
	if code := themeCommand([]string{"new", "dune", "-dir", dir}, io.Discard, io.Discard); code != 0 {
		t.Errorf("-dir: exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "theme.yaml")); err != nil {
		t.Error(err)
	}
	if code := themeCommand([]string{"new", "Dune"}, io.Discard, io.Discard); code != 2 {
		t.Errorf("a bad name: exit %d", code)
	}
}
