// Package manifest declares the 404 log module (its registry entry): the
// paths visitors asked for and the site didn't find, with a redirect made
// from any of them in a click.
package manifest

import (
	"github.com/kosatka-cms/kosatka/internal/modules/not_found_log/ent"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "not_found_log"

// The module's pages: the log (a tab of the Redirects page) and its
// settings; RedirectsPath - the Redirects page (its menu item shows the
// count).
const (
	AdminPath     = "/admin/config/redirects/not-found"
	SettingsPath  = AdminPath + "/settings"
	RedirectsPath = "/admin/config/redirects"
)

// Module - the module's registry entry: its tables are its own schema
// (RegisterSteps). New to the engine after sites were installed: off on
// them until enabled, installed on a new site.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("404 log"),
		Description:      i18n.N("The addresses visitors asked for and the site didn't find (404), with how many times and where from; a redirect from any of them in a click (Configuration → Redirects, the \"Not found\" tab)."),
		Category:         manifestkit.CategoryAdministration,
		CSS:              []string{"not_found_log"},
		JS:               []string{"not_found_log"},
		Tables:           []string{"not_found_paths", "not_found_settings"},
		Data:             []manifestkit.ModuleData{{Label: i18n.N("Addresses not found"), Table: "not_found_paths"}},
		InstallByDefault: true,
		OffByDefault:     true,
	}
}

// RegisterSteps registers the module's own ent schema (its tables and
// migration files).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
}
