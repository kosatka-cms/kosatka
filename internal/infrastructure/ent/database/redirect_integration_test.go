//go:build dbintegration

package database_test

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

func TestRedirectAndBlockRepositories(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()

		redirects := database.NewRedirectRepository(db.DB)
		red, err := redirects.Create(ctx, entity.Redirect{FromPath: "/old ", ToPath: "new"})
		if err != nil || red.ID == 0 || red.FromPath != "old" || red.ToPath != "/new" {
			t.Fatalf("create redirect: %+v %v", red, err)
		}
		if _, err := redirects.Create(ctx, entity.Redirect{FromPath: "a", ToPath: "/b"}); err != nil {
			t.Fatal(err)
		}
		if list, err := redirects.List(ctx); err != nil || len(list) != 2 || list[0].FromPath != "a" {
			t.Errorf("redirects: %+v %v", list, err)
		}
		_ = redirects.DeleteByFromPath(ctx, "/old")
		_ = redirects.Delete(ctx, red.ID)
		if list, _ := redirects.List(ctx); len(list) != 1 {
			t.Errorf("redirects after delete: %+v", list)
		}

		bots := database.NewBlockedBotRepository(db.DB)
		bot, err := bots.Create(ctx, " BadBot ", "")
		if err != nil || bot.ID == 0 {
			t.Fatalf("create bot: %+v %v", bot, err)
		}
		if list, err := bots.List(ctx); err != nil || len(list) != 1 || list[0].Pattern != "BadBot" || list[0].Comment != "" {
			t.Errorf("bots: %+v %v", list, err)
		}
		_ = bots.Delete(ctx, bot.ID)
		if list, _ := bots.List(ctx); len(list) != 0 {
			t.Errorf("bots after delete: %+v", list)
		}

		ips := database.NewBlockedIPRepository(db.DB)
		ip, err := ips.Create(ctx, "10.0.0.1", "spam")
		if err != nil || ip.ID == 0 {
			t.Fatalf("create ip: %+v %v", ip, err)
		}
		if list, err := ips.List(ctx); err != nil || len(list) != 1 || list[0].Comment != "spam" {
			t.Errorf("ips: %+v %v", list, err)
		}
		_ = ips.Delete(ctx, ip.ID)
		if list, _ := ips.List(ctx); len(list) != 0 {
			t.Errorf("ips after delete: %+v", list)
		}
	})
}

// TestRedirectSearch - a page of the redirects by from path; the search
// finds a part of the from or the to path, letter case aside; the count
// is of all the found ones.
func TestRedirectSearch(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		redirects := database.NewRedirectRepository(db.DB)
		for _, r := range []entity.Redirect{
			{FromPath: "news/old-cats", ToPath: "/article/cats"},
			{FromPath: "about-us", ToPath: "/about"},
			{FromPath: "heroes/wolverin", ToPath: "/heroes/wolverine"},
		} {
			if _, err := redirects.Create(ctx, r); err != nil {
				t.Fatal(err)
			}
		}
		page, total, err := redirects.Search(ctx, "", 1, 1)
		if err != nil || total != 3 || len(page) != 1 || page[0].FromPath != "heroes/wolverin" {
			t.Errorf("the second page of one: %+v %d %v", page, total, err)
		}
		for q, want := range map[string]int{"CATS": 1, "/about": 1, "heroes": 1, "o": 3, "nothing": 0} {
			if _, n, err := redirects.Search(ctx, q, 0, 50); err != nil || n != want {
				t.Errorf("%q: %d %v, want %d", q, n, err, want)
			}
		}
	})
}
