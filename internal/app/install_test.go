package app

import (
	"maps"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestInstallModules - -modules picks the modules a new site has: the
// standard ones (InstallByDefault) when empty, "all", "none", or names
// with the modules they require; an unknown name is an error.
func TestInstallModules(t *testing.T) {
	standard, all := map[string]bool{}, map[string]bool{}
	for _, m := range entity.Modules {
		all[m.Name] = true
		if m.InstallByDefault {
			standard[m.Name] = true
		}
	}
	if len(standard) == 0 || len(standard) == len(all) {
		t.Fatalf("standard modules %v of %d: want some, not all", standard, len(all))
	}
	saved := entity.Modules
	t.Cleanup(func() { entity.Modules = saved })
	// A module that needs another, which needs a third.
	entity.Modules = append(slices.Clone(saved),
		entity.Module{Name: "example_top", Requires: []string{"example_mid"}},
		entity.Module{Name: "example_mid", Requires: []string{"example_base"}},
		entity.Module{Name: "example_base"})
	all["example_top"], all["example_mid"], all["example_base"] = true, true, true

	for value, want := range map[string]map[string]bool{
		"":                      standard,
		"  ":                    standard,
		"all":                   all,
		"none":                  {},
		" ratings , polls,":     {"ratings": true, "polls": true},
		"example_top":           {"example_top": true, "example_mid": true, "example_base": true},
		"example_mid,forms":     {"example_mid": true, "example_base": true, "forms": true},
		"example_base":          {"example_base": true},
		"ratings,ratings":       {"ratings": true},
		"error_reports,example": nil,
	} {
		got, err := installModules(value, nil)
		if want == nil {
			if err == nil {
				t.Errorf("%q: %v, want an error", value, got)
			}
			continue
		}
		if err != nil || !maps.Equal(got, want) {
			t.Errorf("%q: %v %v, want %v", value, got, err, want)
		}
	}
}

// TestInstallModulesDemoSet - without -modules a demo set's modules come
// with the standard ones (and what they require); -modules overrides
// them; an unknown one is an error.
func TestInstallModulesDemoSet(t *testing.T) {
	standard, err := installModules("", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := installModules("", []string{"ratings"})
	if err != nil || !got["ratings"] || len(got) <= len(standard) {
		t.Errorf("with the set's ratings: %v %v", got, err)
	}
	if got, err := installModules("polls", []string{"ratings"}); err != nil || got["ratings"] || !got["polls"] {
		t.Errorf("-modules polls: %v %v", got, err)
	}
	if _, err := installModules("", []string{"nope"}); err == nil {
		t.Error("an unknown module of the set: no error")
	}
}
