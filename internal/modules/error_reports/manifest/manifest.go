// Package manifest declares the error reports module (its registry
// entry).
package manifest

import (
	"github.com/kosatka-cms/kosatka/internal/modules/error_reports/ent"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name (the name of the feature switch it
// was, so a saved state carries over).
const Name = "error_reports"

// PluginBlockHint - how to report an error in the text.
const PluginBlockHint manifestkit.PluginBlockID = "error_report_hint"

// CaptchaForm - the report form (on every page).
const CaptchaForm = "error_report"

// Module - the module's registry entry.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Error reports"), Description: i18n.N("Visitors report errors in the text: they select it and press Ctrl+Shift+E. Reports go to Content → Error reports, the new ones counted in the admin menu; the error report hint block."),
		Category:     manifestkit.CategoryAdministration,
		PluginBlocks: []manifestkit.ModulePluginBlock{{ID: PluginBlockHint, Label: i18n.N("Error report hint")}},
		JS:           []string{"error_reports"},
		Tables:       []string{"error_reports"},
		Data:         []manifestkit.ModuleData{{Label: i18n.N("Error reports"), Table: "error_reports"}},
		// Under Content: the editors' queue, like moderation (the number of
		// new ones by it - AddAdminMenuCount).
		AdminMenu: []manifestkit.ModuleMenuItem{{Section: manifestkit.AdminSectionContent,
			Item: manifestkit.AdminMenuItem{Label: i18n.N("Error reports"), Path: "/admin/error-reports"}}},
		CaptchaForms: []manifestkit.ModuleCaptchaForm{{Key: CaptchaForm, Label: i18n.N("Error report (Ctrl+Shift+E)"), Default: true}},
	}
}

// RegisterSteps registers the module's own ent schema: its set's tables
// and migration files (manifestkit.RegisterSchema).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
}
