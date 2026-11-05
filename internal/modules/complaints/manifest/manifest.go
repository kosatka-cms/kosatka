// Package manifest declares the complaints module (its registry entry).
package manifest

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/modules/complaints/ent"
	"github.com/kosatka-cms/kosatka/internal/modules/complaints/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "complaints"

// The module's permissions.
const (
	// PermComplain - reporting a content item or a comment.
	PermComplain = "complain about content"
	// PermHandle - the queue of complaints and the module's settings.
	PermHandle = "handle complaints"
)

// AdminQueuePath, AdminSettingsPath - the module's admin pages.
const (
	AdminQueuePath    = "/admin/content/complaints"
	AdminSettingsPath = "/admin/config/complaints"
)

// SettingsRowID - the id of complaint_settings' single row.
const SettingsRowID = 1

// Module - the module's registry entry.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Complaints"),
		Description:  i18n.N("Signed-in visitors report content and comments (spam, insults, broken rules); moderators handle them in Content → Complaints. Enough complaints hide the item until a moderator decides."),
		Category:     manifestkit.CategoryCommunity,
		OffByDefault: true,
		Permissions: []manifestkit.ModulePermission{
			{Key: PermComplain, Label: i18n.N("Report content and comments")},
			{Key: PermHandle, Label: i18n.N("Handle complaints")},
		},
		AdminMenu: []manifestkit.ModuleMenuItem{
			{Section: manifestkit.AdminSectionContent, Item: manifestkit.AdminMenuItem{Label: i18n.N("Complaints"), Path: AdminQueuePath, Permission: PermHandle}},
			{Section: manifestkit.AdminSectionConfiguration, Item: manifestkit.AdminMenuItem{Label: i18n.N("Complaints"), Path: AdminSettingsPath, Group: manifestkit.AdminGroupModules, Permission: PermHandle}},
		},
		Tables: []string{"complaints", "complaint_targets", "complaint_settings"},
		Data:   []manifestkit.ModuleData{{Label: i18n.N("Complaints"), Table: "complaints"}},
		CSS:    []string{"complaints"},
		JS:     []string{"complaints"},
	}
}

// RegisterSteps registers the module's own ent schema and its install
// step: the settings row, and signed-in users may complain.
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
	manifestkit.RegisterInstall(Name, func(ctx context.Context, env manifestkit.InstallEnv) error {
		c := generated.NewClient(generated.Driver(manifestkit.Driver(env)))
		if err := c.ComplaintSettings.Create().SetID(SettingsRowID).Exec(ctx); err != nil {
			return fmt.Errorf("the complaints' settings row: %w", err)
		}
		return manifestkit.GrantPermission(ctx, env, manifestkit.RoleUser, PermComplain)
	})
}
