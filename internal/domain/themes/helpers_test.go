package themes

import (
	"path/filepath"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/paths"
)

// zzChdirToProjectRoot - paths.TemplatesDir only checks the project root
// and two levels up, while this package lives three levels deep, so chdir
// to the root for the test's duration (t.Chdir restores it).
func zzChdirToProjectRoot(t *testing.T) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
}

// zzBundledTheme - a theme the engine ships, for a mechanism any theme
// has (a theme's info, regions, stylesheets).
const zzBundledTheme = "kosatka"

// zzUseTheme activates theme for the test (paths.SetTheme), restoring
// "no theme" afterwards.
func zzUseTheme(t *testing.T, theme string) {
	t.Helper()
	paths.SetTheme(theme)
	t.Cleanup(func() { paths.SetTheme("") })
}
