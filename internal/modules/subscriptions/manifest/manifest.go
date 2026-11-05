// Package manifest declares the subscriptions module (its registry
// entry): subscriptions to content's comments and to terms, the
// notifications of them on the site and by e-mail.
package manifest

import (
	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/ent"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "subscriptions"

// TabPath - the profile tab of the notifications: /me/<TabPath>.
const TabPath = "notifications"

// Module - the module's registry entry: its tables are its own schema
// (RegisterSteps). New to the engine after sites were installed: off on
// them until enabled, installed on a new site.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Subscriptions"),
		Description: i18n.N("Subscriptions and notifications: new comments on content, replies to one's comments, new content with a term - on the site (the Notifications tab, the user menu) and by e-mail."),
		Category:    manifestkit.CategoryCommunity,
		CSS:         []string{"subscriptions"},
		Paths:       []string{"/subscriptions", "/notifications"},
		Tables:      []string{"subscriptions", "subscription_notifications", "subscription_user_settings", "subscription_settings"},
		Data: []manifestkit.ModuleData{{Label: i18n.N("Subscriptions"), Table: "subscriptions"},
			{Label: i18n.N("Notifications"), Table: "subscription_notifications"}},
		InstallByDefault: true,
		OffByDefault:     true,
	}
}

// RegisterSteps registers the module's own ent schema: its set's tables
// and migration files (manifestkit.RegisterSchema).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
}
