package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/filestorage"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// TestModuleAddRemove - "app module add" writes the site's modules file
// importing the package, runs go get and marks the requirement direct;
// "remove" drops the import and the requirement; a failed go get leaves
// the file as it was; the engine's own tree is refused.
func TestModuleAddRemove(t *testing.T) {
	dir := t.TempDir()
	goMod := "module example.com/site\n\ngo 1.27\n\nrequire (\n\tgithub.com/kosatka-cms/kosatka v0.0.0\n\texample.com/mod v1.2.0 // indirect\n)\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	fail := false
	saved := goCommand
	t.Cleanup(func() { goCommand = saved })
	goCommand = func(d string, _, _ io.Writer, args ...string) error {
		if d != dir {
			t.Errorf("go ran in %s", d)
		}
		calls = append(calls, args)
		if fail {
			return errors.New("no such module")
		}
		return nil
	}
	run := func(args ...string) int { return moduleCommand(args, dir, io.Discard, io.Discard) }
	imports := func() []string {
		got, err := readModulesFile(dir)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for p := range got {
			out = append(out, p)
		}
		slices.Sort(out)
		return out
	}

	// Written into the site's Go source and handed to go get: a package
	// path and a version only.
	for _, bad := range []string{"example.com/x\"\nimport \"os", "example.com", "example.com/x@v1 -x", "../x", "example.com/x y"} {
		if code := run("add", bad); code != 1 {
			t.Errorf("add %q: %d, want refused", bad, code)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("go ran for a refused path: %v", calls)
	}
	if code := run("add", "example.com/mod/hello@v1.2.0"); code != 0 {
		t.Fatalf("add: %d", code)
	}
	if code := run("add", "example.com/other"); code != 0 {
		t.Fatalf("add without a version: %d", code)
	}
	if got := imports(); !slices.Equal(got, []string{"example.com/mod/hello", "example.com/other"}) {
		t.Errorf("imports after add: %v", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "go.mod")); !strings.Contains(string(b), "\texample.com/mod v1.2.0\n") {
		t.Errorf("the requirement stays indirect:\n%s", b)
	}
	if code := run("remove", "example.com/other"); code != 0 {
		t.Fatalf("remove: %d", code)
	}
	if got := imports(); !slices.Equal(got, []string{"example.com/mod/hello"}) {
		t.Errorf("imports after remove: %v", got)
	}
	want := [][]string{{"get", "example.com/mod/hello@v1.2.0"}, {"get", "example.com/other@latest"}, {"get", "example.com/other@none"}}
	if !slices.EqualFunc(calls, want, slices.Equal[[]string]) {
		t.Errorf("go: %v, want %v", calls, want)
	}

	fail = true
	if code := run("add", "example.com/broken"); code != 1 {
		t.Errorf("a failed go get: %d", code)
	}
	if got := imports(); !slices.Equal(got, []string{"example.com/mod/hello"}) {
		t.Errorf("a failed add changed the file: %v", got)
	}
	if code := run("remove", "example.com/not-there"); code != 1 {
		t.Errorf("removing a module the site hasn't: %d", code)
	}

	engine := t.TempDir()
	if err := os.WriteFile(filepath.Join(engine, "go.mod"), []byte("module "+engineModule+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := moduleCommand([]string{"add", "example.com/mod"}, engine, io.Discard, io.Discard); code != 1 {
		t.Errorf("add in the engine's tree: %d", code)
	}
	if code := moduleCommand([]string{"add", "example.com/mod"}, t.TempDir(), io.Discard, io.Discard); code != 1 {
		t.Errorf("add without a go.mod: %d", code)
	}
}

// TestModuleRemoveInstalled - "app module remove" refuses a module the
// site's database still has installed and leaves the file as it was;
// -force removes it anyway; a database that can't be checked - a
// warning, and it is removed.
func TestModuleRemoveInstalled(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/site\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := registrationPackage(modulekit.Registration{Wire: testWire})
	savedGo, savedRegs, savedInstalled := goCommand, moduleRegistrations, siteModulesInstalled
	t.Cleanup(func() { goCommand, moduleRegistrations, siteModulesInstalled = savedGo, savedRegs, savedInstalled })
	goCommand = func(string, io.Writer, io.Writer, ...string) error { return nil }
	moduleRegistrations = func() []modulekit.Registration {
		return []modulekit.Registration{{Module: manifestkit.Module{Name: "greetings"}, Wire: testWire}}
	}
	var checkErr error
	var asked []string
	siteModulesInstalled = func(names []string) (map[string]bool, error) {
		asked = names
		return map[string]bool{"greetings": true}, checkErr
	}
	remove := func(args ...string) (int, string) {
		t.Helper()
		if err := writeModulesFile(dir, map[string]bool{pkg: true}); err != nil {
			t.Fatal(err)
		}
		var stderr strings.Builder
		return moduleCommand(append([]string{"remove"}, args...), dir, io.Discard, &stderr), stderr.String()
	}
	kept := func() bool {
		got, err := readModulesFile(dir)
		if err != nil {
			t.Fatal(err)
		}
		return got[pkg]
	}

	if code, stderr := remove(pkg); code != 1 || !strings.Contains(stderr, "greetings is installed in the site's database") || !kept() {
		t.Errorf("an installed module: %d %q, kept %v", code, stderr, kept())
	}
	if !slices.Equal(asked, []string{"greetings"}) {
		t.Errorf("checked %v, want the module the package registers", asked)
	}
	if code, _ := remove("-force", pkg); code != 0 || kept() {
		t.Errorf("-force: %d, kept %v", code, kept())
	}
	checkErr = errors.New("no config")
	if code, stderr := remove(pkg); code != 0 || !strings.Contains(stderr, "couldn't check the site's database (no config)") || kept() {
		t.Errorf("no database to check: %d %q, kept %v", code, stderr, kept())
	}
}

// testWire - a module's Wire in this package (TestModuleRemoveInstalled).
func testWire(modulekit.Deps) modulekit.Parts { return modulekit.Parts{} }

// TestModuleRemoveStorage - "app module remove" refuses a package whose
// file storage the site keeps its files in, -force removes it anyway; a
// config that can't be read - a warning, and it is removed. app module
// list shows the binary's storages.
func TestModuleRemoveStorage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/site\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	filestorage.Register(filestorage.Driver{Name: "teststore", Public: testStorePublic})
	t.Cleanup(func() { filestorage.Unregister("teststore") })
	pkg := funcPackage(testStorePublic)
	savedGo, savedRegs, savedStorages := goCommand, moduleRegistrations, siteStorages
	t.Cleanup(func() { goCommand, moduleRegistrations, siteStorages = savedGo, savedRegs, savedStorages })
	goCommand = func(string, io.Writer, io.Writer, ...string) error { return nil }
	moduleRegistrations = func() []modulekit.Registration { return nil }
	used, readErr := []string{"local", "teststore"}, error(nil)
	siteStorages = func() ([]string, error) { return used, readErr }
	remove := func(args ...string) (int, string) {
		t.Helper()
		if err := writeModulesFile(dir, map[string]bool{pkg: true}); err != nil {
			t.Fatal(err)
		}
		var stderr strings.Builder
		return moduleCommand(append([]string{"remove"}, args...), dir, io.Discard, &stderr), stderr.String()
	}

	if code, stderr := remove(pkg); code != 1 || !strings.Contains(stderr, "keeps its files in teststore") {
		t.Errorf("a storage in use: %d %q", code, stderr)
	}
	if code, _ := remove("-force", pkg); code != 0 {
		t.Errorf("-force: %d", code)
	}
	used = []string{"local", "local"}
	if code, stderr := remove(pkg); code != 0 || stderr != "" {
		t.Errorf("a storage not in use: %d %q", code, stderr)
	}
	readErr = errors.New("no config")
	if code, stderr := remove(pkg); code != 0 || !strings.Contains(stderr, "couldn't read the site's config (no config)") {
		t.Errorf("no config to read: %d %q", code, stderr)
	}

	var list strings.Builder
	moduleList(&list)
	if !strings.Contains(list.String(), "teststore        file storage: "+pkg) {
		t.Errorf("list: %s", list.String())
	}
}

// testStorePublic - a file storage's Public in this package
// (TestModuleRemoveStorage).
func testStorePublic(filestorage.Config) (filestore.Store, string, error) { return nil, "", nil }
