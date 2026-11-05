package modulekit

import (
	"io/fs"

	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

// Registration - a module from outside the engine (its own repository,
// imported by the site's main package): what the engine's own modules
// give it too - its description and steps, its Wire, its interface
// catalogs (translations/) and embedded files (templates/, static/).
type Registration struct {
	Module manifestkit.Module
	// Steps - the module's RegisterSteps (its schema, install, update and
	// demo steps); nil - none.
	Steps func()
	Wire  func(Deps) Parts
	// Catalog - its translations: <lang>.json files at the root of the
	// file system (the module's translations/ directory); nil - none.
	Catalog fs.FS
	// Assets - its templates/ and static/; nil - none.
	Assets fs.FS
}

// registrations - the modules from outside the engine, in the order they
// registered.
var registrations []Registration

// Register adds a module from outside the engine: called from the
// module's package init, so importing the package ("_" in the site's
// main package) is all it takes - the engine lists it after its own. A
// name the engine has already panics at startup.
func Register(r Registration) {
	manifestkit.Register(r.Module, r.Steps)
	registrations = append(registrations, r)
}

// Registered - the modules from outside the engine, in the order they
// registered (the engine lists them after its own).
func Registered() []Registration { return registrations }
