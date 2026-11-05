package schema

import (
	"entgo.io/ent/dialect/entsql"

	"github.com/kosatka-cms/kosatka/modulekit/schemakit"
)

// The engine's conventions for ent schemas (schemakit, shared with the
// modules' own schemas), by their names in this package.

// mysqlType - schemakit.MySQLType.
func mysqlType(t string) map[string]string { return schemakit.MySQLType(t) }

// fixedChar - schemakit.FixedChar.
func fixedChar(n int) map[string]string { return schemakit.FixedChar(n) }

// notIncremental - schemakit.NotIncremental.
func notIncremental() *entsql.Annotation { return schemakit.NotIncremental() }

// table - schemakit.Table.
func table(name string) entsql.Annotation { return schemakit.Table(name) }
