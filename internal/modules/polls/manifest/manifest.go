// Package manifest declares the polls module (its registry entry).
package manifest

import (
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "polls"

// ContentType - the poll node type.
const ContentType manifestkit.ContentType = "poll"

// PermViewVotes - see who voted for what.
const PermViewVotes = "view poll votes"

// PluginBlockPoll - a poll, chosen on the placement page (its PluginConfig
// "target": the poll's node ID), to vote in where the block stands.
const PluginBlockPoll manifestkit.PluginBlockID = "poll"

// FileSourcePoll - a poll's description in the files manager's "used in".
const FileSourcePoll = "poll"

// Module - the module's registry entry.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Polls"), Description: i18n.N("The poll content type with voting."),
		Category:     manifestkit.CategoryContent,
		NodeTypes:    []manifestkit.ModuleNodeType{{Name: string(ContentType), Label: i18n.N("Poll"), CreatePath: "/create/poll"}},
		Permissions:  []manifestkit.ModulePermission{{Key: PermViewVotes, Label: i18n.N("View poll votes")}},
		PluginBlocks: []manifestkit.ModulePluginBlock{{ID: PluginBlockPoll, Label: i18n.N("Poll")}},
		JS:           []string{"polls"},
		CSS:          []string{"polls"},
		Tables:       []string{"polls", "poll_options", "poll_votes"},
		Data:         []manifestkit.ModuleData{{Label: i18n.N("Votes"), Table: "poll_votes"}},
		FileSources: []manifestkit.ModuleFileSource{{Source: FileSourcePoll, Table: "polls", IDColumn: "nid", Columns: []string{"description"},
			Label: i18n.N("Poll \"@title\""), NodeBacked: true}},
	}
}
