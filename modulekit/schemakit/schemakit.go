// Package schemakit - the engine's conventions for ent schemas, shared by
// the core's schema and a module's own (its ent/schema): the same table
// options and column types, so a module's tables look as the core's do.
package schemakit

import (
	"fmt"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// ErrEntityNotFound - what a missing record is to the engine
// (modulekit.ErrEntityNotFound): a module's generated ent package makes
// its NotFoundError match it with a hand-written Is method, as the
// core's does, so the handlers answer 404 for it.
var ErrEntityNotFound = entity.ErrEntityNotFound

// MySQLType - a column type for MySQL only; other databases (PostgreSQL)
// take ent's default for the field.
func MySQLType(t string) map[string]string {
	return map[string]string{dialect.MySQL: t}
}

// FixedChar - CHAR(n) on every database.
func FixedChar(n int) map[string]string {
	t := fmt.Sprintf("char(%d)", n)
	return map[string]string{dialect.MySQL: t, dialect.Postgres: t}
}

// NotIncremental - an id set by the code (a settings table's single row),
// not by auto-increment.
func NotIncremental() *entsql.Annotation {
	no := false
	return &entsql.Annotation{Incremental: &no}
}

// Table - a table's annotation: its name, and on MySQL utf8mb4 with
// case-insensitive comparison (utf8mb4_unicode_ci, as the site databases
// have it) instead of ent's default utf8mb4_bin, under which equality and
// LIKE would tell the case apart. PostgreSQL ignores both.
func Table(name string) entsql.Annotation {
	return entsql.Annotation{Table: name, Charset: "utf8mb4", Collation: "utf8mb4_unicode_ci"}
}
