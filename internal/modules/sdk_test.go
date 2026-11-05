package modules

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// engineModules - the engine's modules: the directories here with a
// wire.go (a module's Wire).
func engineModules(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(e.Name(), "wire.go")); e.IsDir() && err == nil {
			out = append(out, e.Name())
		}
	}
	if len(out) == 0 {
		t.Fatal("no modules found")
	}
	return out
}

// TestModulesUseSDKOnly - the code of every engine module, its tests too
// (its generated ent code aside), imports nothing of the engine's
// internal/ but its own packages, as a module from its own repository
// can: it would build, and its tests run, outside this repository
// (modulekit/testkit has what they need).
func TestModulesUseSDKOnly(t *testing.T) {
	const engine = "github.com/kosatka-cms/kosatka/"
	for _, name := range engineModules(t) {
		own := engine + "internal/modules/" + name
		err := filepath.WalkDir(name, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == "generated" {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") {
				return nil
			}
			f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				switch {
				case !strings.HasPrefix(path, engine),
					path == own || strings.HasPrefix(path, own+"/"),
					path == engine+"modulekit" || strings.HasPrefix(path, engine+"modulekit/"),
					strings.HasPrefix(path, engine+"pkg/"):
				default:
					t.Errorf("%s imports %s: a module on the SDK uses modulekit and pkg/* only", p, path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
