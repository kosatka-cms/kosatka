package database

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	// The schema's defaults, hooks and interceptors (the blocks fields'
	// items left out of node queries) - with every core client.
	_ "github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/runtime"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
)

// entDialect - the database of the ent clients built for repositories
// given a *sql.DB (SetDialect; MySQL until the app runs on another).
var entDialect = dialect.MySQL

// SetDialect sets the database those repositories' ent clients speak
// (dialect.MySQL, dialect.Postgres).
func SetDialect(d string) { entDialect = d }

// clientFor - an ent client over db: repositories built from a *sql.DB
// query through ent, so their SQL fits the database. Inside a unit of
// work GetClient returns the transaction's client instead.
func clientFor(db *sql.DB) *generated.Client {
	return generated.NewClient(generated.Driver(entsql.OpenDB(entDialect, db)))
}

// settingsRowID - the id of a settings table's single row.
const settingsRowID = 1

// inTx runs fn with a transaction's client: the unit of work's when ctx
// carries one, otherwise a new transaction committed when fn succeeds.
func inTx(ctx context.Context, client *generated.Client, fn func(c *generated.Client) error) error {
	if c, ok := ctx.Value(txKey{}).(*generated.Client); ok {
		return fn(c)
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("inTx: %w", err)
	}
	if err := fn(tx.Client()); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// nilIfEmpty - s for a nullable column, nil (NULL) when it is empty.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// orderRandom - ent order option shuffling the rows (the random function
// of the query's database).
func orderRandom(s *entsql.Selector) {
	s.OrderExpr(entsql.Expr(sqldialect.Dialect(s.Dialect()).Random()))
}

// rebind - a hand-written query (? placeholders) in the placeholders of
// the repositories' database.
func rebind(query string) string {
	return sqldialect.Dialect(entDialect).Rebind(query)
}

// TxDriver - the context's transaction (a unit of work's) as an ent
// driver, for a module's own client writing in it (modulekit.Driver);
// false outside one.
func TxDriver(ctx context.Context) (dialect.Driver, bool) {
	c, ok := ctx.Value(txKey{}).(*generated.Client)
	if !ok {
		return nil, false
	}
	return InTxDriver(entDialect, c), true
}

// InTxDriver - a transaction running queries through eq (a transaction's
// client) as an ent driver: a transaction ent begins on it (an update
// with edges does) is that one, committed or rolled back by its owner.
func InTxDriver(d string, eq entsql.ExecQuerier) dialect.Driver {
	return inTxDriver{Conn: entsql.Conn{ExecQuerier: eq}, dialect: d}
}

// inTxDriver - InTxDriver's driver.
type inTxDriver struct {
	entsql.Conn
	dialect string
}

// Dialect - the database's.
func (d inTxDriver) Dialect() string { return d.dialect }

// Tx - the same transaction: its Commit and Rollback do nothing.
func (d inTxDriver) Tx(context.Context) (dialect.Tx, error) { return dialect.NopTx(d), nil }

// Close does nothing: the transaction's owner ends it.
func (d inTxDriver) Close() error { return nil }

// Dialect - the database the repositories' ent clients speak (SetDialect):
// a module's own ent client (its own schema) opens its driver with it.
func Dialect() string { return entDialect }

// ClientFor - an ent client on db in the configured dialect (a module's
// repository built on the shared *sql.DB).
func ClientFor(db *sql.DB) *generated.Client { return clientFor(db) }

// OrderRandom - ent order option shuffling the rows (a module's query).
func OrderRandom(s *entsql.Selector) { orderRandom(s) }

// Rebind - a module's hand-written query (? placeholders) in the
// placeholders of the repositories' database.
func Rebind(query string) string { return rebind(query) }

// SettingsRowID - the id of a settings table's single row (a module's
// settings).
const SettingsRowID = settingsRowID
