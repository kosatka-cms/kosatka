package entity

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ThemeSettingType - the kind of a theme setting's value.
type ThemeSettingType string

const (
	// ThemeSettingColor - a colour, "#rrggbb"; with CSSVar it becomes that
	// CSS variable of the theme's pages.
	ThemeSettingColor ThemeSettingType = "color"
	// ThemeSettingSelect - one of the setting's Options (their Value).
	ThemeSettingSelect ThemeSettingType = "select"
	// ThemeSettingBool - on ("1") or off ("0").
	ThemeSettingBool ThemeSettingType = "bool"
	// ThemeSettingText - a line of text of at most MaxLength characters
	// (empty allowed), no line breaks or control characters; for the
	// theme's templates (themeSetting), never a CSS variable.
	ThemeSettingText ThemeSettingType = "text"
)

// ThemeSetting - an option a theme offers in its manifest (theme.yaml
// "settings"), set on the Appearance
// page: ID - its name in the templates (themeSetting "<id>"), Label -
// English, translated like the region labels, Default - its value until
// set. A colour's CSSVar ("--name") is set to it on the theme's pages, in
// Selector (":root" when empty; e.g. `[data-bs-theme="dark"]` for a dark
// scheme's colour).
type ThemeSetting struct {
	ID       string               `yaml:"id"`
	Label    string               `yaml:"label"`
	Type     ThemeSettingType     `yaml:"type"`
	Default  string               `yaml:"default"`
	Options  []ThemeSettingOption `yaml:"options"`
	CSSVar   string               `yaml:"css_var"`
	Selector string               `yaml:"selector"`
	// MaxLength - a text's limit in characters (defaultThemeTextLength
	// when unset).
	MaxLength int `yaml:"max_length"`
}

// defaultThemeTextLength - a text setting's limit when the manifest
// gives none.
const defaultThemeTextLength = 200

// ThemeSettingOption - a select setting's choice: its value and English
// label.
type ThemeSettingOption struct {
	Value string `yaml:"value"`
	Label string `yaml:"label"`
}

var (
	themeSettingIDRe    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	themeSettingColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	themeSettingVarRe   = regexp.MustCompile(`^--[a-zA-Z0-9-]+$`)
)

// Valid reports whether v is a value of the setting: a "#rrggbb" colour,
// one of the options, "1" or "0", a line of text within MaxLength.
func (s ThemeSetting) Valid(v string) bool {
	switch s.Type {
	case ThemeSettingColor:
		return themeSettingColorRe.MatchString(v)
	case ThemeSettingSelect:
		for _, o := range s.Options {
			if o.Value == v {
				return true
			}
		}
	case ThemeSettingBool:
		return v == "1" || v == "0"
	case ThemeSettingText:
		if utf8.RuneCountInString(v) > s.MaxLength {
			return false
		}
		for _, r := range v {
			if unicode.IsControl(r) {
				return false
			}
		}
		return true
	}
	return false
}

// CSSSelector - where the setting's CSS variable is set.
func (s ThemeSetting) CSSSelector() string {
	if s.Selector == "" {
		return ":root"
	}
	return s.Selector
}

// normalize - a bool's default as "1"/"0" (the manifest may say true or
// false), a text's limit.
func (s *ThemeSetting) normalize() {
	if s.Type == ThemeSettingText && s.MaxLength <= 0 {
		s.MaxLength = defaultThemeTextLength
	}
	if s.Type != ThemeSettingBool {
		return
	}
	switch s.Default {
	case "1", "true", "yes", "on":
		s.Default = "1"
	default:
		s.Default = "0"
	}
}

// CheckSettings validates and normalizes the theme's settings: an ID
// (latin, digits, "_"; unique), a known type, a valid default, options of
// a select, a CSS variable only of a colour.
func (t *ThemeInfo) CheckSettings() error {
	seen := map[string]bool{}
	for i := range t.Settings {
		s := &t.Settings[i]
		s.normalize()
		switch {
		case !themeSettingIDRe.MatchString(s.ID):
			return fmt.Errorf("setting %q: the id must be latin letters, digits and _", s.ID)
		case seen[s.ID]:
			return fmt.Errorf("setting %q: a second setting with this id", s.ID)
		case s.Type != ThemeSettingColor && s.Type != ThemeSettingSelect && s.Type != ThemeSettingBool && s.Type != ThemeSettingText:
			return fmt.Errorf("setting %q: unknown type %q", s.ID, s.Type)
		case s.Type == ThemeSettingSelect && len(s.Options) == 0:
			return fmt.Errorf("setting %q: a select without options", s.ID)
		case s.CSSVar != "" && (s.Type != ThemeSettingColor || !themeSettingVarRe.MatchString(s.CSSVar)):
			return fmt.Errorf("setting %q: css_var is a colour's, \"--name\"", s.ID)
		case strings.ContainsAny(s.Selector, "<>{};"):
			return fmt.Errorf("setting %q: the selector can't hold < > { } ;", s.ID)
		case !s.Valid(s.Default):
			return fmt.Errorf("setting %q: invalid default %q", s.ID, s.Default)
		}
		seen[s.ID] = true
	}
	return nil
}

// Setting - the theme's setting with id, if any.
func (t ThemeInfo) Setting(id string) (ThemeSetting, bool) {
	for _, s := range t.Settings {
		if s.ID == id {
			return s, true
		}
	}
	return ThemeSetting{}, false
}

// ThemeSettingsRepository - the themes' stored setting values (theme
// key, entity.ThemeKey → setting ID → value).
type ThemeSettingsRepository interface {
	All(ctx context.Context) (map[string]map[string]string, error)
	Save(ctx context.Context, theme string, values map[string]string) error
}
