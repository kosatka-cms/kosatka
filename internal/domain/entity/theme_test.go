package entity

import (
	"slices"
	"testing"
)

// TestThemeInfoRegions - "Hidden" appears only in the admin list
// (RegionIDsWithDisabled, Allows, Labels), never among the regions render
// paths resolve (RegionIDs, Has): a hidden placement must stay invisible.
func TestThemeInfoRegions(t *testing.T) {
	info := ThemeInfo{Regions: []ThemeRegion{{ID: "sidebar", Label: "Sidebar"}, {ID: "footer", Label: "Footer"}}}
	if got := info.RegionIDs(); !slices.Equal(got, []Region{"sidebar", "footer"}) {
		t.Fatalf("RegionIDs = %v", got)
	}
	if got := info.RegionIDsWithDisabled(); !slices.Equal(got, []Region{"sidebar", "footer", RegionDisabled}) {
		t.Fatalf("RegionIDsWithDisabled = %v", got)
	}
	if got := info.RegionIDs(); slices.Contains(got, RegionDisabled) {
		t.Fatal("RegionIDsWithDisabled must not write into RegionIDs")
	}
	if info.Has(RegionDisabled) || !info.Allows(RegionDisabled) || !info.Has("footer") || info.Allows("header_nav") {
		t.Fatal("Has/Allows")
	}
	if l := info.Labels(); l["footer"] != "Footer" || l[RegionDisabled] != "Hidden" || len(l) != 3 {
		t.Fatalf("Labels = %v", l)
	}
}

func TestThemeKeyDir(t *testing.T) {
	if ThemeKey("") != DefaultTheme || ThemeKey("site") != "site" || ThemeDir(DefaultTheme) != "" || ThemeDir("site") != "site" {
		t.Error("theme key/dir mapping")
	}
}
