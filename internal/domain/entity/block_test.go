package entity

import (
	"reflect"
	"testing"
)

func TestPlacementVisibleToRoles(t *testing.T) {
	p := BlockPlacementDef{VisibilityRoles: " moderator , admin "}
	if !p.VisibleToRoles([]string{"user", "admin"}) || p.VisibleToRoles([]string{"user"}) || p.VisibleToRoles(nil) {
		t.Error("roles condition")
	}
	if !(BlockPlacementDef{}).VisibleToRoles(nil) {
		t.Error("no condition must show to everyone")
	}
	if !p.HasRole("moderator") || p.HasRole("user") {
		t.Error("HasRole")
	}
	if got := UserRoleNames(nil); len(got) != 1 || got[0] != RoleAnonymous {
		t.Errorf("guest roles %v", got)
	}
	if got := UserRoleNames(&User{Role: &Role{Name: "moderator"}}); len(got) != 2 || got[1] != "moderator" {
		t.Errorf("moderator roles %v", got)
	}
}

func TestBlockPlacementVisibilityContentTypes(t *testing.T) {
	v := " article,group ,,events"
	p := BlockPlacementDef{VisibilityContentType: &v}
	if got, want := p.VisibilityContentTypes(), []string{"article", "group", "events"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("VisibilityContentTypes() = %v, want %v", got, want)
	}
	for typ, want := range map[string]bool{"article": true, "group": true, "events": true, "video": false, "": false} {
		if got := p.MatchesContentType(typ); got != want {
			t.Fatalf("MatchesContentType(%q) = %v, want %v", typ, got, want)
		}
	}
	if (BlockPlacementDef{}).MatchesContentType("article") {
		t.Fatal("unset VisibilityContentType must match nothing")
	}
}

// TestPluginSettingOr - a placement's setting while valid, else its
// default (unset, a number out of range, an option gone).
func TestPluginSettingOr(t *testing.T) {
	count := PluginBlockSetting{Key: "count", Type: PluginBlockSettingNumber, Default: "10", Min: 1, Max: 100}
	vocabulary := PluginBlockSetting{Key: "vocabulary", Type: PluginBlockSettingSelect, Default: "0", Options: []PluginBlockTarget{{Value: "0"}, {Value: "3"}}}
	for config, want := range map[string][2]string{
		"":        {"10", "0"},
		"valid":   {"25", "3"},
		"invalid": {"10", "0"},
	} {
		p := BlockPlacementDef{}
		switch config {
		case "valid":
			p.PluginConfig = map[string]string{"count": "25", "vocabulary": "3"}
		case "invalid":
			p.PluginConfig = map[string]string{"count": "101", "vocabulary": "7"}
		}
		if got := [2]string{p.PluginSettingOr(count), p.PluginSettingOr(vocabulary)}; got != want {
			t.Errorf("%q: %v, want %v", config, got, want)
		}
	}
	if count.Check("x") || count.Check("0") || !count.Check("1") {
		t.Error("a number setting checks its range")
	}
}

// TestPluginBlockSettingText - a text setting: empty or a line of at
// most Max characters (runes, not bytes), no line breaks.
func TestPluginBlockSettingText(t *testing.T) {
	holder := PluginBlockSetting{Key: "holder", Type: PluginBlockSettingText, Max: 5}
	for value, want := range map[string]bool{"": true, "Пятёрк": false, "Пятёр": true, "a\nb": false, "a\tb": false} {
		if got := holder.Check(value); got != want {
			t.Errorf("Check(%q) = %v, want %v", value, got, want)
		}
	}
}
