package service

import (
	"context"
	"errors"
	"html/template"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeModuleRepo struct {
	entity.ModuleRepository
	states map[string]bool
	err    error
}

func (f *fakeModuleRepo) States(context.Context) (map[string]bool, error) {
	return f.states, f.err
}

func (f *fakeModuleRepo) SetEnabled(_ context.Context, name string, enabled bool) error {
	f.states[name] = enabled
	return nil
}

// resetModules enables every module and feature.
func resetModules() {
	modulesMu.Lock()
	defer modulesMu.Unlock()
	disabledModules = map[string]bool{}
}

// withExtraModules adds modules to the registry for a test.
func withExtraModules(t *testing.T, mods ...entity.Module) {
	t.Helper()
	saved := entity.Modules
	entity.Modules = append(slices.Clone(saved), mods...)
	t.Cleanup(func() { entity.Modules = saved })
}

// withModules sets the module states for a test and restores all enabled.
func withModules(t *testing.T, states map[string]bool) {
	t.Helper()
	resetModules()
	SetModules(states)
	t.Cleanup(resetModules)
}

func TestModuleStates(t *testing.T) {
	withModules(t, nil)
	// Saving one name leaves the others as they are.
	SetModules(map[string]bool{entity.FeatureSearch: false})
	SetModules(map[string]bool{"ratings": true})
	if FeatureEnabled(entity.FeatureSearch) {
		t.Fatal("a feature state reset by saving another name")
	}
	resetModules()
	for _, m := range entity.Modules {
		if !ModuleEnabled(m.Name) {
			t.Errorf("%s: disabled with no stored state", m.Name)
		}
	}
	if err := LoadModules(t.Context(), &fakeModuleRepo{states: map[string]bool{"ratings": false, "polls": true}}); err != nil {
		t.Fatal(err)
	}
	if ModuleEnabled("ratings") || !ModuleEnabled("polls") || !ModuleEnabled("unknown") {
		t.Fatal("loaded states not applied")
	}
	// A failed load keeps the current states.
	if err := LoadModules(t.Context(), &fakeModuleRepo{err: errors.New("db down")}); err == nil {
		t.Fatal("expected the load error")
	}
	if ModuleEnabled("ratings") {
		t.Fatal("states reset by a failed load")
	}
}

func TestModuleOwnedItems(t *testing.T) {
	withModules(t, map[string]bool{"ratings": false, "polls": false})
	if NodeTypeEnabled("poll") || !NodeTypeEnabled("article") {
		t.Error("node types of a disabled module")
	}
	if !slices.Equal(DisabledNodeTypes(), []string{"poll"}) {
		t.Errorf("disabled node types = %v", DisabledNodeTypes())
	}
	rating := entity.PluginBlockID("node_rating")
	if PluginBlockEnabled(rating) || !PluginBlockEnabled(entity.PluginBlockLogo) {
		t.Error("plugin blocks of a disabled module")
	}
	if slices.Contains(EnabledPluginBlocks(), rating) || !slices.Contains(EnabledPluginBlocks(), entity.PluginBlockLogo) {
		t.Errorf("picker = %v", EnabledPluginBlocks())
	}

	SetModules(map[string]bool{"forms": false, "polls": false})
	if !slices.Equal(DisabledNodeTypes(), []string{"form", "poll"}) {
		t.Errorf("disabled node types = %v", DisabledNodeTypes())
	}
	SetModules(map[string]bool{"forms": true})

	p := entity.BlockPlacementDef{PluginID: &rating, VisibilityScope: entity.BlockVisibilityAll}
	if PlacementVisible(p, map[string]any{}) {
		t.Error("a placed block of a disabled module is shown")
	}
	resetModules()
	if !PlacementVisible(p, map[string]any{}) {
		t.Error("a placed block of an enabled module is hidden")
	}
}

func TestPermissionCatalogHidesDisabledModules(t *testing.T) {
	s, _ := newTestPermissions()
	withModules(t, map[string]bool{"ratings": false, "polls": false, "forms": false})
	catalog, err := s.Catalog(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	keys := catalogKeys(catalog)
	if slices.Contains(keys["General"], "rate content") || slices.Contains(keys["General"], "view poll votes") {
		t.Errorf("general = %v", keys["General"])
	}
	for _, label := range []string{"Poll", "Form"} {
		if _, ok := keys[label]; ok {
			t.Errorf("the %s node type permissions are listed", label)
		}
	}
	if !slices.Contains(keys["General"], PermAccessAdmin) {
		t.Error("core permissions hidden")
	}
}

func TestModulesPageRenders(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	type detail struct {
		Heading string
		Items   []string
	}
	type row struct {
		Name, Label, Description, UninstallBlocked string
		Enabled, Installed, External               bool
		Details                                    []detail
	}
	type group struct {
		Name, Label string
		Modules     []row
		On          int
	}
	data := map[string]any{"groups": []group{{Name: "content", Label: "Content", On: 1, Modules: []row{
		{Name: "galleries", Label: "Galleries", Description: "Image galleries.", Enabled: true,
			Details: []detail{{Heading: "Data", Items: []string{"Galleries - 12", "Gallery images - 3"}}}},
		{Name: "polls", Label: "Polls", Description: "Polls.", Enabled: false},
	}}}}
	var buf strings.Builder
	if err := svc.RenderNamed(&buf, "content", data, "admin/modules/index.html", "admin/alerts.html"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `name="module_galleries" value="1" class="form-check-input" checked`) || strings.Contains(out, `name="module_polls" value="1" class="form-check-input" checked`) {
		t.Fatalf("modules form:\n%s", out)
	}
	if !strings.Contains(out, "Galleries - 12<br>Gallery images - 3") || strings.Count(out, `class="admin-module-details"`) != 1 {
		t.Errorf("module details:\n%s", out)
	}
}

// TestOffByDefaultModule - a module that is off by default stays off
// without a state row and follows its row once there is one; others are
// on without a row.
func TestOffByDefaultModule(t *testing.T) {
	t.Cleanup(func() {
		modulesMu.Lock()
		disabledModules = defaultDisabledModules()
		modulesMu.Unlock()
	})
	modulesMu.Lock()
	disabledModules = map[string]bool{}
	modulesMu.Unlock()
	if err := LoadModules(context.Background(), &fakeModuleRepo{states: map[string]bool{}}); err != nil {
		t.Fatal(err)
	}
	if ModuleEnabled("forms") || !ModuleEnabled("polls") || NodeTypeEnabled("form") {
		t.Error("without rows: forms must be off, polls on")
	}
	if err := LoadModules(context.Background(), &fakeModuleRepo{states: map[string]bool{"forms": true}}); err != nil {
		t.Fatal(err)
	}
	if !ModuleEnabled("forms") {
		t.Error("enabled forms must be on")
	}
}

// TestPluginBlockTargets - a block registered with targets offers them; any
// other has none.
func TestPluginBlockTargets(t *testing.T) {
	SetPluginBlockTargets("test_targets", func(context.Context) ([]entity.PluginBlockTarget, error) {
		return []entity.PluginBlockTarget{{Value: "1", Label: "One"}}, nil
	})
	if got, ok, err := PluginBlockTargets(context.Background(), "test_targets"); err != nil || !ok || len(got) != 1 || got[0].Label != "One" {
		t.Errorf("registered: %v %v %v", got, ok, err)
	}
	if _, ok, _ := PluginBlockTargets(context.Background(), entity.PluginBlockLogo); ok {
		t.Error("the logo block has no targets")
	}
}
