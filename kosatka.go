// Package kosatka - the engine as a library, for a site's own binary: its
// main package imports the modules it has from outside the engine (each
// registers itself, modulekit.Register) and calls Main.
//
//	package main
//
//	import (
//		"github.com/kosatka-cms/kosatka"
//
//		_ "example.com/kosatka-hello"
//	)
//
//	func main() { kosatka.Main() }
package kosatka

import (
	"github.com/kosatka-cms/kosatka/internal/app"
	"github.com/kosatka-cms/kosatka/internal/migrationsgen"
)

// Main runs the engine: the server, or the command its arguments name
// ("app help" lists them).
func Main() { app.Main() }

// GenerateMigrations runs the migrations generator (-name <name> writes
// the next migration of each changed set, -check fails when the files are
// behind the ent schemas) for the modules the binary imports: a module's
// own repository calls it from a main package importing the module, run
// in its tree - its migrations go into its own schema's directory (the
// databases: KOSATKA_TEST_MYSQL_DSN, KOSATKA_TEST_POSTGRES_DSN).
func GenerateMigrations() { migrationsgen.Main() }
