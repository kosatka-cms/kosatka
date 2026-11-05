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

// TestRecentRevisions - the recent changes: every node's revisions
// newest first, each with its node's title, type and address and the
// node's revision before it; only the nodes the viewer may see (any of
// AnyTypes, their own published ones of OwnTypes); the filter by a part
// of the title, the type, a part of the saver's name and the days; a
// change of the publishing alone carries the status.
func TestRecentRevisions(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedRow(t, db, `INSERT INTO users (id, username, slug, password_hash) VALUES (2, 'bob', 'bob', '')`)
		seedNode(t, db, 10, "article", "Cache for a CDN", "article/cache")
		seedNode(t, db, 11, "page", "About", "about")
		if _, err := db.DB.Exec(db.Dialect.Rebind(`UPDATE nodes SET created_by = 2 WHERE id = ?`), 11); err != nil {
			t.Fatal(err)
		}
		repo := database.NewNodeRevisionRepository(entClient(db))
		day := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
		add := func(node, user int, at time.Time, kind entity.RevisionKind, changed []string, status int) int {
			t.Helper()
			id, err := repo.Create(ctx, entity.NodeRevision{NodeID: node, User: &entity.User{ID: user}, CreatedAt: at, Kind: kind,
				Changed: changed, Snapshot: entity.NodeSnapshot{Title: "t", Status: status}})
			if err != nil {
				t.Fatal(err)
			}
			return id
		}
		a1 := add(10, 1, day, entity.RevisionCreate, nil, 1)
		p1 := add(11, 2, day.Add(time.Hour), entity.RevisionCreate, nil, 1)
		a2 := add(10, 2, day.AddDate(0, 0, 1), entity.RevisionEdit, []string{entity.SnapshotTitle}, 1)
		a3 := add(10, 1, day.AddDate(0, 0, 2), entity.RevisionEdit, []string{entity.SnapshotStatus}, 0)

		all := entity.RecentRevisionsFilter{AnyTypes: []string{"article", "page"}, Limit: 50}
		got, total, err := repo.Recent(ctx, all)
		if err != nil || total != 4 || len(got) != 4 {
			t.Fatalf("all: %+v %d %v", got, total, err)
		}
		if r := got[0]; r.ID != a3 || r.PreviousID != a2 || r.NodeTitle != "Cache for a CDN" || r.NodeType != "article" ||
			r.NodeURL != "/article/cache" || r.User == nil || r.User.Username != "alice" || r.Snapshot.Status != 0 {
			t.Errorf("newest: %+v", r)
		}
		if got[1].ID != a2 || got[1].PreviousID != a1 || got[2].ID != p1 || got[2].PreviousID != 0 || got[3].ID != a1 || got[3].PreviousID != 0 {
			t.Errorf("order and previous: %+v", got)
		}
		if got[1].Snapshot.Status != 0 {
			t.Error("a revision changing more than the publishing carries no status")
		}

		count := func(f entity.RecentRevisionsFilter) int {
			t.Helper()
			f.Limit = 50
			_, n, err := repo.Recent(ctx, f)
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
		for name, tc := range map[string]struct {
			f    entity.RecentRevisionsFilter
			want int
		}{
			"title":    {entity.RecentRevisionsFilter{AnyTypes: all.AnyTypes, Title: "cdn"}, 3},
			"type":     {entity.RecentRevisionsFilter{AnyTypes: all.AnyTypes, Type: "page"}, 1},
			"who":      {entity.RecentRevisionsFilter{AnyTypes: all.AnyTypes, Actor: "BO"}, 2},
			"days":     {entity.RecentRevisionsFilter{AnyTypes: all.AnyTypes, From: day.AddDate(0, 0, 1), To: day.AddDate(0, 0, 2)}, 1},
			"any type": {entity.RecentRevisionsFilter{AnyTypes: []string{"page"}}, 1},
			"own":      {entity.RecentRevisionsFilter{OwnTypes: []string{"article", "page"}, OwnerID: 2}, 1},
			"none":     {entity.RecentRevisionsFilter{}, 0},
		} {
			if n := count(tc.f); n != tc.want {
				t.Errorf("%s: %d, want %d", name, n, tc.want)
			}
		}
		// Their own, but unpublished: not theirs to see.
		if _, err := db.DB.Exec(db.Dialect.Rebind(`UPDATE nodes SET status = 0 WHERE id = ?`), 11); err != nil {
			t.Fatal(err)
		}
		if n := count(entity.RecentRevisionsFilter{OwnTypes: []string{"page"}, OwnerID: 2}); n != 0 {
			t.Errorf("own unpublished: %d", n)
		}
	})
}
