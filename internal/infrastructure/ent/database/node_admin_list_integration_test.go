//go:build dbintegration

package database_test

import (
	"context"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestListForAdminSearch - the admin's content list searches a part of
// the title in any case, or of the address.
func TestListForAdminSearch(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "article", "Морские глубины", "article/sea")
		seedNode(t, db, 2, "article", "Озеро", "article/lake")
		seedNode(t, db, 3, "page", "О проекте", "about")
		repo := database.NewNodeRepository(entClient(db))
		for q, want := range map[string][]int{
			"морские": {1},
			"ОЗЕР":    {2},
			"article": {1, 2},
			"about":   {3},
			"нет":     {},
		} {
			list, total, err := repo.ListForAdmin(ctx, entity.AdminNodesFilter{Search: q, SortBy: entity.AdminNodesSortCreated, Limit: 10})
			if err != nil {
				t.Fatal(err)
			}
			var got []int
			for _, n := range *list {
				got = append(got, n.ID)
			}
			slices.Sort(got)
			if total != len(want) || !slices.Equal(got, want) && len(want) > 0 {
				t.Errorf("search %q: %v (total %d), want %v", q, got, total, want)
			}
		}
	})
}
