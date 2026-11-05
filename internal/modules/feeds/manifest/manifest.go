// Package manifest declares the feeds module (its registry entry).
package manifest

import (
	"github.com/kosatka-cms/kosatka/internal/modules/feeds/ent"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "feeds"

// Module - the module's registry entry.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Feeds"), Description: i18n.N("Configurable XML feeds of content at their own addresses (the site RSS at /rss.xml is not affected)."),
		Category: manifestkit.CategoryIntegrations,
		Tables:   []string{"feeds", "feed_field_maps"},
		Data:     []manifestkit.ModuleData{{Label: i18n.N("Feeds"), Table: "feeds"}},
		AdminMenu: []manifestkit.ModuleMenuItem{{Section: manifestkit.AdminSectionConfiguration,
			Item: manifestkit.AdminMenuItem{Label: i18n.N("Feeds"), Path: "/admin/config/feeds", Group: manifestkit.AdminGroupModules}}},
	}
}

// RegisterSteps registers the module's own ent schema: its tables and
// migration files (manifests.Register).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
}
