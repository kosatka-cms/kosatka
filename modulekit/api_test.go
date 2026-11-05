package modulekit_test

import (
	"bytes"
	"flag"
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateAPI = flag.Bool("update", false, "write testdata/api.txt from the SDK as it is")

// sdkPackages - the module SDK's packages: what a module from its own
// repository may import of the engine (with pkg/*).
var sdkPackages = []string{
	"github.com/kosatka-cms/kosatka/modulekit",
	"github.com/kosatka-cms/kosatka/modulekit/demokit",
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit",
	"github.com/kosatka-cms/kosatka/modulekit/schemakit",
	"github.com/kosatka-cms/kosatka/modulekit/testkit",
}

// TestAPI - the SDK's exported API as testdata/api.txt has it: every
// name, its signature, and of a type (an alias's too: the core's type it
// stands for) its exported fields and methods. A change of the SDK's
// surface - the core's types it shows included - is a change of that
// file, made on purpose (go test ./modulekit -run TestAPI -update) and
// seen in review.
func TestAPI(t *testing.T) {
	imp := exportImporter(t)
	var b strings.Builder
	for _, path := range sdkPackages {
		pkg, err := imp.Import(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		writeAPI(&b, pkg)
	}
	got := b.String()
	golden := filepath.Join("testdata", "api.txt")
	if *updateAPI {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("the SDK's API differs from %s: if on purpose, go test ./modulekit -run TestAPI -update, and review the diff\n%s", golden, firstDiff(string(want), got))
	}
}

// exportImporter - an importer of the packages' compiled export data (go
// list -export: the build cache's), the SDK's and their dependencies'.
func exportImporter(t *testing.T) types.Importer {
	t.Helper()
	args := append([]string{"list", "-export", "-deps", "-f", "{{.ImportPath}}={{.Export}}"}, sdkPackages...)
	out, err := exec.Command("go", args...).Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	exports := map[string]string{}
	for line := range strings.Lines(string(out)) {
		if path, file, ok := strings.Cut(strings.TrimSpace(line), "="); ok && file != "" {
			exports[path] = file
		}
	}
	return importer.ForCompiler(token.NewFileSet(), "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	})
}

// qualifier - a package's path without the engine's module.
func qualifier(p *types.Package) string {
	return strings.TrimPrefix(p.Path(), "github.com/kosatka-cms/kosatka/")
}

// writeAPI writes pkg's exported names, sorted.
func writeAPI(b *strings.Builder, pkg *types.Package) {
	fmt.Fprintf(b, "package %s\n", qualifier(pkg))
	scope := pkg.Scope()
	names := scope.Names()
	sort.Strings(names)
	for _, name := range names {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		tn, ok := obj.(*types.TypeName)
		if !ok {
			fmt.Fprintf(b, "\t%s\n", types.ObjectString(obj, qualifier))
			continue
		}
		if tn.IsAlias() {
			fmt.Fprintf(b, "\ttype %s = %s\n", name, types.TypeString(types.Unalias(obj.Type()), qualifier))
		} else {
			fmt.Fprintf(b, "\ttype %s %s\n", name, kind(obj.Type()))
		}
		writeMembers(b, obj.Type())
	}
}

// kind - a named type's underlying type as its declaration shows it: a
// struct's and an interface's members are listed apart (writeMembers).
func kind(t types.Type) string {
	switch t.Underlying().(type) {
	case *types.Struct:
		return "struct"
	case *types.Interface:
		return "interface"
	}
	return types.TypeString(t.Underlying(), qualifier)
}

// writeMembers writes t's exported fields (a struct's) and methods (an
// interface's, or a named type's).
func writeMembers(b *strings.Builder, t types.Type) {
	method := func(name string, sig types.Type) {
		fmt.Fprintf(b, "\t\tmethod %s%s\n", name, strings.TrimPrefix(types.TypeString(sig, qualifier), "func"))
	}
	switch u := t.Underlying().(type) {
	case *types.Interface:
		for i := range u.NumMethods() {
			if m := u.Method(i); m.Exported() {
				method(m.Name(), m.Type())
			}
		}
		return
	case *types.Struct:
		if _, named := types.Unalias(t).(*types.Named); !named {
			return
		}
		for i := range u.NumFields() {
			if f := u.Field(i); f.Exported() {
				fmt.Fprintf(b, "\t\tfield %s %s\n", f.Name(), types.TypeString(f.Type(), qualifier))
			}
		}
	}
	if _, named := types.Unalias(t).(*types.Named); !named {
		return
	}
	ms := types.NewMethodSet(types.NewPointer(types.Unalias(t)))
	for m := range ms.Methods() {
		if m.Obj().Exported() {
			method(m.Obj().Name(), m.Type())
		}
	}
}

// firstDiff - the first lines where want and got part.
func firstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := range max(len(w), len(g)) {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("line %d:\n- %s\n+ %s", i+1, wl, gl)
		}
	}
	return ""
}
