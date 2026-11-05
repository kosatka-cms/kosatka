//go:build dbintegration

package viewquery_test

import (
	"context"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/viewquery"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestRunViewModuleCriteria - a module's sort criterion orders by the
// module's numbers (its params passed, a node it leaves out at 0, ties by
// ascending ID, as every criterion sorted in Go) and its filter keeps the nodes whose number passes, a page and the
// total counting only them, the glossary too.
func TestRunViewModuleCriteria(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		for i, slug := range []string{"one", "two", "three", "four"} {
			seedNode(t, db, i+1, "article", slug, "article/"+slug)
		}
		// Node 4 has no rating.
		ratings := map[int]float64{1: 3.5, 2: 4.8, 3: 3.5}
		var gotParams map[string]string
		sort := entity.ViewModuleSort{Module: "stars", Key: "stars.weighted",
			Values: func(_ context.Context, ids []int, params map[string]string) (map[int]float64, error) {
				gotParams = params
				out := map[int]float64{}
				for _, id := range ids {
					if v, ok := ratings[id]; ok {
						out[id] = v
					}
				}
				return out, nil
			}}
		filter := entity.ViewModuleFilter{Module: "stars", Key: "stars.average",
			Values: func(_ context.Context, ids []int) (map[int]float64, error) {
				out := map[int]float64{}
				for _, id := range ids {
					if v, ok := ratings[id]; ok {
						out[id] = v
					}
				}
				return out, nil
			}}
		ids := func(nodes entity.Nodes) []int {
			out := make([]int, 0, len(nodes))
			for _, n := range nodes {
				out = append(out, n.ID)
			}
			return out
		}
		repo := viewquery.New(entClient(db))
		bySort := []entity.ViewRunSort{{Builtin: "stars.weighted", Direction: "desc", Module: &sort, Params: map[string]string{"min": "5"}}}

		nodes, total, err := repo.RunView(ctx, entity.ViewRunParams{Sorts: bySort, PageSize: 10})
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(nodes); total != 4 || !slices.Equal(got, []int{2, 1, 3, 4}) {
			t.Errorf("by rating: %v (total %d), want [2 1 3 4]", got, total)
		}
		if gotParams["min"] != "5" {
			t.Errorf("the criterion got params %v", gotParams)
		}

		atLeast := func(op entity.ViewFilterOperator, value string) []entity.ViewRunModuleFilter {
			return []entity.ViewRunModuleFilter{{Def: entity.ViewFilterDef{BuiltinField: "stars.average", Operator: op, Value: value}, Filter: filter}}
		}
		for _, tc := range []struct {
			name    string
			filters []entity.ViewRunModuleFilter
			want    []int
		}{
			{"at least 3.5", atLeast(entity.ViewFilterOperatorGte, "3.5"), []int{2, 1, 3}},
			{"at least 4", atLeast(entity.ViewFilterOperatorGte, "4"), []int{2}},
			{"none rated", atLeast(entity.ViewFilterOperatorEmpty, ""), []int{4}},
			{"none passes", atLeast(entity.ViewFilterOperatorGte, "5"), []int{}},
		} {
			nodes, total, err := repo.RunView(ctx, entity.ViewRunParams{Sorts: bySort, ModuleFilters: tc.filters, PageSize: 2})
			if err != nil {
				t.Fatal(err)
			}
			want := tc.want[:min(2, len(tc.want))]
			if got := ids(nodes); total != len(tc.want) || !slices.Equal(got, want) {
				t.Errorf("%s: page %v (total %d), want %v (total %d)", tc.name, got, total, want, len(tc.want))
			}
		}

		// An exposed filter applies only when asked.
		exposed := []entity.ViewRunModuleFilter{{Def: entity.ViewFilterDef{BuiltinField: "stars.average", Operator: entity.ViewFilterOperatorGte, Exposed: true, ExposedParam: "rating"}, Filter: filter}}
		for query, want := range map[string]int{"": 4, "4": 1} {
			_, total, err := repo.RunView(ctx, entity.ViewRunParams{ModuleFilters: exposed, QueryParams: map[string]string{"rating": query}, PageSize: 10})
			if err != nil {
				t.Fatal(err)
			}
			if total != want {
				t.Errorf("exposed rating=%q: total %d, want %d", query, total, want)
			}
		}

		glossary, err := repo.GlossaryForView(ctx, nil, atLeast(entity.ViewFilterOperatorGte, "4"), nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(glossary) != 1 || glossary[0].Letter != "T" || glossary[0].Count != 1 {
			t.Errorf("glossary = %+v, want T: 1 (node \"two\")", glossary)
		}
	})
}

// TestRunViewModuleChoiceFilter - a module's choice filter keeps the
// nodes whose value is the one, or one of the several, asked; empty -
// those without one; exposed - only when asked.
func TestRunViewModuleChoiceFilter(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		for i, slug := range []string{"one", "two", "three", "four"} {
			seedNode(t, db, i+1, "article", slug, "article/"+slug)
		}
		states := map[int]string{1: "draft", 2: "review", 3: "review"}
		filter := entity.ViewModuleFilter{Module: "flow", Key: "flow.state",
			Options: func(context.Context) ([]entity.ViewModuleOption, error) { return nil, nil },
			Choices: func(_ context.Context, ids []int) (map[int]string, error) {
				out := map[int]string{}
				for _, id := range ids {
					if v, ok := states[id]; ok {
						out[id] = v
					}
				}
				return out, nil
			}}
		repo := viewquery.New(entClient(db))
		for _, tc := range []struct {
			name string
			def  entity.ViewFilterDef
			q    string
			want int
		}{
			{"in review", entity.ViewFilterDef{Operator: entity.ViewFilterOperatorEq, Value: "review"}, "", 2},
			{"draft or review", entity.ViewFilterDef{Operator: entity.ViewFilterOperatorIn, Value: "draft, review"}, "", 3},
			{"none", entity.ViewFilterDef{Operator: entity.ViewFilterOperatorEmpty}, "", 1},
			{"exposed, not asked", entity.ViewFilterDef{Operator: entity.ViewFilterOperatorEq, Exposed: true, ExposedParam: "state"}, "", 4},
			{"exposed draft", entity.ViewFilterDef{Operator: entity.ViewFilterOperatorEq, Exposed: true, ExposedParam: "state"}, "draft", 1},
		} {
			tc.def.BuiltinField = "flow.state"
			_, total, err := repo.RunView(ctx, entity.ViewRunParams{ModuleFilters: []entity.ViewRunModuleFilter{{Def: tc.def, Filter: filter}},
				QueryParams: map[string]string{"state": tc.q}, PageSize: 10})
			if err != nil {
				t.Fatal(err)
			}
			if total != tc.want {
				t.Errorf("%s: %d, want %d", tc.name, total, tc.want)
			}
		}
	})
}
