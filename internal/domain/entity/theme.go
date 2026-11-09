package entity

import "github.com/kosatka-cms/kosatka/pkg/i18n"

// DefaultTheme - the placement theme key of the engine's own templates (no
// theme directory). Every other theme's placements are keyed by its
// directory under themes/.
const DefaultTheme = "default"

// ThemeKey - the placement theme key of a theme directory ("" - the
// engine's templates).
func ThemeKey(dir string) string {
	if dir == "" {
		return DefaultTheme
	}
	return dir
}

// ThemeDir - the theme directory of a placement theme key.
func ThemeDir(key string) string {
	if key == DefaultTheme {
		return ""
	}
	return key
}

// Region - a region machine name. Regions are declared by the theme in
// theme.yaml (ThemeInfo, themes.LoadInfo) and rendered by theme templates; Go has no list of them.
type Region string

// RegionDisabled - the built-in "Hidden" region: the placement is kept but
// rendered nowhere. Exists in every theme and is not declared in theme.yaml.
const RegionDisabled Region = "disabled"

// ThemeRegion - a theme region: machine name and admin label.
type ThemeRegion struct {
	ID    Region `yaml:"id"`
	Label string `yaml:"label"`
}

// ThemeInfo - the active theme's description (theme.yaml): regions in admin
// order, the default region for a new block, the theme's settings
// (ThemeSetting, CheckSettings), a subtheme's base theme (its templates,
// regions and settings are inherited, paths.ThemeChain) and the
// stylesheets the theme adds after its base's (css/<name>.css).
type ThemeInfo struct {
	Name          string         `yaml:"name"`
	Base          string         `yaml:"base"`
	DefaultRegion Region         `yaml:"default_region"`
	Regions       []ThemeRegion  `yaml:"regions"`
	Settings      []ThemeSetting `yaml:"settings"`
	Stylesheets   []string       `yaml:"stylesheets"`
	// Favicon - the theme's icon, a path on the site ("/static/…"): the
	// site's when Site information has none (service.SiteIconFor); a
	// subtheme without one takes its base's.
	Favicon string `yaml:"favicon"`
	// Logo - the theme's logo, a path on the site ("/static/…"): the
	// site's when Site information has none (service.SiteLogoFor); a
	// subtheme without one takes its base's.
	Logo string `yaml:"logo"`
	// Blocks - the blocks the theme comes with (ThemeBlock, CheckBlocks);
	// a subtheme without its own takes its base's.
	Blocks []ThemeBlock `yaml:"blocks"`
	// The theme's description for the Appearance page: Description - English,
	// translated by the theme's catalog as Name; Version - its release
	// ("" - the engine's own, released with it); Author, Homepage (its
	// page or repository, http(s)), License (an SPDX identifier, e.g.
	// "MIT"); Tags - lowercase words (dark, two-columns, retro…), also
	// translated; Screenshot - a file in the theme's static directory
	// (themes/<theme>/static/, 3:2, e.g. 1500×1000); MinVersion - the
	// engine release it needs at least ("0.2.0"), the theme can't be
	// previewed or set active on an older one.
	Description string   `yaml:"description"`
	Version     string   `yaml:"version"`
	Author      string   `yaml:"author"`
	Homepage    string   `yaml:"homepage"`
	License     string   `yaml:"license"`
	Tags        []string `yaml:"tags"`
	Screenshot  string   `yaml:"screenshot"`
	MinVersion  string   `yaml:"min_version"`
}

// RegionIDs returns the theme's regions in order.
func (t ThemeInfo) RegionIDs() []Region {
	out := make([]Region, len(t.Regions))
	for i, r := range t.Regions {
		out[i] = r.ID
	}
	return out
}

// RegionIDsWithDisabled returns the theme's regions followed by RegionDisabled.
func (t ThemeInfo) RegionIDsWithDisabled() []Region {
	return append(t.RegionIDs(), RegionDisabled)
}

// Has reports whether the region belongs to the theme (RegionDisabled does not).
func (t ThemeInfo) Has(r Region) bool {
	for _, tr := range t.Regions {
		if tr.ID == r {
			return true
		}
	}
	return false
}

// Allows reports whether a placement may use the region: a theme region or RegionDisabled.
func (t ThemeInfo) Allows(r Region) bool {
	return r == RegionDisabled || t.Has(r)
}

// Labels returns labels for the theme's regions and RegionDisabled.
func (t ThemeInfo) Labels() map[Region]string {
	out := make(map[Region]string, len(t.Regions)+1)
	for _, r := range t.Regions {
		out[r.ID] = r.Label
	}
	out[RegionDisabled] = i18n.N("Hidden")
	return out
}
