// Package manifest declares the private messages module (its registry
// entry): conversations between users, the messages tab of the profile.
package manifest

import (
	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/ent"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name. It is the key the feature switch of
// the private messages had in the modules table, so a site keeps its
// state.
const Name = "private_messages"

// TabPath - the profile tab of the conversations: /me/<TabPath>,
// /<profile>/<TabPath>; the module's pages live under /me/<TabPath>/.
const TabPath = "messages"

// Module - the module's registry entry: its tables (pm_threads,
// pm_thread_members, pm_messages) are its own schema (RegisterSteps).
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Private messages"),
		Description:      i18n.N("Conversations between users: the Messages tab of the profile, the new message notification."),
		Category:         manifestkit.CategoryCommunity,
		CSS:              []string{"private_messages"},
		JS:               []string{"private_messages"},
		Paths:            []string{"/me/messages", "/user/messages"},
		Tables:           []string{"pm_threads", "pm_thread_members", "pm_messages", "pm_settings", "pm_user_settings"},
		Data:             []manifestkit.ModuleData{{Label: i18n.N("Conversations"), Table: "pm_threads"}, {Label: i18n.N("Messages"), Table: "pm_messages"}},
		InstallByDefault: true,
		// A message's text can link to its author's files; private: no
		// title or link on the file's page of uses.
		FileSources: []manifestkit.ModuleFileSource{{Source: "private_message", Table: "pm_messages", IDColumn: "id",
			Columns: []string{"body"}, Label: i18n.N("A private message")}},
	}
}

// RegisterSteps registers the module's own ent schema: its set's tables
// and migration files (manifestkit.RegisterSchema).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
}
