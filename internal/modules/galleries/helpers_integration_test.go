//go:build dbintegration

package galleries

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// insertRow runs an INSERT (? placeholders) of a core row the test needs
// and returns its ID.
func insertRow(t *testing.T, db testkit.DB, query string, args ...any) int {
	t.Helper()
	id, err := db.Dialect.InsertID(context.Background(), db.DB, db.Dialect.Rebind(query), args...)
	if err != nil {
		t.Fatal(err)
	}
	return int(id)
}

// execSQL runs a statement (? placeholders) on the core's tables.
func execSQL(t *testing.T, db testkit.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.DB.ExecContext(context.Background(), db.Dialect.Rebind(query), args...); err != nil {
		t.Fatal(err)
	}
}
