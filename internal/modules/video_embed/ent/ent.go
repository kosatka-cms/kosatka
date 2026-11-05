// Package ent - the video embed module's own ent schema (schema/), its
// generated client (generated/, go generate) and migration files
// (migrations/<dialect>/, scripts/migrations_gen.sh writes them), apart
// from the core's: the module's set of migrations (Schema).
package ent

import (
	"embed"
	"io/fs"

	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/ent/generated/migrate"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./schema --target generated

//go:embed migrations
var files embed.FS

// Schema - the module's tables and migration files, for its set
// (manifestkit.RegisterSchema). Dir - where the files are in the source
// tree, from the project's root.
var Schema = manifestkit.Schema{Tables: migrate.Tables, Files: migrationFiles(), Dir: "internal/modules/video_embed/ent/migrations"}

// migrationFiles - the embedded migrations/ directory as the root.
func migrationFiles() fs.FS {
	sub, err := fs.Sub(files, "migrations")
	if err != nil {
		panic(err) // a constant, valid path
	}
	return sub
}
