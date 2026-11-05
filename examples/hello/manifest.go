package hello

import (
	"example.com/kosatka-hello/ent"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "hello"

// PluginBlockLatest - the latest greetings, a block placed on the Blocks
// page (templates/blocks/hello_latest.html).
const PluginBlockLatest manifestkit.PluginBlockID = "hello_latest"

// Module - the module's registry entry.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name:         Name,
		Label:        i18n.N("Hello"),
		Description:  i18n.N("An example module from outside the engine: the greetings page (/hello)."),
		Category:     manifestkit.CategoryContent,
		Tables:       []string{"hello_greetings"},
		Data:         []manifestkit.ModuleData{{Label: i18n.N("Greetings"), Table: "hello_greetings"}},
		CSS:          []string{"hello"},
		PluginBlocks: []manifestkit.ModulePluginBlock{{ID: PluginBlockLatest, Label: i18n.N("Latest greetings")}},
	}
}

// RegisterSteps registers the module's own ent schema: its tables and
// migration files.
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
}
