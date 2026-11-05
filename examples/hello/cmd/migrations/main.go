// Command migrations writes the hello module's next migration
// (kosatka.GenerateMigrations), run from the module's root with the two
// test databases running:
//
//	KOSATKA_TEST_MYSQL_DSN=… KOSATKA_TEST_POSTGRES_DSN=… go run ./cmd/migrations -name <name>
//	… go run ./cmd/migrations -check
package main

import (
	"github.com/kosatka-cms/kosatka"

	_ "example.com/kosatka-hello"
)

func main() { kosatka.GenerateMigrations() }
