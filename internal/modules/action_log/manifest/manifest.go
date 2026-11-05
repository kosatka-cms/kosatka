// Package manifest declares the action log module (its registry entry):
// who did what on the site - content deleted, a role's permissions
// changed, settings saved, sign-ins.
package manifest

import (
	"github.com/kosatka-cms/kosatka/internal/modules/action_log/ent"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "action_log"

// AdminPath - the log; its settings are under it (SettingsPath).
const (
	AdminPath    = "/admin/reports/action-log"
	SettingsPath = AdminPath + "/settings"
)

// PermView - seeing the log and changing its settings (who did what,
// from which address: not every administrator's business).
const PermView = "view action log"

// Module - the module's registry entry: its tables are its own schema
// (RegisterSteps). New to the engine after sites were installed: off on
// them until enabled, installed on a new site.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Action log"),
		Description: i18n.N("Who did what on the site: content deleted or unpublished, users blocked, roles' permissions and settings changed, sign-ins. Other modules add their own actions."),
		Category:    manifestkit.CategoryAdministration,
		CSS:         []string{"action_log"},
		Tables:      []string{"action_log", "action_log_settings"},
		Data:        []manifestkit.ModuleData{{Label: i18n.N("Action log entries"), Table: "action_log"}},
		Permissions: []manifestkit.ModulePermission{{Key: PermView, Label: i18n.N("View the action log")}},
		AdminMenu: []manifestkit.ModuleMenuItem{{Section: manifestkit.AdminSectionReports,
			Item: manifestkit.AdminMenuItem{Label: i18n.N("Action log"), Path: AdminPath, Permission: PermView}}},
		InstallByDefault: true,
		OffByDefault:     true,
	}
}

// RegisterSteps registers the module's own ent schema (its tables and
// migration files).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
}
