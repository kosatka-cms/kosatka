//go:build dbintegration

package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestNodeCounterRepository - a view counts in all, today (from 1 again
// the next day) and in today's row of the views by day; the days before
// a day are deleted, the later ones stay.
func TestNodeCounterRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "article", "A", "a")
		counter := database.NewNodeCounterRepository(db.DB)
		for range 3 { // creates the row, then counts on it
			if err := counter.IncrementView(ctx, 1); err != nil {
				t.Fatal(err)
			}
		}
		var total, day int
		if err := db.DB.QueryRow(`SELECT totalcount, daycount FROM node_counter WHERE nid = 1`).Scan(&total, &day); err != nil || total != 3 || day != 3 {
			t.Errorf("counter: %d %d %v", total, day, err)
		}
		// Last viewed yesterday: the day count starts over.
		seedRow(t, db, `UPDATE node_counter SET last_viewed = '2000-01-01 00:00:00' WHERE nid = 1`)
		if err := counter.IncrementView(ctx, 1); err != nil {
			t.Fatal(err)
		}
		_ = db.DB.QueryRow(`SELECT totalcount, daycount FROM node_counter WHERE nid = 1`).Scan(&total, &day)
		if total != 4 || day != 1 {
			t.Errorf("counter after a day: %d %d", total, day)
		}
		today := entity.ViewDay(time.Now())
		viewsOf := func(d int) int {
			t.Helper()
			var n int
			_ = db.DB.QueryRow(db.Dialect.Rebind(`SELECT COALESCE(SUM(views), 0) FROM node_view_days WHERE nid = 1 AND day = ?`), d).Scan(&n)
			return n
		}
		if n := viewsOf(today); n != 4 {
			t.Errorf("views today by day: %d, want 4", n)
		}
		seedRow(t, db, `INSERT INTO node_view_days (nid, day, views) VALUES (1, 20000101, 5)`)
		if err := counter.DeleteViewDaysBefore(ctx, 20000102); err != nil {
			t.Fatal(err)
		}
		if viewsOf(20000101) != 0 || viewsOf(today) != 4 {
			t.Errorf("after deleting: old %d, today %d", viewsOf(20000101), viewsOf(today))
		}
	})
}
