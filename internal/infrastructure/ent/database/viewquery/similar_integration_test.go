//go:build dbintegration

package viewquery_test

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/viewquery"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestRunViewSimilarToCurrentNode - the similar_to_current_node filter
// ranks live by the shared terms weighted by rarity (1 / ln(1 + n), n -
// the published nodes with the term): one rare shared term beats two
// common ones, unpublished and moderated nodes neither show nor count,
// the vocabulary scopes the terms, the View's sort only breaks ties and
// its page is cut from the ranked list.
func TestRunViewSimilarToCurrentNode(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()

		vocabulary := func(name string) int {
			v, err := client.TaxonomyVocabulary.Create().SetName(name).SetSlug(name).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return v.ID
		}
		term := func(vid int, name string) int {
			tm, err := client.TaxonomyTermData.Create().SetVid(vid).SetName(name).SetSlug(name).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return tm.ID
		}
		index := func(tid int, nids ...int) {
			for _, nid := range nids {
				if err := client.TaxonomyIndex.Create().SetNid(nid).SetTid(tid).SetCreatedAt(now).Exec(ctx); err != nil {
					t.Fatal(err)
				}
			}
		}
		node := func(id int, typ string, status, onModeration int) {
			title := fmt.Sprintf("n%02d", id)
			if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO nodes (id, type, title, slug, status, on_moderation, created_by) VALUES (?, ?, ?, ?, ?, ?, 1)`),
				id, typ, title, title, status, onModeration); err != nil {
				t.Fatal(err)
			}
		}

		tags, genres := vocabulary("tags"), vocabulary("genres")
		rare, common, common2 := term(tags, "rare"), term(tags, "common"), term(tags, "common2")
		genre := term(genres, "genre")
		lonely := term(tags, "lonely")

		// Node 1 - the current one. 2..19 articles, 20 a page; 21
		// unpublished and 22 on moderation (both with the rare term, which
		// must neither list them nor make the term commoner); 23 without
		// terms.
		for id := 1; id <= 19; id++ {
			node(id, "article", 1, 0)
		}
		node(20, "page", 1, 0)
		node(21, "article", 0, 0)
		node(22, "article", 1, 1)
		node(23, "article", 1, 0)

		index(rare, 1, 2, 21, 22)                            // n = 2: 1/ln 3 = 0.91
		index(common, 1, 3, 4, 5, 6, 7, 8, 9, 10, 11)        // n = 10: 1/ln 11 = 0.42
		index(common2, 1, 3, 12, 13, 14, 15, 16, 17, 18, 19) // 0.42 too; node 3: 0.83
		index(genre, 1, 20)                                  // 0.91, another vocabulary
		index(lonely, 23)

		repo := viewquery.New(client)
		run := func(current int, vid string, sorts []entity.ViewRunSort, page, pageSize int, more ...entity.ViewFilterDef) ([]int, int) {
			t.Helper()
			filters := append(entity.ViewFilterDefs{{
				BuiltinField: entity.ViewFilterBuiltinSimilarToCurrentNode,
				Operator:     entity.ViewFilterOperatorEq,
				Contextual:   true,
				Value:        vid,
			}}, more...)
			nodes, total, err := repo.RunView(ctx, entity.ViewRunParams{
				Filters: filters, Sorts: sorts, Page: page, PageSize: pageSize, ContextNodeID: current,
			})
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]int, 0, len(nodes))
			for _, n := range nodes {
				ids = append(ids, n.ID)
			}
			return ids, total
		}
		check := func(name string, got []int, total int, want []int, wantTotal int) {
			t.Helper()
			if !slices.Equal(got, want) || total != wantTotal {
				t.Errorf("%s: got %v (total %d), want %v (total %d)", name, got, total, want, wantTotal)
			}
		}

		// Every vocabulary: 2 and 20 tie (one term of 2 nodes each) and
		// keep the ID order without a View sort, then 3 (two common terms),
		// then the nodes sharing one common term.
		all := []int{2, 20, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
		got, total := run(1, "", nil, 1, 50)
		check("every vocabulary", got, total, all, len(all))

		// The View's page and limit apply to the ranked list.
		got, total = run(1, "", nil, 1, 3)
		check("first page of 3", got, total, all[:3], len(all))
		got, total = run(1, "", nil, 2, 3)
		check("second page of 3", got, total, all[3:6], len(all))

		// The View's sort breaks the ties only.
		got, total = run(1, "", []entity.ViewRunSort{{Builtin: entity.ViewSortBuiltinTitle, Direction: "desc"}}, 1, 50)
		check("ties by title descending", got, total,
			[]int{20, 2, 3, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4}, len(all))

		// The vocabulary scopes the compared terms.
		got, total = run(1, strconv.Itoa(tags), nil, 1, 50)
		check("tags only", got, total, slices.DeleteFunc(slices.Clone(all), func(id int) bool { return id == 20 }), len(all)-1)
		got, total = run(1, strconv.Itoa(genres), nil, 1, 50)
		check("genres only", got, total, []int{20}, 1)

		// The View's other filters apply to the ranked candidates.
		typeFilter := entity.ViewFilterDef{BuiltinField: entity.ViewFilterBuiltinContentType, Operator: entity.ViewFilterOperatorEq, Value: "page"}
		got, total = run(1, "", nil, 1, 50, typeFilter)
		check("pages only", got, total, []int{20}, 1)

		// Seen from node 2, node 1 is the one similar (the current node
		// never is); a node whose terms nobody shares, a node without
		// terms and no current node give nothing.
		got, total = run(2, "", nil, 1, 50)
		check("from node 2", got, total, []int{1}, 1)
		got, total = run(23, "", nil, 1, 50)
		check("terms nobody shares", got, total, []int{}, 0)
		if _, err := db.DB.Exec(db.Dialect.Rebind(`DELETE FROM taxonomy_index WHERE nid = ?`), 23); err != nil {
			t.Fatal(err)
		}
		got, total = run(23, "", nil, 1, 50)
		check("no terms", got, total, []int{}, 0)
		got, total = run(0, "", nil, 1, 50)
		check("no current node", got, total, []int{}, 0)
	})
}
