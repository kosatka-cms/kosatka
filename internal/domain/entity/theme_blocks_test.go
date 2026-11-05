package entity

import "testing"

// TestThemeInfoCheckBlocks - a theme's block is a plugin block or a custom
// one with a title and a body, in one of the theme's regions.
func TestThemeInfoCheckBlocks(t *testing.T) {
	regions := []ThemeRegion{{ID: "footer"}, {ID: "footer_bottom"}}
	ok := ThemeInfo{Regions: regions, Blocks: []ThemeBlock{
		{Plugin: PluginBlockCopyright, Region: "footer_bottom"},
		{Title: "Buttons", Body: "<p>88×31</p>", Region: "footer", Weight: -10},
	}}
	if err := ok.CheckBlocks(); err != nil {
		t.Fatalf("valid blocks: %v", err)
	}
	for name, b := range map[string]ThemeBlock{
		"a plugin block with a body":   {Plugin: PluginBlockCopyright, Body: "<p>x</p>", Region: "footer"},
		"a custom block without title": {Body: "<p>x</p>", Region: "footer"},
		"a custom block without body":  {Title: "Buttons", Region: "footer"},
		"a custom block's settings":    {Title: "Buttons", Body: "<p>x</p>", Region: "footer", Settings: map[string]string{"a": "b"}},
		"a region the theme lacks":     {Plugin: PluginBlockCopyright, Region: "sidebar"},
		"the hidden region":            {Plugin: PluginBlockCopyright, Region: RegionDisabled},
	} {
		if err := (ThemeInfo{Regions: regions, Blocks: []ThemeBlock{b}}).CheckBlocks(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
