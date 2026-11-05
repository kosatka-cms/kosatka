package admin

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// TestFiltersNoStatusWarning - a content View without a status filter
// says it lists unpublished content too; one with it, and a comment
// View, don't.
func TestFiltersNoStatusWarning(t *testing.T) {
	templates := service.NewTemplateService(false)
	render := func(view *entity.ViewDisplayDef, filters entity.ViewFilterDefs) string {
		t.Helper()
		var b strings.Builder
		if err := templates.RenderNamed(&b, "view_filters_list", map[string]any{"view": view, "filters": filters, "filterValueDisplays": map[int]string{}, "fieldLabels": map[int]string{}}, viewFragmentFiles...); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	content := &entity.ViewDisplayDef{ID: 1}
	if out := render(content, entity.ViewFilterDefs{{ID: 2, BuiltinField: entity.ViewFilterBuiltinPromote, Operator: "eq", Value: "1"}}); !strings.Contains(out, `id="filters-no-status"`) {
		t.Error("no warning without a status filter")
	}
	if out := render(content, entity.ViewFilterDefs{{ID: 3, BuiltinField: entity.ViewFilterBuiltinStatus, Operator: "eq", Value: "1"}}); strings.Contains(out, `id="filters-no-status"`) {
		t.Error("a warning with a status filter")
	}
	if out := render(&entity.ViewDisplayDef{ID: 4, BaseEntity: entity.ViewBaseEntityComment}, nil); strings.Contains(out, `id="filters-no-status"`) {
		t.Error("a warning on a comment View")
	}
}
