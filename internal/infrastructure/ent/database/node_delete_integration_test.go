//go:build dbintegration

package database_test

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

func TestNodeDeleteRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "poll", "Poll", "poll")
		seedNode(t, db, 2, "article", "Other", "other")
		// Modules' tables keyed by the node (polls, ratings) cascade.
		seedRow(t, db, `INSERT INTO polls (nid) VALUES (1)`)
		seedRow(t, db, `INSERT INTO rating_votes (nid, guest, score, scale, created_at, updated_at) VALUES (1, 'g', 5, 5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (5, 'see_also', 'reference', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO field_value_reference (node_id, field_id, target_node_id) VALUES (2, 5, 1)`)
		if err := database.NewNodeDeleteRepository(db.DB).DeleteNode(ctx, 1); err != nil {
			t.Fatal(err)
		}
		for _, q := range []string{
			`SELECT COUNT(*) FROM nodes WHERE id = 1`, `SELECT COUNT(*) FROM polls`, `SELECT COUNT(*) FROM rating_votes`,
			`SELECT COUNT(*) FROM field_value_reference`,
		} {
			var n int
			if err := db.DB.QueryRow(q).Scan(&n); err != nil || n != 0 {
				t.Errorf("%s: %d %v", q, n, err)
			}
		}
	})
}
