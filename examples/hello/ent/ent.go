// Package ent - the hello module's own ent schema (schema/), its generated
// client (generated/, go generate) and migration files
// (migrations/<dialect>/, written by cmd/migrations).
package ent

import (
	"embed"
	"io/fs"

	"example.com/kosatka-hello/ent/generated/migrate"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./schema --target generated

//go:embed migrations
var files embed.FS

// Schema - the module's tables and migration files (Dir: where the files
// are, from the module's root - cmd/migrations runs there).
var Schema = manifestkit.Schema{Tables: migrate.Tables, Files: migrationFiles(), Dir: "ent/migrations"}

// migrationFiles - the embedded migrations/ directory as the root.
func migrationFiles() fs.FS {
	sub, err := fs.Sub(files, "migrations")
	if err != nil {
		panic(err) // a constant, valid path
	}
	return sub
}
