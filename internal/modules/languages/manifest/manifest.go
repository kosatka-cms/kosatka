// Package manifest declares the languages module (its registry entry):
// the site's languages besides its default one, each at its own address
// prefix (/en/…), the language switcher and the pages' addresses in the
// other languages for search engines.
package manifest

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/modules/languages/ent"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "languages"

// AdminPath - the Languages page.
const AdminPath = "/admin/config/languages"

// PluginBlockSwitcher - the language switcher block.
const PluginBlockSwitcher = "language_switcher"

// Module - the module's registry entry: its data is the site's list of
// languages (CoreData, cleared by its uninstall step); installing it puts
// the switcher in the theme's header. New to the engine after sites were
// installed: off until enabled.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Languages"),
		Description:  i18n.N("The site in several languages: each language besides the default one at its own address prefix (/en/…), with the interface in it, a language switcher block and the pages' addresses in the other languages for search engines."),
		Category:     manifestkit.CategoryContent,
		CoreData:     []string{i18n.N("The site's languages")},
		Tables:       []string{"node_translations", "language_settings", "text_translations", "language_visibility"},
		PluginBlocks: []manifestkit.ModulePluginBlock{{ID: PluginBlockSwitcher, Label: i18n.N("Language switcher"), Region: "header"}},
		AdminMenu: []manifestkit.ModuleMenuItem{{Section: manifestkit.AdminSectionConfiguration,
			Item: manifestkit.AdminMenuItem{Label: i18n.N("Languages"), Path: AdminPath, Group: manifestkit.AdminGroupSite}}},
		OffByDefault: true,
	}
}

// RegisterSteps registers the module's own ent schema (its tables and
// migration files) and its install step: nothing to write, it records
// the module installed where its first version (without tables) was.
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
	manifestkit.RegisterInstall(Name, func(context.Context, manifestkit.InstallEnv) error { return nil })
}
