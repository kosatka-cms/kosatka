// Package sqldialect keeps hand-written SQL portable between MySQL and
// PostgreSQL: the few constructs that differ (placeholders, upsert, the id
// of an inserted row, random order, today's date) go through these helpers
// instead of being written for one database. New code uses them; the
// MySQL-only SQL that predates them is listed in portability_test.go and
// migrated over time (docs/contributing/database.md).
package sqldialect

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Dialect - the database a query is written for.
type Dialect string

// The dialects, by the names ent and config.DBConfig.Dialect give them.
const (
	MySQL    Dialect = "mysql"
	Postgres Dialect = "postgres"
)

// Rebind turns the ? placeholders of query into the dialect's ($1, $2, …
// for PostgreSQL). A ? inside a quoted string or identifier is left alone.
func (d Dialect) Rebind(query string) string {
	if d != Postgres || !strings.Contains(query, "?") {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 0
	var quote byte
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '?':
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// Upsert - an INSERT of cols into table that updates update (a subset of
// cols) when a row with the same unique key already exists; conflict
// lists that key's columns (PostgreSQL needs them, MySQL uses any unique
// key). Placeholders are ?; run the result through Rebind.
func (d Dialect) Upsert(table string, cols, conflict, update []string) string {
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	q := "INSERT INTO " + table + " (" + strings.Join(cols, ", ") + ") VALUES (" + ph + ")"
	sets := make([]string, len(update))
	switch d {
	case Postgres:
		for i, c := range update {
			sets[i] = c + " = EXCLUDED." + c
		}
		if len(sets) == 0 {
			return q + " ON CONFLICT (" + strings.Join(conflict, ", ") + ") DO NOTHING"
		}
		return q + " ON CONFLICT (" + strings.Join(conflict, ", ") + ") DO UPDATE SET " + strings.Join(sets, ", ")
	default:
		for i, c := range update {
			sets[i] = c + " = new." + c
		}
		if len(sets) == 0 {
			// A no-op update keeps the row as it is (INSERT IGNORE would
			// also hide other errors).
			sets = []string{conflict[0] + " = new." + conflict[0]}
		}
		return q + " AS new ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", ")
	}
}

// Execer is what InsertID needs (*sql.DB, *sql.Tx, *sql.Conn).
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// InsertID runs an INSERT (? placeholders) and returns the new row's id
// (column id): LastInsertId on MySQL, RETURNING id on PostgreSQL.
func (d Dialect) InsertID(ctx context.Context, db Execer, query string, args ...any) (int64, error) {
	if d == Postgres {
		var id int64
		err := db.QueryRowContext(ctx, d.Rebind(query)+" RETURNING id", args...).Scan(&id)
		return id, err
	}
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Random - the expression for random order (ORDER BY …).
func (d Dialect) Random() string {
	if d == Postgres {
		return "RANDOM()"
	}
	return "RAND()"
}

// CurrentDate - today's date in the database's time zone.
func (d Dialect) CurrentDate() string {
	return "CURRENT_DATE"
}
