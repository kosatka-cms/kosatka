package modules

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
)

// TestEveryModuleListed - every module of manifests.All() is listed here
// with its Wire, and nothing else is: a module missing from one list would
// be described but never wired (or the other way round).
func TestEveryModuleListed(t *testing.T) {
	described := map[string]bool{}
	for _, m := range manifests.All() {
		described[m.Name] = true
		if e, ok := registry[m.Name]; !ok || e.wire == nil {
			t.Errorf("%s: in manifests.All() but not wired here", m.Name)
		}
	}
	for name := range registry {
		if !described[name] {
			t.Errorf("%s: wired here but not in manifests.All()", name)
		}
	}
}

// TestAssetsEmbedded - every file of a module's templates/ and static/ is
// built into the binary (its Assets): a site runs from those
// (paths.SetModuleAssets), so a directory left out of the module's embed
// directive would be missing there only.
func TestAssetsEmbedded(t *testing.T) {
	for _, name := range Names() {
		assets := Assets()[name]
		for _, kind := range []string{"templates", "static"} {
			dir := filepath.Join(name, kind)
			if _, err := os.Stat(dir); err != nil {
				continue
			}
			if assets == nil {
				t.Errorf("%s has %s/ but no Assets", name, kind)
				continue
			}
			err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || d.Name() == ".DS_Store" {
					return err
				}
				rel, _ := filepath.Rel(name, p)
				if _, err := fs.Stat(assets, filepath.ToSlash(rel)); err != nil {
					t.Errorf("%s: %s isn't embedded", name, rel)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
