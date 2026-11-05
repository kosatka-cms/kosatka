package entity

import (
	"strings"
	"testing"
)

func TestThemeSettingValid(t *testing.T) {
	color := ThemeSetting{Type: ThemeSettingColor}
	choice := ThemeSetting{Type: ThemeSettingSelect, Options: []ThemeSettingOption{{Value: "light"}, {Value: "dark"}}}
	flag := ThemeSetting{Type: ThemeSettingBool}
	line := ThemeSetting{Type: ThemeSettingText, MaxLength: 5}
	for _, tc := range []struct {
		setting ThemeSetting
		value   string
		want    bool
	}{
		{color, "#0e7490", true},
		{color, "#0E7490", true},
		{color, "#fff", false},
		{color, "red", false},
		{color, "#0e7490; background: url(x)", false},
		{choice, "dark", true},
		{choice, "sepia", false},
		{flag, "1", true},
		{flag, "0", true},
		{flag, "true", false},
		{line, "", true},
		{line, "Пятёр", true},
		{line, "Пятёрк", false},
		{line, "a\nb", false},
	} {
		if got := tc.setting.Valid(tc.value); got != tc.want {
			t.Errorf("%s %q: %v, want %v", tc.setting.Type, tc.value, got, tc.want)
		}
	}
}

// CheckSettings rejects a manifest's broken settings and normalizes a
// bool's default and a text's limit.
func TestThemeInfoCheckSettings(t *testing.T) {
	ok := ThemeInfo{Settings: []ThemeSetting{
		{ID: "accent", Type: ThemeSettingColor, Default: "#0e7490", CSSVar: "--accent", Selector: `:root[data-bs-theme="dark"]`},
		{ID: "marquee", Type: ThemeSettingBool, Default: "true"},
		{ID: "tagline", Type: ThemeSettingText},
	}}
	if err := ok.CheckSettings(); err != nil {
		t.Fatalf("a valid manifest: %v", err)
	}
	if ok.Settings[2].MaxLength != defaultThemeTextLength {
		t.Errorf("a text's limit unset: %d, want %d", ok.Settings[2].MaxLength, defaultThemeTextLength)
	}
	if ok.Settings[1].Default != "1" {
		t.Errorf("a bool's default true: %q, want 1", ok.Settings[1].Default)
	}
	for name, s := range map[string]ThemeSetting{
		"an id with a dash":       {ID: "accent-colour", Type: ThemeSettingColor, Default: "#000000"},
		"an unknown type":         {ID: "font", Type: "font", Default: "Arial"},
		"a select, no options":    {ID: "scheme", Type: ThemeSettingSelect, Default: "light"},
		"a bool's css_var":        {ID: "marquee", Type: ThemeSettingBool, CSSVar: "--marquee"},
		"a text's css_var":        {ID: "tagline", Type: ThemeSettingText, CSSVar: "--tagline"},
		"a text default too long": {ID: "tagline", Type: ThemeSettingText, MaxLength: 3, Default: "four"},
		"a css_var without --":    {ID: "accent", Type: ThemeSettingColor, Default: "#000000", CSSVar: "accent"},
		"a selector closing { }":  {ID: "accent", Type: ThemeSettingColor, Default: "#000000", CSSVar: "--a", Selector: "a{}b"},
		"an invalid default":      {ID: "accent", Type: ThemeSettingColor, Default: "teal"},
	} {
		info := ThemeInfo{Settings: []ThemeSetting{s}}
		if err := info.CheckSettings(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	twice := ThemeInfo{Settings: []ThemeSetting{
		{ID: "accent", Type: ThemeSettingColor, Default: "#000000"},
		{ID: "accent", Type: ThemeSettingColor, Default: "#ffffff"},
	}}
	if err := twice.CheckSettings(); err == nil || !strings.Contains(err.Error(), "second") {
		t.Errorf("a repeated id: %v", err)
	}
}
