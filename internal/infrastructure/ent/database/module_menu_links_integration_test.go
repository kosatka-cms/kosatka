//go:build dbintegration

package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestDeleteMenuLinks - a module's uninstall removes the menu links to its
// paths and under them, not a longer path that only starts the same nor a
// content link; a removed link's child takes its place.
func TestDeleteMenuLinks(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		for _, q := range []string{
			`INSERT INTO menus (id, machine_name, label, created_at, updated_at) VALUES (50, 'test', 'Test', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			`INSERT INTO menu_links (id, menu_id, parent_id, title, link_type, link_url, created_at, updated_at) VALUES
				(1, 50, NULL, 'Quotes', 'url', '/quotes', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
				(2, 50, 1, 'By author', 'url', '/quotes/byauthor', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
				(3, 50, 1, 'Home', 'url', '/', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
				(4, 50, NULL, 'Quotes of the day', 'url', '/quotesday', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
				(5, 50, NULL, 'Page two', 'url', '/quotes?page=2', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			`INSERT INTO menu_links (id, menu_id, title, link_type, link_node_id, created_at, updated_at) VALUES
				(6, 50, 'A node', 'node', 7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		} {
			if _, err := db.DB.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
		if err := database.NewModuleRepository(db.DB).DeleteMenuLinks(ctx, []string{"/quotes"}); err != nil {
			t.Fatal(err)
		}
		rows, err := db.DB.Query(`SELECT id, parent_id FROM menu_links WHERE menu_id = 50 ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		left := map[int]sql.NullInt64{}
		for rows.Next() {
			var id int
			var parent sql.NullInt64
			if err := rows.Scan(&id, &parent); err != nil {
				t.Fatal(err)
			}
			left[id] = parent
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if len(left) != 3 || left[3].Valid || !hasKey(left, 4) || !hasKey(left, 6) {
			t.Errorf("left: %v, want 3 (moved to the top), 4 and 6", left)
		}
	})
}

func hasKey[V any](m map[int]V, k int) bool {
	_, ok := m[k]
	return ok
}
