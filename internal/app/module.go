package app

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// moduleUsage - "app module" (the modules from outside the engine,
// docs/modules/using-modules.md).
const moduleUsage = `usage:
  app module list                   the modules the binary has: the engine's and those from outside it, with their Go modules' versions
  app module add <path>[@version]   add a module from its Go module to the site (go get; latest without a version)
  app module remove [-force] <path> remove it from the site: refused while it is installed in the site's database (-config or CONFIG_PATH; uninstall it on the Modules page first), -force removes it anyway
add and remove work in the site's own main package (its go.mod and ` + modulesFile + `); rebuild the binary after them.`

// modulesFile - the file of the site's main package importing the modules
// from outside the engine it has (each registers itself when imported):
// written by "app module add" and "remove", never by hand.
const modulesFile = "kosatka_modules.go"

// engineModule - the engine's Go module: its own modules are built in.
const engineModule = "github.com/kosatka-cms/kosatka"

// importPathRe / versionRe - a module's Go package path (a host, then its
// path) and a version ("" - the latest): written into the site's Go source
// and handed to go get, so nothing else is taken.
var (
	importPathRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*\.[a-zA-Z]{2,}(/[a-zA-Z0-9._~+-]+)+$`)
	versionRe    = regexp.MustCompile(`^[a-zA-Z0-9._+-]*$`)
)

// goCommand runs the go tool in dir (a variable: the tests replace it).
var goCommand = func(dir string, stdout, stderr io.Writer, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, stdout, stderr
	return cmd.Run()
}

// runModule is "app module …".
func runModule(args []string) int { return moduleCommand(args, ".", os.Stdout, os.Stderr) }

// moduleCommand runs "app module …" in dir (the site's main package).
func moduleCommand(args []string, dir string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, moduleUsage)
		return 2
	}
	fail := func(err error) int {
		_, _ = fmt.Fprintln(stderr, "module:", err)
		return 1
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet("module "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	force := false
	if cmd == "remove" {
		fs.BoolVar(&force, "force", false, "remove the module even if it is installed in the site's database")
	}
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	switch {
	case cmd == "list" && fs.NArg() == 0:
		moduleList(stdout)
		return 0
	case (cmd == "add" || cmd == "remove") && fs.NArg() == 1:
		path, version, _ := strings.Cut(fs.Arg(0), "@")
		if !importPathRe.MatchString(path) || !versionRe.MatchString(version) {
			return fail(fmt.Errorf("%q: not a Go package path[@version]", fs.Arg(0)))
		}
		if err := checkSiteModule(dir); err != nil {
			return fail(err)
		}
		imports, err := readModulesFile(dir)
		if err != nil {
			return fail(err)
		}
		before := maps.Clone(imports)
		get := path + "@none"
		if cmd == "add" {
			if version == "" {
				version = "latest"
			}
			imports[path], get = true, path+"@"+version
		} else {
			if !imports[path] {
				return fail(fmt.Errorf("%s isn't in %s", path, modulesFile))
			}
			if !force {
				if err := checkUninstalled(path, stderr); err != nil {
					return fail(err)
				}
			}
			delete(imports, path)
		}
		// The file first: go get then finds the package imported (a direct
		// requirement of the site's go.mod); it fails - the file as it was.
		if err := writeModulesFile(dir, imports); err != nil {
			return fail(err)
		}
		if err := goCommand(dir, stdout, stderr, "get", get); err != nil {
			if werr := writeModulesFile(dir, before); werr != nil {
				err = errors.Join(err, werr)
			}
			return fail(fmt.Errorf("go get %s: %w", get, err))
		}
		if cmd == "remove" {
			_, _ = fmt.Fprintln(stdout, "rebuild the binary and deploy it: the module and its pages are gone")
			return 0
		}
		if err := markDirect(dir, path); err != nil {
			return fail(err)
		}
		_, _ = fmt.Fprintln(stdout, "rebuild the binary and deploy it, then run app migrate -apply: it installs the module (one off by default: enable it on the Modules page)")
		return 0
	default:
		_, _ = fmt.Fprintln(stderr, moduleUsage)
		return 2
	}
}

// siteModulesInstalled - which of the modules are installed in the site's
// database (its config: -config before the command or CONFIG_PATH); an
// error - it couldn't be checked (a variable: the tests replace it).
var siteModulesInstalled = func(names []string) (map[string]bool, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	db, _, err := NewEntConn(&cfg.DB)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r := migrations.Runner{DB: db, Dialect: sqldialect.Dialect(cfg.DB.Dialect())}
	out := map[string]bool{}
	for _, name := range names {
		applied, err := r.Applied(ctx, name)
		if err != nil {
			return nil, err
		}
		out[name] = len(applied) > 0
	}
	return out, nil
}

// moduleRegistrations - the modules from outside the engine the binary
// has (a variable: the tests replace it).
var moduleRegistrations = modulekit.Registered

// packageModules - the modules the Go package path registers (their Wire
// in it or under it).
func packageModules(path string) []string {
	var out []string
	for _, r := range moduleRegistrations() {
		if pkg := registrationPackage(r); pkg == path || strings.HasPrefix(pkg, path+"/") {
			out = append(out, r.Module.Name)
		}
	}
	return out
}

// checkUninstalled - the modules from path aren't installed in the site's
// database: a binary without one can't uninstall it, its data would stay.
// A database that can't be checked (no config here, say) - a warning, and
// the removal goes on.
func checkUninstalled(path string, stderr io.Writer) error {
	names := packageModules(path)
	if len(names) == 0 {
		_, _ = fmt.Fprintf(stderr, "module: this binary has no module from %s to check: make sure it is uninstalled on the Modules page\n", path)
		return nil
	}
	installed, err := siteModulesInstalled(names)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "module: couldn't check the site's database (%v): make sure %s is uninstalled on the Modules page\n", err, strings.Join(names, ", "))
		return nil
	}
	for _, name := range names {
		if installed[name] {
			return fmt.Errorf("%s is installed in the site's database: uninstall it on the Modules page first (-force removes it anyway: its data stays, and a binary without the module can't uninstall it)", name)
		}
	}
	return nil
}

// moduleList prints the modules the binary has: the engine's own, then
// each from outside it with the Go package it came from and its version.
func moduleList(w io.Writer) {
	external := map[string]string{}
	for _, r := range modulekit.Registered() {
		external[r.Module.Name] = registrationPackage(r)
	}
	for _, m := range entity.Modules {
		pkg, ok := external[m.Name]
		if !ok {
			_, _ = fmt.Fprintf(w, "%-16s the engine's\n", m.Name)
			continue
		}
		_, _ = fmt.Fprintf(w, "%-16s %s %s\n", m.Name, pkg, packageVersion(pkg))
	}
}

// registrationPackage - the Go package a module from outside the engine
// registered from (its Wire's), "" when unknown.
func registrationPackage(r modulekit.Registration) string {
	if r.Wire == nil {
		return ""
	}
	fn := runtime.FuncForPC(reflect.ValueOf(r.Wire).Pointer())
	if fn == nil {
		return ""
	}
	name := fn.Name()
	if i := strings.LastIndex(name, "/"); i >= 0 {
		if j := strings.Index(name[i:], "."); j >= 0 {
			return name[:i+j]
		}
	}
	if j := strings.Index(name, "."); j >= 0 {
		return name[:j]
	}
	return name
}

// packageVersion - the version of the Go module the binary took pkg from
// (its build information), "(devel)" for one replaced by a directory.
func packageVersion(pkg string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	best := ""
	version := ""
	for _, d := range info.Deps {
		if (pkg == d.Path || strings.HasPrefix(pkg, d.Path+"/")) && len(d.Path) > len(best) {
			best, version = d.Path, d.Version
			if d.Replace != nil {
				version = d.Replace.Version
				if version == "" {
					version = "(devel) => " + d.Replace.Path
				}
			}
		}
	}
	return version
}

// goModRe - a go.mod's module line.
var goModRe = regexp.MustCompile(`(?m)^module\s+(\S+)`)

// checkSiteModule - dir is a site's own main package: a Go module of its
// own (go.mod), not the engine (whose modules are built in).
func checkSiteModule(dir string) error {
	b, err := os.ReadFile(dir + "/go.mod")
	if err != nil {
		return fmt.Errorf("no go.mod here: run it in the site's own main package (%w)", err)
	}
	m := goModRe.FindSubmatch(b)
	if m == nil {
		return errors.New("go.mod names no module")
	}
	if string(m[1]) == engineModule {
		return errors.New("this is the engine: its modules are built in; add modules to a site's own main package")
	}
	return nil
}

// indirectRe - a go.mod requirement marked indirect.
var indirectRe = regexp.MustCompile(`^(\s*(?:require\s+)?(\S+)\s+\S+)\s*// indirect\s*$`)

// markDirect drops the "// indirect" of the requirement of dir's go.mod
// providing pkg: the site imports it (modulesFile), as go mod tidy would
// put it.
func markDirect(dir, pkg string) error {
	name := dir + "/go.mod"
	b, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for i, l := range lines {
		if m := indirectRe.FindStringSubmatch(l); m != nil && (pkg == m[2] || strings.HasPrefix(pkg, m[2]+"/")) {
			lines[i] = m[1]
		}
	}
	return os.WriteFile(name, []byte(strings.Join(lines, "\n")), 0o644)
}

// modulesImportRe - an import of modulesFile.
var modulesImportRe = regexp.MustCompile(`^\s*_\s+"([^"]+)"`)

// readModulesFile - the packages modulesFile of dir imports (none
// without the file).
func readModulesFile(dir string) (map[string]bool, error) {
	out := map[string]bool{}
	f, err := os.Open(dir + "/" + modulesFile)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := modulesImportRe.FindStringSubmatch(sc.Text()); m != nil {
			out[m[1]] = true
		}
	}
	return out, sc.Err()
}

// writeModulesFile writes modulesFile of dir importing the packages, in
// order.
func writeModulesFile(dir string, imports map[string]bool) error {
	paths := make([]string, 0, len(imports))
	for p := range imports {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b strings.Builder
	b.WriteString("// Code generated by \"app module\"; DO NOT EDIT.\n\n")
	b.WriteString("// The modules from outside the engine this site has: each registers\n")
	b.WriteString("// itself when imported (modulekit.Register).\n\n")
	b.WriteString("package main\n")
	if len(paths) > 0 {
		b.WriteString("\nimport (\n")
		for _, p := range paths {
			b.WriteString("\t_ " + strconv.Quote(p) + "\n")
		}
		b.WriteString(")\n")
	}
	return os.WriteFile(dir+"/"+modulesFile, []byte(b.String()), 0o644)
}
