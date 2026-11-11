package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

type stubModules struct{ states map[string]bool }

func (s *stubModules) States(context.Context) (map[string]bool, error) { return s.states, nil }

func (s *stubModules) SetEnabled(_ context.Context, name string, enabled bool) error {
	s.states[name] = enabled
	return nil
}

func (s *stubModules) CountRows(context.Context, string) (int, error) { return 0, nil }

func (s *stubModules) PluginPlacements(context.Context, []entity.PluginBlockID) ([]entity.PluginPlacementRef, error) {
	return nil, nil
}

func (s *stubModules) DeletePluginPlacements(context.Context, []entity.PluginBlockID) error {
	return nil
}

func (s *stubModules) DeleteMenuLinks(context.Context, []string) error { return nil }

// TestModulesPageSave - unchecked modules are stored as disabled, checked
// ones as enabled; the states apply at once and the change hook runs.
func TestModulesPageSave(t *testing.T) {
	t.Cleanup(func() {
		all := map[string]bool{}
		for _, m := range entity.Modules {
			all[m.Name] = true
		}
		service.SetModules(all)
	})
	repo := &stubModules{states: map[string]bool{}}
	changed := false
	ac := testController(interfaces.BaseDeps{Logger: testLogger()}, nil)
	ac.modules, ac.modulesChanged = repo, func() { changed = true }

	form := url.Values{"module_" + "polls": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/config/modules", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ac.ModulesPage(rec, req)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "saved=1") {
		t.Fatalf("redirect %q", loc)
	}
	if repo.states["galleries"] || !repo.states["polls"] {
		t.Errorf("stored %v", repo.states)
	}
	if service.ModuleEnabled("galleries") || !service.ModuleEnabled("polls") {
		t.Error("states not applied")
	}
	if !changed {
		t.Error("change hook not called")
	}
}

// TestAppearanceSettingsSave - the back to top switch is stored and
// applied; the Modules page doesn't touch feature switches.
func TestAppearanceSettingsSave(t *testing.T) {
	t.Cleanup(func() { service.SetModules(map[string]bool{entity.FeatureBackToTop: true}) })
	repo := &stubModules{states: map[string]bool{}}
	ac := testController(interfaces.BaseDeps{Logger: testLogger()}, nil)
	ac.modules, ac.modulesChanged = repo, nil

	req := httptest.NewRequest(http.MethodPost, "/admin/config/appearance/settings", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ac.AppearanceSettingsSave(rec, req)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "settings_saved=1") {
		t.Fatalf("redirect %q", loc)
	}
	if enabled, ok := repo.states[entity.FeatureBackToTop]; !ok || enabled || service.FeatureEnabled(entity.FeatureBackToTop) {
		t.Fatalf("back to top not switched off: %v", repo.states)
	}

	all := url.Values{}
	for _, m := range entity.Modules {
		all.Set("module_"+m.Name, "1")
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/config/modules", strings.NewReader(all.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ac.ModulesPage(httptest.NewRecorder(), req)
	if service.FeatureEnabled(entity.FeatureBackToTop) {
		t.Fatal("saving the Modules page reset a feature switch")
	}
}

// TestModuleDetails - the Modules page lists what a module includes:
// content types and data with counts, where its blocks are placed, its
// permissions with role counts, admin sections and files; empty kinds are
// left out and a failed count drops only the number.
func TestModuleDetails(t *testing.T) {
	m := entity.Module{
		Name:         "boards",
		NodeTypes:    []entity.ModuleNodeType{{Name: "board_topic", Label: "Topic"}},
		PluginBlocks: []entity.ModulePluginBlock{{ID: "latest_topics", Label: "Latest topics"}},
		Permissions:  []entity.ModulePermission{{Key: "moderate boards", Label: "Moderate boards"}},
		AdminMenu: []entity.ModuleMenuItem{{Section: entity.AdminSectionConfiguration,
			Item: entity.AdminMenuItem{Label: "Boards", Path: "/admin/config/boards"}}},
		Data: []entity.ModuleData{{Label: "Topics", Table: "topics"}, {Label: "Broken", Table: "broken"}},
		CSS:  []string{"boards"},
	}
	// Registered: its admin sections come from the registry's menu.
	saved := entity.Modules
	t.Cleanup(func() { entity.Modules = saved })
	entity.Modules = append(slices.Clone(saved), m)
	src := moduleDetailSources{
		countRows: func(_ context.Context, table string) (int, error) {
			if table == "broken" {
				return 0, errors.New("no table")
			}
			return 7, nil
		},
		pluginPlacements: func(context.Context, []entity.PluginBlockID) ([]entity.PluginPlacementRef, error) {
			return []entity.PluginPlacementRef{{PluginID: entity.PluginBlockID("latest_topics"), Theme: "site", Region: "sidebar"}}, nil
		},
		countNodes:  func(context.Context, string) (int, error) { return 12, nil },
		typeLabels:  map[string]string{"board_topic": "Board topic"},
		roles:       entity.Roles{{Permissions: map[string]any{"moderate boards": true}}, {Permissions: map[string]any{}}},
		regionLabel: func(_ string, r entity.Region) string { return "Sidebar" },
	}
	got := map[string][]string{}
	for _, d := range moduleDetails(context.Background(), m, src, testLogger()) {
		got[d.Heading] = d.Items
	}
	want := map[string][]string{
		"Content types":        {"Board topic - 12"},
		"Blocks":               {"Latest topics - site: Sidebar"},
		"Permissions":          {"Moderate boards - roles: 1"},
		"Admin sections":       {"Configuration → Boards"},
		"Data (kept when off)": {"Topics - 7", "Broken"},
		"Files":                {"boards.css"},
	}
	for heading, items := range want {
		if strings.Join(got[heading], "|") != strings.Join(items, "|") {
			t.Errorf("%s: %q, want %q", heading, got[heading], items)
		}
	}
	if len(got) != len(want) {
		t.Errorf("headings: %v", got)
	}

	// A built-in module type without a content_types row is named by
	// entity.ModuleNodeTypeLabels().
	src.typeLabels = map[string]string{}
	if d := moduleDetails(context.Background(), entity.Module{Name: "polls", NodeTypes: []entity.ModuleNodeType{{Name: "poll", Label: "Poll"}}}, src, testLogger()); len(d) != 1 || d[0].Items[0] != "Poll - 12" {
		t.Errorf("built-in type label: %+v", d)
	}

	if d := moduleDetails(context.Background(), entity.Module{Name: "empty"}, src, testLogger()); len(d) != 0 {
		t.Errorf("a module owning nothing: %+v", d)
	}
}

// TestModulesPageRequires - a save that would leave a required module off
// is refused, nothing is stored.
func TestModulesPageRequires(t *testing.T) {
	saved := entity.Modules
	t.Cleanup(func() { entity.Modules = saved })
	entity.Modules = append(append([]entity.Module(nil), saved...), entity.Module{Name: "extra", Label: "Extra", Requires: []string{"galleries"}})

	repo := &stubModules{states: map[string]bool{}}
	ac := testController(interfaces.BaseDeps{Logger: testLogger()}, nil)
	ac.modules, ac.modulesChanged = repo, nil
	form := url.Values{"module_extra": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/config/modules", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ac.ModulesPage(rec, req)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=requires") || !strings.Contains(loc, "module=extra") || !strings.Contains(loc, "needs="+"galleries") {
		t.Fatalf("redirect %q", loc)
	}
	if len(repo.states) != 0 {
		t.Errorf("saved despite the refusal: %v", repo.states)
	}
	if msg := moduleRequiresMessage(context.Background(), "extra", "galleries"); !strings.Contains(msg, "\"Galleries\"") {
		t.Errorf("message: %q", msg)
	}
	if msg := moduleRequiresMessage(context.Background(), "<script>", "x"); msg != "" {
		t.Errorf("unknown module in the message: %q", msg)
	}
}

// TestModulesPageInstalls - enabling a module installs it (the installer
// is called for every enabled module); a failed install saves nothing.
func TestModulesPageInstalls(t *testing.T) {
	t.Cleanup(func() {
		all := map[string]bool{}
		for _, m := range entity.Modules {
			all[m.Name] = true
		}
		service.SetModules(all)
	})
	post := func(install func(context.Context, string) error, form url.Values) (*stubModules, string) {
		repo := &stubModules{states: map[string]bool{}}
		ac := testController(interfaces.BaseDeps{Logger: testLogger()}, nil)
		ac.modules, ac.modulesChanged = repo, nil
		ac.installModule = install
		req := httptest.NewRequest(http.MethodPost, "/admin/config/modules", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		ac.ModulesPage(rec, req)
		return repo, rec.Header().Get("Location")
	}

	var installed []string
	repo, loc := post(func(_ context.Context, m string) error { installed = append(installed, m); return nil },
		url.Values{"module_" + "ratings": {"1"}})
	if !strings.Contains(loc, "saved=1") || len(installed) != 1 || installed[0] != "ratings" || !repo.states["ratings"] {
		t.Fatalf("install on enabling: %s %v %v", loc, installed, repo.states)
	}

	repo, loc = post(func(context.Context, string) error { return errors.New("no tables") },
		url.Values{"module_" + "ratings": {"1"}})
	if !strings.Contains(loc, "error=install") || !strings.Contains(loc, "module="+"ratings") || len(repo.states) != 0 {
		t.Errorf("failed install: %s %v", loc, repo.states)
	}
}

type stubRevoker struct{}

func (stubRevoker) Revoke(context.Context, []string) error { return nil }

// TestModuleUninstallPost - the module is uninstalled only when its
// machine name is typed; then the change hook runs and the Modules page
// reports it.
func TestModuleUninstallPost(t *testing.T) {
	t.Cleanup(func() { service.SetModules(map[string]bool{"ratings": true}) })
	service.SetModules(map[string]bool{"ratings": false})
	repo := &stubModules{states: map[string]bool{}}
	changed := false
	ac := testController(interfaces.BaseDeps{Logger: testLogger()}, nil)
	ac.modules, ac.modulesChanged = repo, func() { changed = true }
	installed := true
	ac.uninstall = &service.ModuleUninstall{
		Installed:   func(context.Context, string) (bool, error) { return installed, nil },
		DropSchema:  func(context.Context, string) error { installed = false; return nil },
		Modules:     repo,
		Permissions: stubRevoker{},
	}
	post := func(confirm string) string {
		form := url.Values{"confirm": {confirm}}
		req := httptest.NewRequest(http.MethodPost, "/admin/config/modules/ratings/uninstall", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetPathValue("name", "ratings")
		rec := httptest.NewRecorder()
		ac.ModuleUninstallPage(rec, req)
		return rec.Header().Get("Location")
	}
	if loc := post("Ratings"); !strings.Contains(loc, "error=confirm") || !installed || changed {
		t.Fatalf("wrong confirmation: %q, installed %v", loc, installed)
	}
	if loc := post("ratings"); loc != "/admin/config/modules?uninstalled=ratings" || installed || !changed {
		t.Fatalf("uninstall: %q, installed %v, changed %v", loc, installed, changed)
	}
	if loc := post("ratings"); !strings.Contains(loc, "error=run") {
		t.Errorf("second uninstall: %q", loc)
	}
}

type stubUninstallStore struct{ nodes []int }

func (s *stubUninstallStore) NodeIDsOfTypes(_ context.Context, _ []string, limit int) ([]int, error) {
	if len(s.nodes) < limit {
		return slices.Clone(s.nodes), nil
	}
	return slices.Clone(s.nodes[:limit]), nil
}
func (s *stubUninstallStore) FileIDs(context.Context, [][2]string) ([]int, error)         { return nil, nil }
func (s *stubUninstallStore) DeleteNodeTypeSettings(context.Context, []string) error      { return nil }
func (s *stubUninstallStore) DeleteMailTransportSettings(context.Context, []string) error { return nil }
func (s *stubUninstallStore) DeleteMailTexts(context.Context, string) error               { return nil }
func (s *stubUninstallStore) DeleteViewCriteria(context.Context, string) error            { return nil }
func (s *stubUninstallStore) RemoveTextFilters(context.Context, []string) (map[int][]string, error) {
	return nil, nil
}

func (s *stubUninstallStore) ClearFieldFormatters(context.Context, []string) error { return nil }

// TestModuleContentDelete - the module's content is deleted only when its
// machine name is typed; the page then reports how many items went.
func TestModuleContentDelete(t *testing.T) {
	t.Cleanup(func() { service.SetModules(map[string]bool{"polls": true}) })
	service.SetModules(map[string]bool{"polls": false})
	store := &stubUninstallStore{nodes: []int{4, 5}}
	ac := testController(interfaces.BaseDeps{Logger: testLogger()}, nil)
	ac.modules, ac.modulesChanged = &stubModules{states: map[string]bool{}}, nil
	ac.uninstall = &service.ModuleUninstall{
		Installed:   func(context.Context, string) (bool, error) { return true, nil },
		DropSchema:  func(context.Context, string) error { return nil },
		Modules:     &stubModules{states: map[string]bool{}},
		Permissions: stubRevoker{},
		Store:       store,
		DeleteNode: func(_ context.Context, id int) error {
			store.nodes = slices.DeleteFunc(store.nodes, func(n int) bool { return n == id })
			return nil
		},
	}
	post := func(confirm string) string {
		form := url.Values{"confirm": {confirm}}
		req := httptest.NewRequest(http.MethodPost, "/admin/config/modules/polls/delete-content", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetPathValue("name", "polls")
		rec := httptest.NewRecorder()
		ac.ModuleContentDelete(rec, req)
		return rec.Header().Get("Location")
	}
	if loc := post("Polls"); !strings.Contains(loc, "error=confirm") || len(store.nodes) != 2 {
		t.Fatalf("wrong confirmation: %q, nodes %v", loc, store.nodes)
	}
	if loc := post("polls"); loc != "/admin/config/modules/polls/uninstall?deleted=2" || len(store.nodes) != 0 {
		t.Fatalf("delete: %q, nodes %v", loc, store.nodes)
	}
}
