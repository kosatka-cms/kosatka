// Command migrations-gen writes the next schema migration of every set
// whose ent tables changed (internal/migrationsgen; scripts/
// migrations_gen.sh starts the databases it needs).
//
//	go run ./cmd/migrations-gen -name add_feed_language
//	go run ./cmd/migrations-gen -check
package main

import "github.com/kosatka-cms/kosatka/internal/migrationsgen"

func main() { migrationsgen.Main() }
