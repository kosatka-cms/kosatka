//go:build dbintegration

package database_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

func TestUserContentRepositories(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedRow(t, db, `INSERT INTO users (id, username, slug, password_hash) VALUES (2, 'bob', 'users/bob', '')`)
		seedNode(t, db, 1, "article", "A", "a")
		seedNode(t, db, 2, "article", "B", "b")
		seedNode(t, db, 3, "news", "C", "c")
		seedRow(t, db, `INSERT INTO comments (nid, content, created_by, status) VALUES (1, 'see /uploads/u1/pic.png', 1, 1)`)
		seedRow(t, db, `INSERT INTO files (id, uri, created_by) VALUES (1, 'public://a.png', 1), (2, 'public://b.png', 1), (3, 'public://b.png', 2), (4, 'public://c.png', 1)`)
		seedRow(t, db, `INSERT INTO galleries (id, nid) VALUES (1, 1)`)
		seedRow(t, db, `INSERT INTO gallery_items (gallery_id, fid) VALUES (1, 1)`)
		seedRow(t, db, `INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (9, 'doc', 'file', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO field_value_file (node_id, file_id, field_id) VALUES (2, 4, 9)`)

		content := database.NewUserContentRepository(db.DB)
		s, err := content.Summary(ctx, 1)
		want := []entity.UserContentTypeCount{{Type: "article", Count: 2}, {Type: "news", Count: 1}}
		if err != nil || !reflect.DeepEqual(s.NodesByType, want) || s.Nodes != 3 || s.Comments != 1 || s.Files != 3 {
			t.Errorf("summary: %+v %v", s, err)
		}
		if ids, err := content.NodeIDs(ctx, 1, 2); err != nil || !reflect.DeepEqual(ids, []int{1, 2}) {
			t.Errorf("node ids: %v %v", ids, err)
		}
		if ids, err := content.NodeFileIDs(ctx, []int{1, 2}); err != nil || !reflect.DeepEqual(ids, []int{1, 4}) {
			t.Errorf("node file ids: %v %v", ids, err)
		}
		// 1 and 4 are used; 2 goes, but its file is shared with row 3.
		if uris, err := content.DeleteUnreferencedFiles(ctx, []int{1, 2, 4}); err != nil || len(uris) != 0 {
			t.Errorf("removable uris: %v %v", uris, err)
		}
		if ids, _ := content.FileIDsByCreator(ctx, 1); !reflect.DeepEqual(ids, []int{1, 4}) {
			t.Errorf("files left: %v", ids)
		}

		usage := database.NewUserFileUsageRepository(db.DB)
		var found []entity.UserFileUsage
		err = usage.ScanTextsFor(ctx, "u1/pic.png", func(source string, id int, _ string) {
			found = append(found, entity.UserFileUsage{Source: source, EntityID: id})
		})
		if err != nil || len(found) != 1 || found[0].Source != entity.UserFileSourceComment {
			t.Fatalf("scan: %+v %v", found, err)
		}
		found = append(found, entity.UserFileUsage{Source: entity.UserFileSourceSignature, EntityID: 2})
		refs, err := usage.Describe(ctx, found)
		if err != nil || len(refs) != 2 || refs[0].Title != "A" || refs[1].Title != "bob" || refs[1].URL != "/users/bob" {
			t.Errorf("describe: %+v %v", refs, err)
		}
	})
}

// TestNodesWithFile - the items whose file fields (one or several values)
// hold a file; a blocks field's item counts as its node.
func TestNodesWithFile(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "article", "A", "a")
		seedNode(t, db, 2, "article", "B", "b")
		seedNode(t, db, 5, "article", "Block", "block")
		seedRow(t, db, `UPDATE nodes SET parent_id = 1 WHERE id = 5`)
		seedRow(t, db, `INSERT INTO files (id, uri, created_by) VALUES (7, 'private://content/files/a.pdf', 1), (8, 'private://content/files/b.pdf', 1)`)
		seedRow(t, db, `INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (9, 'doc', 'file', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO field_value_file (node_id, file_id, field_id) VALUES (5, 7, 9)`)
		seedRow(t, db, `INSERT INTO field_value_file_multi (node_id, file_id, field_id, delta) VALUES (2, 7, 9, 0), (1, 7, 9, 0)`)

		content := database.NewUserContentRepository(db.DB)
		ids, err := content.NodesWithFile(ctx, 7)
		sort.Ints(ids)
		if err != nil || !reflect.DeepEqual(ids, []int{1, 2}) {
			t.Errorf("nodes with file 7: %v %v", ids, err)
		}
		if ids, err := content.NodesWithFile(ctx, 8); err != nil || len(ids) != 0 {
			t.Errorf("nodes with file 8: %v %v", ids, err)
		}
	})
}
