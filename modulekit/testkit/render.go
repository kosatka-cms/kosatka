package testkit

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

// EngineRoot - the engine's source tree, whose templates (web/templates),
// themes and translations a module's rendering tests read: this package's
// directory two levels up - the engine's repository, or the engine's
// module in the module cache for a module from its own repository.
func EngineRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// UseEngineRoot makes the engine's source tree the test's working
// directory, where the template service reads from, and the modules'
// files where a site finds them: an engine module's directory under the
// engine's, a module from its own repository's assets of its
// registration.
func UseEngineRoot(t testing.TB) {
	t.Helper()
	root, err := filepath.Abs(EngineRoot())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	assets := map[string]fs.FS{}
	for _, r := range modulekit.Registered() {
		if r.Assets != nil {
			assets[r.Module.Name] = r.Assets
		}
	}
	paths.SetModuleAssets(assets, false)
	t.Cleanup(func() { paths.SetModuleAssets(nil, false) })
}

// RegisterModules puts the engine's modules in the registry (a module
// from its own repository registers itself, imported by the test).
func RegisterModules() { manifests.Register() }

// WithModules switches the modules on (true) and off for the test, the
// engine's registered; their states are restored after it.
func WithModules(t testing.TB, states map[string]bool) {
	t.Helper()
	RegisterModules()
	prev := make(map[string]bool, len(states))
	for name := range states {
		prev[name] = service.ModuleEnabled(name)
	}
	service.SetModules(states)
	t.Cleanup(func() { service.SetModules(prev) })
}

// Templates - the template service a controller of a handler test
// renders with (modulekit.BaseDeps.Templates): the site's, uncached.
func Templates() modulekit.TemplateService { return service.NewTemplateService(false) }

// RenderPageBlock - the block of a page template (a template file and
// what it parses with it: the layout, the partials, the modules' hooks),
// as rendering the page renders it; a failure fails the test.
func RenderPageBlock(t testing.TB, page, block string, data any) string {
	t.Helper()
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	if err := ts.RenderPageBlock(&buf, page, block, data); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// RenderPageContent - the "content" block of a page template.
func RenderPageContent(t testing.TB, page string, data any) string {
	t.Helper()
	return RenderPageBlock(t, page, "content", data)
}

// PageData - a whole page's data with no blocks placed in its regions,
// for rendering a layout (RenderPageBlock with "layout.html"); more is
// added to it by the test.
func PageData() map[string]any {
	return map[string]any{"blockPlacements": map[entity.Region][]entity.BlockWithPlacement{}}
}

// RenderNamed - the template name defined in files (template paths, as
// the template service resolves them).
func RenderNamed(t testing.TB, name string, data any, files ...string) string {
	t.Helper()
	var buf strings.Builder
	if err := service.NewTemplateService(false).RenderNamed(&buf, name, data, files...); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// RenderPlacement - the region of p with that one block, as a page
// renders its regions: a plugin block through its module's template
// (block_<id>), its PluginConfig as placed. pageData - the rest of the
// page's data the block reads (nil - none).
func RenderPlacement(t testing.TB, p modulekit.BlockPlacement, pageData map[string]any) string {
	t.Helper()
	ctx := map[string]any{}
	for k, v := range pageData {
		ctx[k] = v
	}
	ctx["blockPlacements"] = map[entity.Region][]entity.BlockWithPlacement{p.Region: {{Placement: p}}}
	return RenderNamed(t, "region_blocks", map[string]any{"Region": p.Region, "Ctx": ctx}, service.RegionBlocksStandaloneFiles...)
}

// UseTheme makes theme, in themesDir, the site's theme for the test
// (themesDir "" - the engine's themes directory).
func UseTheme(t testing.TB, themesDir, theme string) {
	t.Helper()
	paths.SetThemesDir(themesDir)
	paths.SetTheme(theme)
	t.Cleanup(func() {
		paths.SetTheme("")
		paths.SetThemesDir("")
	})
}

// UseModuleAssets serves the modules' templates and static files from
// their assets (the binary's embedded files), as on a site, for the test.
func UseModuleAssets(t testing.TB, assets map[string]fs.FS) {
	t.Helper()
	paths.SetModuleAssets(assets, true)
	t.Cleanup(func() { paths.SetModuleAssets(nil, false) })
}

// ResolveTemplate - where the template path resolves ("embed:…" for a
// module's embedded one).
func ResolveTemplate(path string) (string, error) { return paths.ResolveTemplate(path) }

// UseImageStyleFormats gives the image styles their output formats (as
// the image styles page sets them) for the test.
func UseImageStyleFormats(t testing.TB, formats map[string]imagestyle.Format) {
	t.Helper()
	imageurl.SetStyleFormats(formats)
	t.Cleanup(func() { imageurl.SetStyleFormats(nil) })
}

// UseImageStyleVersions gives the image styles their versions in the
// derivatives' URLs (?v=…, as the site sets them from the styles'
// settings) for the test.
func UseImageStyleVersions(t testing.TB, versions map[string]string) {
	t.Helper()
	imageurl.SetStyleVersions(versions)
	t.Cleanup(func() { imageurl.SetStyleVersions(nil) })
}
