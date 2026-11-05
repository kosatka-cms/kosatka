package entity

import (
	"fmt"
	"unicode/utf8"
)

// ThemeBlock - a block a theme comes with (theme.yaml "blocks"): placed
// when the theme gets its
// layout for the first time (a new site's theme on install, another
// theme when first chosen or opened on the Blocks page), after the
// placements copied from the current theme, unless that layout already
// has it. Either a plugin block (Plugin: its PluginBlockID, Settings: its
// PluginConfig) or a custom block (Title and Body, the HTML sanitized as
// a block saved in the admin); then the site edits or removes it as any
// other.
type ThemeBlock struct {
	Plugin    PluginBlockID     `yaml:"plugin"`
	Settings  map[string]string `yaml:"settings"`
	Title     string            `yaml:"title"`
	ShowTitle bool              `yaml:"show_title"`
	Body      string            `yaml:"body"`
	Region    Region            `yaml:"region"`
	Weight    int               `yaml:"weight"`
}

// IsCustom - a custom block (its Body), not a plugin one.
func (b ThemeBlock) IsCustom() bool {
	return b.Plugin == ""
}

// maxThemeBlockTitle - a custom block's title (blocks.title).
const maxThemeBlockTitle = 255

// CheckBlocks validates the theme's blocks: a plugin block or a custom
// one with a title and a body, in one of the theme's regions.
func (t ThemeInfo) CheckBlocks() error {
	for i, b := range t.Blocks {
		switch {
		case b.Plugin != "" && b.Body != "":
			return fmt.Errorf("block %d: a plugin block (%s) has no body", i+1, b.Plugin)
		case b.Plugin == "" && (b.Title == "" || b.Body == ""):
			return fmt.Errorf("block %d: a custom block needs a title and a body", i+1)
		case b.Plugin == "" && len(b.Settings) > 0:
			return fmt.Errorf("block %d: settings are a plugin block's", i+1)
		}
		if utf8.RuneCountInString(b.Title) > maxThemeBlockTitle {
			return fmt.Errorf("block %d: the title is longer than %d characters", i+1, maxThemeBlockTitle)
		}
		if b.Region == RegionDisabled || !t.Has(b.Region) {
			return fmt.Errorf("block %d: region %q isn't the theme's", i+1, b.Region)
		}
	}
	return nil
}
