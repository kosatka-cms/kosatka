//go:build dbintegration

package database_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestSlugExistsAcrossTypes - slugs are unique across all nodes, so the
// check sees another type's node too.
func TestSlugExistsAcrossTypes(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "page", "About", "about")
		seedNode(t, db, 2, "news", "News", "news/one")
		nodes := database.NewNodeRepository(entClient(db))
		if ok, err := nodes.SlugExists(ctx, "about"); err != nil || !ok {
			t.Errorf("exists: %v %v", ok, err)
		}
		if ok, err := nodes.SlugExistsExcludingNode(ctx, "about", 2); err != nil || !ok {
			t.Errorf("taken by another type's node: %v %v", ok, err)
		}
		if ok, err := nodes.SlugExistsExcludingNode(ctx, "about", 1); err != nil || ok {
			t.Errorf("its own slug: %v %v", ok, err)
		}
	})
}

// TestNodeAltTitles - the nodes' alternative titles (their types'
// AltTitleField) for a View list, every type at once: a node without
// one, or of a type without the field, is left out.
func TestNodeAltTitles(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()
		for name, alt := range map[string]string{"hero": "field_real_name", "page": ""} {
			if _, err := client.ContentTypeDef.Create().SetMachineName(name).SetLabel(name).SetAltTitleField(alt).
				SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx); err != nil {
				t.Fatal(err)
			}
		}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_real_name").SetFieldType(string(entity.FieldTypeText)).
			SetConfig(map[string]any{}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		seedNode(t, db, 1, "hero", "Captain", "hero/captain")
		seedNode(t, db, 2, "hero", "Mate", "hero/mate")
		seedNode(t, db, 3, "page", "About", "about")
		for nid, v := range map[int]string{1: "Ahab", 3: "Not an alternative title"} {
			if _, err := client.FieldValueText.Create().SetNodeID(nid).SetFieldID(st.ID).SetValue(v).Save(ctx); err != nil {
				t.Fatal(err)
			}
		}
		got, err := database.NewNodeRepository(client).AltTitles(ctx, entity.Nodes{{ID: 1, Type: "hero"}, {ID: 2, Type: "hero"}, {ID: 3, Type: "page"}})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, map[int]string{1: "Ahab"}) {
			t.Fatalf("alternative titles = %v, want only node 1's", got)
		}
	})
}

// TestHiddenNodeTypes - a ctx with hidden types (the visitor's) leaves
// their content and the comments on it out of every query; the site's
// own lookups (WithAllNodeTypes) and the slug check still see it.
func TestHiddenNodeTypes(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		seedUser(t, db)
		seedNode(t, db, 1, "article", "Open", "open")
		seedNode(t, db, 2, "members", "Closed", "closed")
		seedRow(t, db, `INSERT INTO comments (id, nid, content, created_by, status) VALUES (1, 1, 'on open', 1, 1), (2, 2, 'on closed', 1, 1)`)
		client := entClient(db)
		nodes := database.NewNodeRepository(client)
		comments := database.NewCommentRepository(client)
		hidden := entity.WithContentVisitor(context.Background(), entity.ContentVisitor{HiddenTypes: []string{"members"}})

		urls, err := nodes.URLsByIDs(hidden, []int{1, 2})
		if err != nil || len(urls) != 1 || urls[1] == "" {
			t.Errorf("URLsByIDs: %v %v", urls, err)
		}
		if _, err := nodes.ByID(hidden, 2); !entity.IsNotFound(err) {
			t.Errorf("ByID of a hidden type: %v", err)
		}
		if _, err := nodes.BySlug(hidden, "closed"); !entity.IsNotFound(err) {
			t.Errorf("BySlug of a hidden type: %v", err)
		}
		if n, err := nodes.BySlug(entity.WithAllNodeTypes(hidden), "closed"); err != nil || n.ID != 2 {
			t.Errorf("BySlug with all types: %+v %v", n, err)
		}
		if ok, err := nodes.SlugExists(hidden, "closed"); err != nil || !ok {
			t.Errorf("SlugExists of a hidden type's slug: %v %v", ok, err)
		}
		if _, err := comments.ByID(hidden, 2); !entity.IsNotFound(err) {
			t.Errorf("a comment on hidden content: %v", err)
		}
		if c, err := comments.ByID(hidden, 1); err != nil || c == nil {
			t.Errorf("a comment on open content: %v %v", c, err)
		}
		if all, err := nodes.URLsByIDs(context.Background(), []int{1, 2}); err != nil || len(all) != 2 {
			t.Errorf("without hidden types: %v %v", all, err)
		}
	})
}

// TestItemViewRoles - the items with view roles of their own leave
// every query but those of a visitor with one of the roles, their author,
// one who views every item of their type or every item, and the site's
// own lookups; the comments on them too; the roles are matched whole.
func TestItemViewRoles(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		seedUser(t, db)
		seedRow(t, db, `INSERT INTO users (id, username, slug, password_hash) VALUES (2, 'bob', 'bob', '')`)
		seedNode(t, db, 1, "article", "Open", "open")
		seedNode(t, db, 2, "article", "Staff", "staff")
		seedNode(t, db, 3, "article", "Editors", "editors")
		seedRow(t, db, `UPDATE nodes SET view_roles = 'staff,content_editor' WHERE id = 2`)
		seedRow(t, db, `UPDATE nodes SET view_roles = 'content_editor', created_by = 2 WHERE id = 3`)
		seedRow(t, db, `INSERT INTO comments (id, nid, content, created_by, status) VALUES (1, 1, 'on open', 1, 1), (2, 2, 'on staff', 1, 1)`)
		client := entClient(db)
		nodes := database.NewNodeRepository(client)
		comments := database.NewCommentRepository(client)
		ids := func(v entity.ContentVisitor) []int {
			t.Helper()
			ctx := entity.WithContentVisitor(context.Background(), v)
			urls, err := nodes.URLsByIDs(ctx, []int{1, 2, 3})
			if err != nil {
				t.Fatal(err)
			}
			var out []int
			for id := range urls {
				out = append(out, id)
			}
			slices.Sort(out)
			return out
		}
		for name, tc := range map[string]struct {
			v    entity.ContentVisitor
			want []int
		}{
			"guest":               {entity.ContentVisitor{ItemRoles: true, Roles: []string{entity.RoleAnonymous}}, []int{1}},
			"staff":               {entity.ContentVisitor{ItemRoles: true, Roles: []string{"staff", entity.RoleUser}, UserID: 1}, []int{1, 2}},
			"a role like another": {entity.ContentVisitor{ItemRoles: true, Roles: []string{"contentxeditor", "staf"}}, []int{1}},
			"the author":          {entity.ContentVisitor{ItemRoles: true, Roles: []string{entity.RoleUser}, UserID: 2}, []int{1, 3}},
			"editor of the type":  {entity.ContentVisitor{ItemRoles: true, Roles: []string{entity.RoleUser}, ItemTypes: []string{"article"}}, []int{1, 2, 3}},
			"views every item":    {entity.ContentVisitor{ItemRoles: true, AllItems: true}, []int{1, 2, 3}},
			"no item has roles":   {entity.ContentVisitor{Roles: []string{entity.RoleAnonymous}}, []int{1, 2, 3}},
		} {
			if got := ids(tc.v); !slices.Equal(got, tc.want) {
				t.Errorf("%s: %v, want %v", name, got, tc.want)
			}
		}
		guest := entity.WithContentVisitor(context.Background(), entity.ContentVisitor{ItemRoles: true, Roles: []string{entity.RoleAnonymous}})
		if _, err := comments.ByID(guest, 2); !entity.IsNotFound(err) {
			t.Errorf("a comment on an item closed to guests: %v", err)
		}
		if n, err := nodes.BySlug(entity.WithAllNodeTypes(guest), "staff"); err != nil || n.ID != 2 || !slices.Equal(n.ViewRoles, []string{"staff", "content_editor"}) {
			t.Errorf("BySlug with all the content: %+v %v", n, err)
		}
	})
}

// TestSearchByTitlePublicOnly - the pickers' title search finds public
// content only: not unpublished, not on moderation.
func TestSearchByTitlePublicOnly(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		seedUser(t, db)
		seedNode(t, db, 1, "article", "Note public", "public")
		seedNode(t, db, 2, "article", "Note draft", "draft")
		seedNode(t, db, 3, "article", "Note waiting", "waiting")
		seedRow(t, db, `UPDATE nodes SET status = 0 WHERE id = 2`)
		seedRow(t, db, `UPDATE nodes SET on_moderation = 1 WHERE id = 3`)
		hits, err := database.NewNodeRepository(entClient(db)).SearchByTitle(context.Background(), "Note", nil, 10)
		if err != nil || len(hits) != 1 || hits[0].Node.ID != 1 {
			t.Errorf("hits: %+v %v", hits, err)
		}
	})
}

// TestPublicListsSkipUnknownTypes - the sitemap and the search leave out
// the nodes of a type nothing serves: neither a site content type nor a
// registered module's (a module missing from the binary).
func TestPublicListsSkipUnknownTypes(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedRow(t, db, `INSERT INTO content_types (id, machine_name, label, created_by, created_at, updated_at) VALUES (1, 'article', 'Article', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedNode(t, db, 1, "article", "Harbour notes", "harbour-notes")
		seedNode(t, db, 2, "gone_topic", "Harbour rumours", "harbour-rumours")
		nodes := database.NewNodeRepository(entClient(db))
		if n, err := nodes.CountPublicSitemapNodes(ctx, nil); err != nil || n != 1 {
			t.Errorf("sitemap count: %d %v, want 1", n, err)
		}
		if rows, err := nodes.ListPublicSitemapNodes(ctx, nil, 10, 0); err != nil || len(rows) != 1 || rows[0].ID != 1 {
			t.Errorf("sitemap rows: %+v %v", rows, err)
		}
		types := entity.SearchTypeDefs{{NodeType: "article", Limit: 10}, {NodeType: "gone_topic", Limit: 10}}
		if _, err := database.RebuildSearchIndex(ctx, entClient(db)); err != nil {
			t.Fatal(err)
		}
		hits, err := database.NewSearchRepository(entClient(db)).Search(ctx, "Harbour", 10, types, entity.SearchSettings{})
		if err != nil || len(hits) != 1 || hits[0].ID != 1 {
			t.Errorf("search: %+v %v", hits, err)
		}
	})
}

// TestNodeTrash - a node in the trash leaves every query and the comments
// on it theirs, keeps its slug taken, is listed by the trash and found
// with WithTrashed; restored, it is back.
func TestNodeTrash(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "article", "Kept", "kept")
		seedNode(t, db, 2, "article", "Gone", "gone")
		seedRow(t, db, `INSERT INTO comments (id, nid, content, created_by, status) VALUES (1, 2, 'on gone', 1, 1)`)
		client := entClient(db)
		nodes := database.NewNodeRepository(client)
		comments := database.NewCommentRepository(client)
		at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
		if err := nodes.TrashNode(ctx, 2, 1, at); err != nil {
			t.Fatal(err)
		}
		if _, err := nodes.ByID(ctx, 2); !entity.IsNotFound(err) {
			t.Errorf("ByID of a node in the trash: %v", err)
		}
		if _, err := comments.ByID(ctx, 1); !entity.IsNotFound(err) {
			t.Errorf("a comment on a node in the trash: %v", err)
		}
		if ok, err := nodes.SlugExists(ctx, "gone"); err != nil || !ok {
			t.Errorf("the slug of a node in the trash: %v %v", ok, err)
		}
		n, err := nodes.ByID(entity.WithTrashed(ctx), 2)
		if err != nil || !n.InTrash() || n.DeletedBy != 1 {
			t.Errorf("WithTrashed: %+v %v", n, err)
		}
		list, total, err := nodes.TrashedNodes(ctx, 10, 0)
		if err != nil || total != 1 || len(list) != 1 || list[0].ID != 2 {
			t.Errorf("the trash: %d %+v %v", total, list, err)
		}
		if ids, err := nodes.TrashedBefore(ctx, at.Add(time.Hour)); err != nil || !slices.Equal(ids, []int{2}) {
			t.Errorf("trashed before: %v %v", ids, err)
		}
		if ids, err := nodes.TrashedBefore(ctx, at.Add(-time.Hour)); err != nil || len(ids) != 0 {
			t.Errorf("trashed before an earlier time: %v %v", ids, err)
		}
		if err := nodes.RestoreNode(ctx, 2); err != nil {
			t.Fatal(err)
		}
		if n, err := nodes.ByID(ctx, 2); err != nil || n.InTrash() {
			t.Errorf("restored: %+v %v", n, err)
		}
	})
}

// TestDeletionGuardsSeeTrash - what guards a deletion counts the nodes in
// the trash and of the types the visitor may not view: a type, a module
// or a user isn't deleted with such nodes left behind.
func TestDeletionGuardsSeeTrash(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		seedUser(t, db)
		seedNode(t, db, 1, "event", "Gone", "gone")
		client := entClient(db)
		nodes := database.NewNodeRepository(client)
		if err := nodes.TrashNode(context.Background(), 1, 1, time.Now()); err != nil {
			t.Fatal(err)
		}
		// A visitor who may view no event.
		ctx := entity.WithContentVisitor(context.Background(), entity.ContentVisitor{HiddenTypes: []string{"event"}})
		if n, err := nodes.CountByType(ctx, "event"); err != nil || n != 1 {
			t.Errorf("CountByType: %d %v", n, err)
		}
		if ids, err := database.NewModuleUninstallRepository(db.DB).NodeIDsOfTypes(ctx, []string{"event"}, 10); err != nil || len(ids) != 1 {
			t.Errorf("NodeIDsOfTypes: %v %v", ids, err)
		}
	})
}

// TestSetPublishing - the node's publishing and moderation change as the
// site's change, not an edit: its last save and its schedule stay; an
// unknown node is not found.
func TestSetPublishing(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "news", "Waiting", "news/waiting")
		saved := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
		publishOn := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
		if _, err := db.DB.ExecContext(ctx, db.Dialect.Rebind(`UPDATE nodes SET status = ?, on_moderation = ?, updated_at = ?, publish_on = ? WHERE id = ?`), 1, 1, saved, publishOn, 1); err != nil {
			t.Fatal(err)
		}
		nodes := database.NewNodeRepository(entClient(db))
		if err := nodes.SetPublishing(ctx, 1, 0, 0); err != nil {
			t.Fatal(err)
		}
		var status, moderation int
		var updated, scheduled time.Time
		if err := db.DB.QueryRowContext(ctx, `SELECT status, on_moderation, updated_at, publish_on FROM nodes WHERE id = 1`).Scan(&status, &moderation, &updated, &scheduled); err != nil {
			t.Fatal(err)
		}
		if status != 0 || moderation != 0 || !updated.Equal(saved) || !scheduled.Equal(publishOn) {
			t.Errorf("status %d, moderation %d, updated %v (want %v), publish_on %v", status, moderation, updated, saved, scheduled)
		}
		if err := nodes.SetPublishing(ctx, 99, 1, 0); !errors.Is(err, entity.ErrEntityNotFound) {
			t.Errorf("an unknown node: %v", err)
		}
	})
}
