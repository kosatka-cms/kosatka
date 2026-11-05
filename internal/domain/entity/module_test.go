package entity

import (
	"slices"
	"testing"
)

// withModules replaces the registry with ms for the test (the real
// modules register from internal/modules/manifests, which this package
// can't import).
func withModules(t *testing.T, ms ...Module) {
	t.Helper()
	saved := Modules
	t.Cleanup(func() { Modules = saved })
	Modules = nil
	for _, m := range ms {
		RegisterModule(m)
	}
}

// testModules - a forum-, quotes- and polls-like set.
func testModules() []Module {
	return []Module{
		{Name: "forum", NodeTypes: []ModuleNodeType{{Name: "forum", Label: "Forum", PathPrefix: "forum/", OwnDiscussion: true, EditPath: "/forum/topic/{id}/edit"}},
			AdminMenu: []ModuleMenuItem{{Section: AdminSectionConfiguration, Item: AdminMenuItem{Label: "Forum", Path: "/admin/config/forum", Group: AdminGroupModules}}}},
		{Name: "quotes", PluginBlocks: []ModulePluginBlock{{ID: "random_quote", Label: "Random quote"}},
			Permissions:  []ModulePermission{{Key: "manage quotes", Label: "Manage quotes"}},
			FileSources:  []ModuleFileSource{{Source: "quote", Table: "quotes", Label: "Quote \"@title\""}},
			CaptchaForms: []ModuleCaptchaForm{{Key: "quote", Label: "Quote form"}}},
		{Name: "polls", NodeTypes: []ModuleNodeType{{Name: "poll", Label: "Poll", CreatePath: "/create/poll"}},
			Permissions: []ModulePermission{{Key: "view poll votes", Label: "View poll votes"}}},
	}
}

// TestRegisterModule - a module registers once; a second one with the same
// name is a programming error.
func TestRegisterModule(t *testing.T) {
	withModules(t, testModules()...)
	if len(Modules) != 3 || Modules[0].Name != "forum" {
		t.Fatalf("registered: %+v", Modules)
	}
	defer func() {
		if recover() == nil {
			t.Error("registering a name twice must panic")
		}
	}()
	RegisterModule(Module{Name: "forum"})
}

// TestModuleLookups - what a module owns is found by its description.
func TestModuleLookups(t *testing.T) {
	withModules(t, testModules()...)
	if m, ok := ModuleOfNodeType("forum"); !ok || m != "forum" {
		t.Errorf("forum node type: %q %v", m, ok)
	}
	if _, ok := ModuleOfNodeType("article"); ok {
		t.Error("a core node type has a module")
	}
	if m, ok := ModuleOfPluginBlock(PluginBlockID("random_quote")); !ok || m != "quotes" {
		t.Errorf("random quote block: %q %v", m, ok)
	}
	if m, ok := ModuleOfPluginBlock(PluginBlockSearchForm); !ok || m != FeatureSearch {
		t.Errorf("search block: %q %v", m, ok)
	}
	if _, ok := ModuleOfPluginBlock(PluginBlockLogo); ok {
		t.Error("the logo block has a module")
	}
	if m, ok := ModuleOfPermission("view poll votes"); !ok || m != "polls" {
		t.Errorf("poll votes permission: %q %v", m, ok)
	}
}

// TestModuleRequirements - an enabled module needs the modules it
// requires; a missing state counts as enabled.
func TestModuleRequirements(t *testing.T) {
	saved := Modules
	t.Cleanup(func() { Modules = saved })
	Modules = append(append([]Module(nil), saved...), Module{Name: "extra", Requires: []string{"galleries"}})

	if missing := ModuleRequirementsMissing(map[string]bool{}); len(missing) != 0 {
		t.Errorf("all enabled: %v", missing)
	}
	if missing := ModuleRequirementsMissing(map[string]bool{"galleries": false}); len(missing["extra"]) != 1 || missing["extra"][0] != "galleries" {
		t.Errorf("galleries off: %v", missing)
	}
	if missing := ModuleRequirementsMissing(map[string]bool{"galleries": false, "extra": false}); len(missing) != 0 {
		t.Errorf("both off: %v", missing)
	}
	if got := ModulesRequiring("galleries"); len(got) != 1 || got[0] != "extra" {
		t.Errorf("required by: %v", got)
	}
}

// TestModuleNodeTypeMaps - the node type labels and create paths come
// from the module descriptions.
func TestModuleNodeTypeMaps(t *testing.T) {
	withModules(t, testModules()...)
	labels, paths := ModuleNodeTypeLabels(), ModuleNodeTypeCreatePaths()
	if labels["poll"] != "Poll" || labels["forum"] != "Forum" {
		t.Errorf("labels: %v", labels)
	}
	if paths["poll"] != "/create/poll" {
		t.Errorf("create paths: %v", paths)
	}
	if _, ok := paths["forum"]; ok {
		t.Error("forum topics have a standalone create path")
	}
	if got := NodeEditPath("forum", 7); got != "/forum/topic/7/edit" {
		t.Errorf("forum edit path: %s", got)
	}
	if got := NodeEditPath("article", 7); got != "/node/7/edit" {
		t.Errorf("core edit path: %s", got)
	}
	if got := NodeDeletePath("forum", 7); got != "/node/7/delete" {
		t.Errorf("forum delete path (none of its own): %s", got)
	}
	if got := OwnDiscussionNodeTypes(); len(got) != 1 || got[0] != "forum" {
		t.Errorf("own discussion: %v", got)
	}
}

// TestRegistryContributions - the modules' plugin blocks, admin pages,
// CAPTCHA forms and file sources join the core's.
func TestRegistryContributions(t *testing.T) {
	withModules(t, testModules()...)
	blocks := PluginBlocks()
	if blocks[len(blocks)-1] != "random_quote" || PluginBlockLabels()["random_quote"] != "Random quote" || PluginBlockLabels()[PluginBlockLogo] == "" {
		t.Errorf("plugin blocks: %v", blocks)
	}
	var forum AdminMenuItem
	for _, sec := range AdminMenu() {
		for _, it := range sec.Items {
			if it.Path == "/admin/config/forum" {
				if sec.Label != AdminSectionConfiguration {
					t.Errorf("forum page in %q", sec.Label)
				}
				forum = it
			}
		}
	}
	if forum.Module != "forum" {
		t.Errorf("the module's page must be tagged with it: %+v", forum)
	}
	names := CaptchaFormNames()
	if len(names) != len(CoreCaptchaForms)+1 || names[len(names)-1] != "quote" {
		t.Errorf("captcha forms: %v", names)
	}
	if s, ok := ModuleFileSourceByKey("quote"); !ok || s.Table != "quotes" {
		t.Errorf("file source: %+v %v", s, ok)
	}
}

// TestNodeTypesByLabel - the node types sorted by label, by machine name
// when two share one.
func TestNodeTypesByLabel(t *testing.T) {
	got := NodeTypesByLabel(map[string]string{"video": "Video", "poll": "Poll", "news": "News", "forum": "Forum", "talk": "Forum"})
	want := []ContentType{"forum", "talk", "news", "poll", "video"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestModuleCategoryName - a module's category when the engine knows it;
// none, or one it doesn't know, is "Other"; "Other" is the last group.
func TestModuleCategoryName(t *testing.T) {
	for cat, want := range map[string]string{
		ModuleCategoryCommunity: ModuleCategoryCommunity,
		"":                      ModuleCategoryOther,
		"games":                 ModuleCategoryOther,
	} {
		if got := (Module{Category: cat}).CategoryName(); got != want {
			t.Errorf("%q: %q, want %q", cat, got, want)
		}
	}
	cats := ModuleCategories()
	if cats[len(cats)-1].Name != ModuleCategoryOther {
		t.Errorf("the last group: %+v", cats[len(cats)-1])
	}
}
