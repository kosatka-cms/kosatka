// Package manifest declares the ratings module (its registry entry).
package manifest

import (
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "ratings"

// PermRate - rating content, for signed-in users (guests - the module's
// setting).
const PermRate = "rate content"

// PluginBlockNodeRating - the rating of the content page the block is on:
// the average, the count, the visitor's own vote.
const PluginBlockNodeRating manifestkit.PluginBlockID = "node_rating"

// SettingsPath - the module's settings page.
const SettingsPath = "/admin/config/ratings"

// The module's Views criteria its demo builds a list with (views.go
// registers them): the rating field and its "look" (the average: stars
// and the number), the weighted rating's sort and its "min_votes", the
// number of votes' filter.
const (
	ViewFieldRating   = Name + ".rating"
	ViewSortWeighted  = Name + ".weighted"
	ViewFilterCount   = Name + ".count"
	ViewParamLook     = "look"
	ViewLookAverage   = "average"
	ViewParamMinVotes = "min_votes"
)

// Module - the module's registry entry.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Ratings"), Description: i18n.N("Star ratings of content: votes on content pages, the rating in Views - its sorts (the weighted rating for a top list), filters and field."),
		Category:    manifestkit.CategoryCommunity,
		Permissions: []manifestkit.ModulePermission{{Key: PermRate, Label: i18n.N("Rate content")}},
		// Installing the module places the block inside the content page.
		PluginBlocks: []manifestkit.ModulePluginBlock{{ID: PluginBlockNodeRating, Label: i18n.N("The content's rating"), Region: "content_inline"}},
		JS:           []string{"ratings"},
		CSS:          []string{"ratings"},
		Tables:       []string{"rating_settings", "rating_content_types", "rating_votes"},
		Data:         []manifestkit.ModuleData{{Label: i18n.N("Star votes"), Table: "rating_votes"}},
		AdminMenu: []manifestkit.ModuleMenuItem{{Section: manifestkit.AdminSectionConfiguration,
			Item: manifestkit.AdminMenuItem{Label: i18n.N("Ratings"), Path: SettingsPath, Group: manifestkit.AdminGroupModules}}},
		OffByDefault: true,
	}
}
