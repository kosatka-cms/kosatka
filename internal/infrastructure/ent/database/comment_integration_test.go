//go:build dbintegration

package database_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

func TestCommentCountsAndFieldUsage(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		client := entClient(db)
		seedUser(t, db)

		seedNode(t, db, 1, "article", "A", "a")
		seedNode(t, db, 2, "article", "B", "b")
		seedRow(t, db, `INSERT INTO comments (id, nid, content, created_by, status) VALUES (1, 1, 'x', 1, 1)`)
		seedRow(t, db, `INSERT INTO comments (id, nid, content, created_by, status, parent_id) VALUES (2, 1, 'reply', 1, 1, 1), (3, 2, 'y', 1, 1, NULL), (4, 2, 'hidden', 1, 0, NULL)`)
		comments := database.NewCommentRepository(client)
		if got, err := comments.CountByNodeIDs(ctx, []int{1, 2, 3}, 1, false); err != nil || !reflect.DeepEqual(got, map[int]int{1: 2, 2: 1}) {
			t.Errorf("comment counts: %+v %v", got, err)
		}
		if got, err := comments.CountByNodeIDs(ctx, []int{1, 2}, 1, true); err != nil || !reflect.DeepEqual(got, map[int]int{1: 1, 2: 1}) {
			t.Errorf("root comment counts: %+v %v", got, err)
		}

		seedRow(t, db, `INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (3, 'tags', 'taxonomy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO seo_rules (content_type, field_storage_id, term_ids, title_template, description_template, weight, created_at, updated_at) VALUES ('news', 3, '', '', '', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		if u, err := database.NewContentTypeRepository(client).FieldStorageUsage(ctx, 3); err != nil || u.SEORules != 1 || u.Removable() {
			t.Errorf("field storage usage: %+v %v", u, err)
		}
	})
}

// TestCommentListsPublicNodesOnly - the public comment lists (the recent
// comments page and block) leave out the comments on unpublished content,
// content on moderation, content of an excluded type (a disabled
// module's) and of a type nothing serves (a module missing from the
// binary).
func TestCommentListsPublicNodesOnly(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedRow(t, db, `INSERT INTO content_types (id, machine_name, label, created_by, created_at, updated_at) VALUES (1, 'article', 'Article', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), (2, 'note', 'Note', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedNode(t, db, 1, "article", "Public", "public")
		seedNode(t, db, 2, "article", "Draft", "draft")
		seedNode(t, db, 3, "article", "Waiting", "waiting")
		seedNode(t, db, 4, "note", "Excluded", "excluded")
		seedNode(t, db, 5, "gone_topic", "Gone", "gone")
		seedRow(t, db, `UPDATE nodes SET status = 0 WHERE id = 2`)
		seedRow(t, db, `UPDATE nodes SET on_moderation = 1 WHERE id = 3`)
		seedRow(t, db, `INSERT INTO comments (id, nid, content, created_by, status) VALUES (1, 1, 'a', 1, 1), (2, 2, 'b', 1, 1), (3, 3, 'c', 1, 1), (4, 4, 'd', 1, 1), (5, 5, 'e', 1, 1)`)
		comments := database.NewCommentRepository(entClient(db))

		recent, total, err := comments.ListRecentPublished(ctx, 1, []string{"note"}, 10, 0)
		if err != nil || total != 1 || len(recent) != 1 || recent[0].ID != 1 {
			t.Errorf("recent: %d %+v %v", total, recent, err)
		}
		latest, err := comments.GetLatestComments(ctx, 10, 1, []string{"note"})
		if err != nil || len(latest) != 1 || latest[0].ID != 1 {
			t.Errorf("latest: %+v %v", latest, err)
		}
	})
}
