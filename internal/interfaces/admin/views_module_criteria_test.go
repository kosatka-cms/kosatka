package admin

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

// useStarsCriteria registers a "stars" module's sort criterion (with a
// threshold), filter and field for the test.
func useStarsCriteria(t *testing.T) {
	t.Helper()
	views.ResetModuleCriteria()
	t.Cleanup(views.ResetModuleCriteria)
	threshold := entity.ViewModuleParam{Key: "min", Label: "Threshold", Type: entity.PluginBlockSettingNumber, Default: "5", Min: 1, Max: 100}
	views.AddModuleSort(entity.ViewModuleSort{Module: "stars", Key: "stars.weighted", Label: "Rating (weighted)", Params: []entity.ViewModuleParam{threshold},
		Values: func(context.Context, []int, map[string]string) (map[int]float64, error) { return nil, nil }})
	views.AddModuleFilter(entity.ViewModuleFilter{Module: "stars", Key: "stars.count", Label: "Rating: votes",
		Values: func(context.Context, []int) (map[int]float64, error) { return nil, nil }})
	views.AddModuleFilter(entity.ViewModuleFilter{Module: "stars", Key: "stars.tier", Label: "Rating: tier",
		Options: func(context.Context) ([]entity.ViewModuleOption, error) {
			return []entity.ViewModuleOption{{Value: "gold", Label: "Gold"}}, nil
		},
		Choices: func(context.Context, []int) (map[int]string, error) { return nil, nil }})
	views.AddModuleField(entity.ViewModuleField{Module: "stars", Key: "stars.rating", Label: "Rating",
		Render: func(context.Context, []int, map[string]string) (map[int]template.HTML, error) { return nil, nil }})
}

// TestViewModuleCriteriaAdmin - a content View's page offers the modules'
// criteria with their settings (a group per criterion, sent only when
// chosen; a choice filter - its operators and values) and names them in
// its lists; a comment View offers none.
func TestViewModuleCriteriaAdmin(t *testing.T) {
	useStarsCriteria(t)
	ctx := t.Context()
	view := &entity.ViewDisplayDef{ID: 1}
	data := map[string]any{
		"view":              view,
		"sorts":             entity.ViewSortDefs{{ID: 7, BuiltinField: "stars.weighted", Direction: "desc", Params: map[string]string{"min": "20"}}},
		"sortBuiltinLabels": sortBuiltinLabels,
	}
	addViewModuleCriteria(ctx, data, view)
	if len(data["moduleSorts"].([]viewModuleOption)) != 1 || len(data["moduleFilters"].([]viewModuleOption)) != 2 || len(data["moduleFields"].([]viewModuleOption)) != 1 {
		t.Fatalf("module criteria offered: %+v", data)
	}
	filters := data["moduleFilters"].([]viewModuleOption)
	if filters[0].Options != "" || filters[1].Operators != "eq in empty not_empty" || filters[1].Options != `[{"value":"gold","label":"Gold"}]` {
		t.Errorf("filters offered: %+v", filters)
	}
	templates := service.NewTemplateService(false)
	var list strings.Builder
	if err := templates.RenderNamed(&list, "view_sorts_list", data, viewFragmentFiles...); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-target-label="Rating (weighted)"`, `data-params="{&#34;min&#34;:&#34;20&#34;}"`, `min=20`} {
		if !strings.Contains(list.String(), want) {
			t.Errorf("sorts list: missing %q in %s", want, list.String())
		}
	}
	var params strings.Builder
	if err := templates.RenderNamed(&params, "view_module_params", map[string]any{"options": data["moduleSorts"], "prefix": "sort-param"}, "admin/views/fragments.html"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-module-target="builtin:stars.weighted"`, `name="param_min"`, `value="5"`, `disabled`} {
		if !strings.Contains(params.String(), want) {
			t.Errorf("params: missing %q in %s", want, params.String())
		}
	}

	comments := map[string]any{}
	addViewModuleCriteria(ctx, comments, &entity.ViewDisplayDef{BaseEntity: entity.ViewBaseEntityComment})
	if len(comments["moduleSorts"].([]viewModuleOption)) != 0 {
		t.Error("a comment View is offered a module's criteria")
	}
}

func TestModuleParamsFromForm(t *testing.T) {
	params := []entity.ViewModuleParam{{Key: "min", Type: entity.PluginBlockSettingNumber, Min: 1, Max: 100}}
	for value, ok := range map[string]bool{"20": true, "0": false, "many": false} {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{"param_min": {value}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		got, accepted := moduleParamsFromForm(r, params)
		if accepted != ok || (ok && got["min"] != value) {
			t.Errorf("param_min=%q: %v, %v", value, got, accepted)
		}
	}
}
