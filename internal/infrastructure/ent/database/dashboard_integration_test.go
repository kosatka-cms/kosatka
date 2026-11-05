//go:build dbintegration

package database_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestDashboardRepository - the Overview's figures: the published content,
// the comments and the users, all and recent; the views today (a node's
// day count only if it was viewed today) and all; the content on
// moderation; the most viewed published content of a period (today - the
// day count, 7 and 30 days - the views by day, all time - the total),
// most first, of a type, a page of it.
func TestDashboardRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		now := time.Now().UTC().Truncate(time.Second)
		old := now.AddDate(0, 0, -30)
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := db.DB.Exec(db.Dialect.Rebind(q), args...); err != nil {
				t.Fatal(err)
			}
		}
		exec(`INSERT INTO users (id, username, slug, password_hash, created_at) VALUES (2, 'bob', 'bob', '', ?)`, old)
		seedNode(t, db, 10, "article", "Fresh", "fresh")
		seedNode(t, db, 11, "article", "Old", "old")
		seedNode(t, db, 12, "article", "Draft", "draft")
		exec(`UPDATE nodes SET created_at = ? WHERE id = 11`, old)
		exec(`UPDATE nodes SET status = 0, on_moderation = 1 WHERE id = 12`)
		exec(`INSERT INTO comments (id, nid, created_by, content, status, created_at, updated_at) VALUES (1, 10, 1, 'new', 1, ?, ?)`, now, now)
		exec(`INSERT INTO comments (id, nid, created_by, content, status, created_at, updated_at) VALUES (2, 11, 1, 'old', 1, ?, ?)`, old, old)
		exec(`INSERT INTO node_counter (nid, totalcount, daycount, last_viewed) VALUES (10, 100, 7, ?)`, now)
		exec(`INSERT INTO node_counter (nid, totalcount, daycount, last_viewed) VALUES (11, 50, 9, ?)`, old)
		exec(`INSERT INTO node_counter (nid, totalcount, daycount, last_viewed) VALUES (12, 5, 3, ?)`, now)
		for _, v := range []struct{ nid, ago, views int }{
			{10, 0, 2}, {10, 3, 4}, {10, 20, 10}, {10, 40, 1000},
			{11, 0, 1}, {11, 10, 30},
			{12, 0, 100},
		} {
			exec(`INSERT INTO node_view_days (nid, day, views) VALUES (?, ?, ?)`, v.nid, entity.ViewDay(time.Now().AddDate(0, 0, -v.ago)), v.views)
		}

		repo := database.NewDashboardRepository(db.DB)
		today := now.Add(-time.Hour)
		st, err := repo.Stats(ctx, now.AddDate(0, 0, -7), today)
		if err != nil {
			t.Fatal(err)
		}
		if st.Published != 2 || st.PublishedSince != 1 || st.Comments != 2 || st.CommentsSince != 1 ||
			st.Users != 2 || st.UsersSince != 1 || st.ViewsTotal != 155 || st.ViewsToday != 10 {
			t.Errorf("stats: %+v", st)
		}
		if n, err := repo.OnModeration(ctx); err != nil || n != 1 {
			t.Errorf("on moderation: %d %v", n, err)
		}
		popular, total, err := repo.Popular(ctx, entity.PopularQuery{Period: entity.PopularPeriodToday, Today: today, Limit: 5})
		if err != nil || total != 1 || len(popular) != 1 || popular[0].Node.Title != "Fresh" || popular[0].Views != 7 ||
			popular[0].Total != 100 || popular[0].Node.URL() != "/fresh" {
			t.Errorf("popular today: %+v %d %v", popular, total, err)
		}
		for _, c := range []struct {
			q    entity.PopularQuery
			want string
		}{
			{entity.PopularQuery{Period: entity.PopularPeriodWeek, Limit: 5}, "10:6/100 11:1/50"},
			{entity.PopularQuery{Period: entity.PopularPeriodMonth, Limit: 5}, "11:31/50 10:16/100"},
			{entity.PopularQuery{Period: entity.PopularPeriodMonth, Limit: 1, Offset: 1}, "10:16/100"},
			{entity.PopularQuery{Period: entity.PopularPeriodAll, Limit: 5}, "10:100/100 11:50/50"},
			{entity.PopularQuery{Period: entity.PopularPeriodAll, Type: "page", Limit: 5}, ""},
		} {
			c.q.Today = time.Now()
			rows, total, err := repo.Popular(ctx, c.q)
			got := ""
			for _, p := range rows {
				got += fmt.Sprintf(" %d:%d/%d", p.Node.ID, p.Views, p.Total)
			}
			if err != nil || strings.TrimSpace(got) != c.want || (c.want != "" && total != 2) {
				t.Errorf("popular %+v: %q (%d), want %q; %v", c.q, got, total, c.want, err)
			}
		}
	})
}
