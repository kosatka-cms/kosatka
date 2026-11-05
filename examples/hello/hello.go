// Package hello - an example of a module from outside the engine: its own
// Go module, built on the module SDK only (modulekit, pkg/*). It has a
// page (/hello) listing the greetings of its own table, a stylesheet and
// translations; importing the package registers it (init), and a site
// that imports it can enable it on its Modules page.
package hello

import (
	"embed"
	"io/fs"

	"github.com/kosatka-cms/kosatka/modulekit"
)

//go:embed templates static
var assets embed.FS

//go:embed translations/*.json
var translations embed.FS

func init() {
	catalog, err := fs.Sub(translations, "translations")
	if err != nil {
		panic(err) // a constant, valid path
	}
	modulekit.Register(modulekit.Registration{
		Module:  Module(),
		Steps:   RegisterSteps,
		Wire:    Wire,
		Catalog: catalog,
		Assets:  assets,
	})
}
