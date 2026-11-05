// Package manifest declares the forms module (its registry entry): a
// "form" content type whose items have their own
// elements and collect submissions.
package manifest

import (
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "forms"

// ContentType - the form node type.
const ContentType manifestkit.ContentType = "form"

// Permissions: see and export a form's submissions; delete them (spam,
// test entries).
const (
	PermViewSubmissions   = "view form submissions"
	PermDeleteSubmissions = "delete form submissions"
)

// PluginBlockWebform - a form, chosen on the placement page (its
// PluginConfig "target": the form's node ID).
const PluginBlockWebform manifestkit.PluginBlockID = "webform"

// CaptchaForm - the forms' submissions.
const CaptchaForm = "webform"

// Module - the module's registry entry.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Forms"), Description: i18n.N("Forms with their own questions (feedback, applications, questionnaires): the form content type, its submissions."),
		Category:  manifestkit.CategoryContent,
		NodeTypes: []manifestkit.ModuleNodeType{{Name: string(ContentType), Label: i18n.N("Form"), CreatePath: "/create/form"}},
		Permissions: []manifestkit.ModulePermission{
			{Key: PermViewSubmissions, Label: i18n.N("View and export form submissions")},
			{Key: PermDeleteSubmissions, Label: i18n.N("Delete form submissions")},
		},
		PluginBlocks: []manifestkit.ModulePluginBlock{{ID: PluginBlockWebform, Label: i18n.N("Form")}},
		JS:           []string{"forms"}, CSS: []string{"forms"},
		RobotsDisallow:   []string{"/node/*/form/submissions", "/node/*/form/analysis", "/node/*/form/file"},
		Tables:           []string{"webforms", "webform_elements", "webform_submissions", "webform_submission_values"},
		Data:             []manifestkit.ModuleData{{Label: i18n.N("Submissions"), Table: "webform_submissions"}},
		OffByDefault:     true,
		InstallByDefault: true,
		AdminMenu: []manifestkit.ModuleMenuItem{{Section: manifestkit.AdminSectionContent,
			Item: manifestkit.AdminMenuItem{Label: i18n.N("Forms"), Path: "/admin/forms", Permission: PermViewSubmissions}}},
		CaptchaForms: []manifestkit.ModuleCaptchaForm{{Key: CaptchaForm, Label: i18n.N("Forms (the forms module)")}},
	}
}
