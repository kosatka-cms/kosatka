package views

import (
	"context"
	"errors"
	"html/template"
	"maps"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// useModuleCriteria registers a "stars" module's sort, filter and field
// for the test (the module on unless off says otherwise) and drops them
// after it.
func useModuleCriteria(t *testing.T, off map[string]bool) {
	t.Helper()
	ResetModuleCriteria()
	SetModuleEnabled(func(name string) bool { return !off[name] })
	t.Cleanup(func() {
		ResetModuleCriteria()
		SetModuleEnabled(nil)
	})
	threshold := entity.ViewModuleParam{Key: "min", Label: "Threshold", Type: entity.PluginBlockSettingNumber, Default: "5", Min: 1, Max: 100}
	AddModuleSort(entity.ViewModuleSort{Module: "stars", Key: "stars.weighted", Label: "Weighted", Params: []entity.ViewModuleParam{threshold},
		Values: func(context.Context, []int, map[string]string) (map[int]float64, error) {
			return map[int]float64{1: 2}, nil
		}})
	AddModuleFilter(entity.ViewModuleFilter{Module: "stars", Key: "stars.count", Label: "Count",
		Values: func(context.Context, []int) (map[int]float64, error) { return nil, nil }})
	look := entity.ViewModuleParam{Key: "look", Label: "Look", Type: entity.PluginBlockSettingSelect, Default: "stars",
		Options: []entity.PluginBlockTarget{{Value: "stars"}, {Value: "average"}}}
	AddModuleField(entity.ViewModuleField{Module: "stars", Key: "stars.rating", Label: "Rating", Params: []entity.ViewModuleParam{look},
		Render: func(_ context.Context, ids []int, params map[string]string) (map[int]template.HTML, error) {
			out := map[int]template.HTML{}
			for _, id := range ids {
				if id == 1 {
					out[id] = template.HTML(`<span class="stars">` + params["look"] + `</span>`)
				}
			}
			return out, nil
		}})
}

func TestModuleCriteriaRegistry(t *testing.T) {
	t.Run("a key is the module's, short enough, once", func(t *testing.T) {
		useModuleCriteria(t, nil)
		for name, s := range map[string]entity.ViewModuleSort{
			"another module's prefix": {Module: "stars", Key: "votes.weighted"},
			"no name":                 {Module: "stars", Key: "stars."},
			"too long":                {Module: "stars", Key: "stars.a_name_longer_than_the_column"},
			"registered twice":        {Module: "stars", Key: "stars.weighted"},
		} {
			func() {
				defer func() {
					if recover() == nil {
						t.Errorf("%s: AddModuleSort(%q) didn't panic", name, s.Key)
					}
				}()
				AddModuleSort(s)
			}()
		}
	})

	t.Run("a module that is off offers nothing", func(t *testing.T) {
		useModuleCriteria(t, map[string]bool{"stars": true})
		if len(ModuleSorts())+len(ModuleFilters())+len(ModuleFields()) != 0 {
			t.Fatal("the criteria of a module that is off are offered")
		}
		if _, ok := ModuleSort("stars.weighted"); ok {
			t.Fatal("ModuleSort found the criterion of a module that is off")
		}
		if !IsModuleKey("stars.weighted") || IsModuleKey(string(entity.ViewSortBuiltinViews)) {
			t.Fatal("IsModuleKey tells a module's key from a builtin wrongly")
		}
	})
}

func TestModuleCriteriaValidation(t *testing.T) {
	useModuleCriteria(t, nil)
	s := &DefaultService{}
	ctx := t.Context()

	t.Run("a sort keeps the values its params accept", func(t *testing.T) {
		sd := entity.ViewSortDef{BuiltinField: "stars.weighted", Params: map[string]string{"min": "500", "x": "1"}}
		if err := s.resolveSortTarget(ctx, &sd); err != nil {
			t.Fatal(err)
		}
		if !maps.Equal(sd.Params, map[string]string{"min": "5"}) {
			t.Errorf("Params = %v, want the default threshold only", sd.Params)
		}
		builtin := entity.ViewSortDef{BuiltinField: entity.ViewSortBuiltinViews, Params: map[string]string{"min": "5"}}
		if err := s.resolveSortTarget(ctx, &builtin); err != nil || builtin.Params != nil {
			t.Errorf("a builtin's params = %v (err %v), want none", builtin.Params, err)
		}
		unknown := entity.ViewSortDef{BuiltinField: "stars.unknown"}
		if err := s.resolveSortTarget(ctx, &unknown); !errors.Is(err, ErrSortFieldTypeUnsupported) {
			t.Errorf("an unknown criterion: err %v", err)
		}
	})

	t.Run("a filter compares a number", func(t *testing.T) {
		ok := entity.ViewFilterDef{BuiltinField: "stars.count", Operator: entity.ViewFilterOperatorGte, Value: "3"}
		if err := s.validateFilter(ctx, ok); err != nil {
			t.Fatal(err)
		}
		for name, f := range map[string]entity.ViewFilterDef{
			"not a number":  {BuiltinField: "stars.count", Operator: entity.ViewFilterOperatorGte, Value: "many"},
			"text operator": {BuiltinField: "stars.count", Operator: entity.ViewFilterOperatorContains, Value: "3"},
			"contextual":    {BuiltinField: "stars.count", Operator: entity.ViewFilterOperatorGte, Contextual: true},
			"unknown":       {BuiltinField: "stars.unknown", Operator: entity.ViewFilterOperatorGte, Value: "3"},
		} {
			if err := s.validateFilter(ctx, f); err == nil {
				t.Errorf("%s: no error", name)
			}
		}
	})

	t.Run("a field drops the options it has no use for", func(t *testing.T) {
		f := entity.ViewFieldDef{BuiltinField: "stars.rating", ImageStyle: "thumbnail", TrimLength: 10, LinkTo: entity.ViewFieldLinkFile, Params: map[string]string{"look": "average"}}
		if err := s.resolveViewFieldTarget(ctx, &f); err != nil {
			t.Fatal(err)
		}
		if f.ImageStyle != "" || f.TrimLength != 0 || f.LinkTo != "" || f.Params["look"] != "average" {
			t.Errorf("field = %+v", f)
		}
	})
}

func TestPlanRunModuleCriteria(t *testing.T) {
	useModuleCriteria(t, nil)
	s := &DefaultService{}
	filters := entity.ViewFilterDefs{
		{BuiltinField: entity.ViewFilterBuiltinStatus, Operator: entity.ViewFilterOperatorEq, Value: "1"},
		{BuiltinField: "stars.count", Operator: entity.ViewFilterOperatorGte, Value: "3"},
		{BuiltinField: "votes.count", Operator: entity.ViewFilterOperatorGte, Value: "3"},
	}
	sorts := entity.ViewSortDefs{
		{BuiltinField: "stars.weighted", Direction: "desc", Params: map[string]string{"min": "10"}},
		{BuiltinField: "votes.total", Direction: "desc"},
		{BuiltinField: entity.ViewSortBuiltinCreatedAt, Direction: "desc"},
	}
	plan, err := s.planRun(t.Context(), filters, sorts)
	if err != nil {
		t.Fatal(err)
	}
	// A criterion of a module not registered (or off) is skipped.
	if len(plan.filters) != 1 || len(plan.moduleFilters) != 1 || plan.moduleFilters[0].Filter.Key != "stars.count" {
		t.Errorf("filters %+v, module filters %+v", plan.filters, plan.moduleFilters)
	}
	if len(plan.sorts) != 2 || plan.sorts[0].Module == nil || plan.sorts[0].Params["min"] != "10" || plan.sorts[1].Builtin != entity.ViewSortBuiltinCreatedAt {
		t.Errorf("sorts %+v", plan.sorts)
	}
}

func TestBuildContentListModuleField(t *testing.T) {
	useModuleCriteria(t, nil)
	viewFields := entity.ViewFieldDefs{
		{ID: 1, BuiltinField: entity.ViewFieldBuiltinTitle},
		{ID: 2, BuiltinField: "stars.rating", Params: map[string]string{"look": "average"}},
		// Its module isn't registered: left out.
		{ID: 3, BuiltinField: "votes.total"},
	}
	nodes := entity.Nodes{{ID: 1, Title: "Moby"}, {ID: 2, Title: "Dick"}}
	fields, items, err := BuildContentList(t.Context(), ContentListSources{}, 1, viewFields, nodes, ContentListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields[1].Label != "Rating" || fields[1].MachineName != "stars.rating" {
		t.Fatalf("fields = %+v", fields)
	}
	id := fields[1].ID
	if got, _ := items[0].Values[id].(template.HTML); got != `<span class="stars">average</span>` {
		t.Errorf("node 1's rating = %q", items[0].Values[id])
	}
	if _, ok := items[1].Values[id]; ok {
		t.Errorf("node 2 has no rating, got %q", items[1].Values[id])
	}
}

// TestModuleChoiceFilterValidation - a module's choice filter takes eq,
// in, empty and not empty with any value; a number filter's operators
// (gte) not.
func TestModuleChoiceFilterValidation(t *testing.T) {
	useModuleCriteria(t, nil)
	AddModuleFilter(entity.ViewModuleFilter{Module: "flow", Key: "flow.state", Label: "State",
		Options: func(context.Context) ([]entity.ViewModuleOption, error) { return nil, nil },
		Choices: func(context.Context, []int) (map[int]string, error) { return nil, nil }})
	for op, ok := range map[entity.ViewFilterOperator]bool{
		entity.ViewFilterOperatorEq: true, entity.ViewFilterOperatorIn: true, entity.ViewFilterOperatorEmpty: true, entity.ViewFilterOperatorGte: false,
	} {
		err := validateModuleFilter(entity.ViewFilterDef{BuiltinField: "flow.state", Operator: op, Value: "review,draft"})
		if (err == nil) != ok {
			t.Errorf("%s: %v", op, err)
		}
	}
}
