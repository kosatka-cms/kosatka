//go:build dbintegration

package database_test

import (
	"context"
	"maps"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestViewRepositoryModuleCriteria - a module's sort criterion and field
// keep their params (replaced on update, none kept as none), and an
// uninstall removes a module's criteria by their key's prefix, the
// core's and another module's staying.
func TestViewRepositoryModuleCriteria(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		repo := database.NewViewRepository(entClient(db))
		display, err := repo.CreateView(ctx, entity.ViewDisplayDef{Label: "Top", MachineName: "top", DisplayType: entity.ViewDisplayTypePage, URLPath: "top", BaseEntity: entity.ViewBaseEntityContent})
		if err != nil {
			t.Fatal(err)
		}
		sort, err := repo.CreateSort(ctx, entity.ViewSortDef{ViewDisplayID: display.ID, BuiltinField: "stars.weighted", Direction: "desc", Params: map[string]string{"min": "5"}})
		if err != nil {
			t.Fatal(err)
		}
		if !maps.Equal(sort.Params, map[string]string{"min": "5"}) {
			t.Errorf("created sort params %v", sort.Params)
		}
		sort.Params = map[string]string{"min": "20"}
		if sort, err = repo.UpdateSort(ctx, *sort); err != nil || sort.Params["min"] != "20" {
			t.Fatalf("updated sort params %v, %v", sort, err)
		}
		builtin, err := repo.CreateSort(ctx, entity.ViewSortDef{ViewDisplayID: display.ID, BuiltinField: entity.ViewSortBuiltinCreatedAt, Direction: "desc", SortOrder: 10, ExposedKey: "new", ExposedLabel: "Newest"})
		if err != nil || builtin.Params != nil {
			t.Fatalf("a builtin's params %v, %v", builtin, err)
		}
		// The offer to visitors is kept, and changed on update.
		if builtin.ExposedKey != "new" || builtin.ExposedLabel != "Newest" {
			t.Errorf("created sort's offer %q, %q", builtin.ExposedKey, builtin.ExposedLabel)
		}
		builtin.ExposedKey, builtin.ExposedLabel = "", ""
		if updated, err := repo.UpdateSort(ctx, *builtin); err != nil || updated.ExposedKey != "" || updated.ExposedLabel != "" {
			t.Fatalf("updated sort's offer %v, %v", updated, err)
		}

		field, err := repo.CreateViewField(ctx, entity.ViewFieldDef{ViewDisplayID: display.ID, BuiltinField: "stars.rating", Params: map[string]string{"look": "stars"}})
		if err != nil {
			t.Fatal(err)
		}
		field.Params = nil
		if field, err = repo.UpdateViewField(ctx, *field); err != nil || field.Params != nil {
			t.Fatalf("a field's params cleared: %v, %v", field, err)
		}
		if _, err := repo.CreateViewField(ctx, entity.ViewFieldDef{ViewDisplayID: display.ID, BuiltinField: "votes.total"}); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateFilter(ctx, entity.ViewFilterDef{ViewDisplayID: display.ID, BuiltinField: "stars.count", Operator: entity.ViewFilterOperatorGte, Value: "3"}); err != nil {
			t.Fatal(err)
		}

		if err := database.NewModuleUninstallRepository(db.DB).DeleteViewCriteria(ctx, "stars."); err != nil {
			t.Fatal(err)
		}
		sorts, _ := repo.SortsByViewID(ctx, display.ID)
		fields, _ := repo.FieldsByViewID(ctx, display.ID)
		filters, _ := repo.FiltersByViewID(ctx, display.ID)
		if len(sorts) != 1 || sorts[0].BuiltinField != entity.ViewSortBuiltinCreatedAt || len(fields) != 1 || fields[0].BuiltinField != "votes.total" || len(filters) != 0 {
			t.Errorf("after the uninstall: sorts %+v, fields %+v, filters %+v", sorts, fields, filters)
		}
	})
}
