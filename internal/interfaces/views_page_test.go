package interfaces

import (
	"context"
	"net/url"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

func TestBuildExposedFilterFields(t *testing.T) {
	fieldStorages := map[int]entity.FieldStorageDef{
		1: {ID: 1, MachineName: "title_alt", FieldType: entity.FieldTypeText},
		2: {ID: 2, MachineName: "field_publisher", FieldType: entity.FieldTypeTaxonomy},
	}
	fieldStorageLabels := map[int]string{1: "Title (alt.)", 2: "Publisher"}
	taxonomyOptions := map[int][]exposedFilterOption{
		2: {{Value: "22", Label: "Alpha"}, {Value: "23", Label: "Beta"}},
	}

	t.Run("text contains becomes a text field, falling back to the field's own label", func(t *testing.T) {
		filters := entity.ViewFilterDefs{
			{ID: 1, FieldStorageID: intPtr(1), Operator: entity.ViewFilterOperatorContains, Exposed: true, ExposedParam: "q"},
		}
		got := buildExposedFilterFields(context.Background(), filters, fieldStorages, fieldStorageLabels, taxonomyOptions, map[string]string{"q": "human"})
		if len(got) != 1 {
			t.Fatalf("got %d fields, want 1", len(got))
		}
		if got[0].Kind != exposedFilterKindText || got[0].InputType != "search" {
			t.Fatalf("got %+v, want a search text field", got[0])
		}
		if got[0].Label != "Title (alt.)" || got[0].Param != "q" || got[0].Value != "human" {
			t.Fatalf("got %+v, unexpected", got[0])
		}
	})

	t.Run("ExposedLabel overrides the field's own label", func(t *testing.T) {
		filters := entity.ViewFilterDefs{
			{ID: 1, FieldStorageID: intPtr(1), Operator: entity.ViewFilterOperatorContains, Exposed: true, ExposedParam: "q", ExposedLabel: "Name"},
		}
		got := buildExposedFilterFields(context.Background(), filters, fieldStorages, fieldStorageLabels, taxonomyOptions, nil)
		if len(got) != 1 || got[0].Label != "Name" {
			t.Fatalf("got %+v, want label %q", got, "Name")
		}
	})

	t.Run("taxonomy eq becomes a select populated from taxonomyOptions", func(t *testing.T) {
		filters := entity.ViewFilterDefs{
			{ID: 2, FieldStorageID: intPtr(2), Operator: entity.ViewFilterOperatorEq, Exposed: true, ExposedParam: "publisher"},
		}
		got := buildExposedFilterFields(context.Background(), filters, fieldStorages, fieldStorageLabels, taxonomyOptions, map[string]string{"publisher": "22"})
		if len(got) != 1 {
			t.Fatalf("got %d fields, want 1", len(got))
		}
		if got[0].Kind != exposedFilterKindSelect || len(got[0].Options) != 2 {
			t.Fatalf("got %+v, want a select with 2 options", got[0])
		}
		if got[0].Value != "22" {
			t.Fatalf("got value %q, want %q (round-tripped from queryParams)", got[0].Value, "22")
		}
	})

	t.Run("title builtin falls back to Title when ExposedLabel is empty", func(t *testing.T) {
		filters := entity.ViewFilterDefs{
			{ID: 3, BuiltinField: entity.ViewFilterBuiltinTitle, Operator: entity.ViewFilterOperatorContains, Exposed: true, ExposedParam: "title"},
		}
		got := buildExposedFilterFields(context.Background(), filters, fieldStorages, fieldStorageLabels, taxonomyOptions, nil)
		if len(got) != 1 || got[0].Label != "Title" {
			t.Fatalf("got %+v, want label %q", got, "Title")
		}
	})

	t.Run("taxonomy in/not_in become checkboxes with the current values ticked", func(t *testing.T) {
		filters := entity.ViewFilterDefs{
			{ID: 4, FieldStorageID: intPtr(2), Operator: entity.ViewFilterOperatorIn, Exposed: true, ExposedParam: "publishers"},
			{ID: 5, FieldStorageID: intPtr(2), Operator: entity.ViewFilterOperatorNotIn, Exposed: true, ExposedParam: "not_publishers"},
		}
		got := buildExposedFilterFields(context.Background(), filters, fieldStorages, fieldStorageLabels, taxonomyOptions, map[string]string{"publishers": "22,23"})
		if len(got) != 2 || got[0].Kind != exposedFilterKindCheckboxes || got[1].Kind != exposedFilterKindCheckboxes || len(got[0].Options) != 2 {
			t.Fatalf("got %+v, want two checkbox fields", got)
		}
		if !got[0].Checked["22"] || !got[0].Checked["23"] || len(got[1].Checked) != 0 {
			t.Fatalf("checked: %v %v", got[0].Checked, got[1].Checked)
		}
	})

	t.Run("presence operators are skipped, not fixed operators or unexposed filters", func(t *testing.T) {
		filters := entity.ViewFilterDefs{
			{ID: 6, FieldStorageID: intPtr(1), Operator: entity.ViewFilterOperatorEmpty, Exposed: true, ExposedParam: "empty_q"},
			{ID: 7, FieldStorageID: intPtr(1), Operator: entity.ViewFilterOperatorNotEmpty, Exposed: true, ExposedParam: "not_empty_q"},
			{ID: 8, FieldStorageID: intPtr(1), Operator: entity.ViewFilterOperatorContains, Exposed: false, Value: "fixed"},
			{ID: 9, BuiltinField: entity.ViewFilterBuiltinStatus, Operator: entity.ViewFilterOperatorEq, Exposed: true, ExposedParam: "status"},
		}
		got := buildExposedFilterFields(context.Background(), filters, fieldStorages, fieldStorageLabels, taxonomyOptions, nil)
		if len(got) != 0 {
			t.Fatalf("got %d fields, want 0: %+v", len(got), got)
		}
	})

	t.Run("a module's choice filter becomes a select of its options; its presence operators are skipped", func(t *testing.T) {
		views.ResetModuleCriteria()
		t.Cleanup(views.ResetModuleCriteria)
		views.AddModuleFilter(entity.ViewModuleFilter{Module: "stage", Key: "stage.name", Label: "Stage",
			Options: func(context.Context) ([]entity.ViewModuleOption, error) {
				return []entity.ViewModuleOption{{Value: "draft", Label: "Draft"}, {Value: "done", Label: "Done"}}, nil
			},
			Choices: func(context.Context, []int) (map[int]string, error) { return nil, nil }})
		filters := entity.ViewFilterDefs{
			{ID: 10, BuiltinField: "stage.name", Operator: entity.ViewFilterOperatorEq, Exposed: true, ExposedParam: "stage"},
			{ID: 11, BuiltinField: "stage.name", Operator: entity.ViewFilterOperatorEmpty, Exposed: true, ExposedParam: "no_stage"},
		}
		got := buildExposedFilterFields(context.Background(), filters, fieldStorages, fieldStorageLabels, taxonomyOptions, map[string]string{"stage": "done"})
		if len(got) != 1 || got[0].Kind != exposedFilterKindSelect || got[0].Label != "Stage" || got[0].Value != "done" {
			t.Fatalf("got %+v, want the Stage select", got)
		}
		if len(got[0].Options) != 2 || got[0].Options[1] != (exposedFilterOption{Value: "done", Label: "Done"}) {
			t.Fatalf("options: %+v", got[0].Options)
		}
	})

	t.Run("a list field's eq becomes a select of its options, its in checkboxes", func(t *testing.T) {
		storages := map[int]entity.FieldStorageDef{9: {ID: 9, MachineName: "field_verdict", FieldType: entity.FieldTypeList, Config: map[string]any{"allowed_values": []any{
			map[string]any{"value": "great", "label": "Great"}, map[string]any{"value": "miss", "label": "Miss"},
		}}}}
		filters := entity.ViewFilterDefs{
			{ID: 12, FieldStorageID: intPtr(9), Operator: entity.ViewFilterOperatorEq, Exposed: true, ExposedParam: "verdict"},
			{ID: 13, FieldStorageID: intPtr(9), Operator: entity.ViewFilterOperatorIn, Exposed: true, ExposedParam: "verdicts"},
		}
		got := buildExposedFilterFields(context.Background(), filters, storages, map[int]string{9: "Verdict"}, nil, map[string]string{"verdict": "miss", "verdicts": "great"})
		if len(got) != 2 || got[0].Kind != exposedFilterKindSelect || got[0].Label != "Verdict" || got[0].Value != "miss" {
			t.Fatalf("got %+v, want the verdict select", got)
		}
		if got[1].Kind != exposedFilterKindCheckboxes || !got[1].Checked["great"] || got[1].Checked["miss"] {
			t.Fatalf("got %+v, want the verdicts checkboxes", got[1])
		}
		if len(got[0].Options) != 2 || got[0].Options[1] != (exposedFilterOption{Value: "miss", Label: "Miss"}) {
			t.Fatalf("options: %+v", got[0].Options)
		}
	})

	t.Run("output order follows input order (caller sorts via SortBySortOrder)", func(t *testing.T) {
		filters := entity.ViewFilterDefs{
			{ID: 2, FieldStorageID: intPtr(2), Operator: entity.ViewFilterOperatorEq, Exposed: true, ExposedParam: "publisher", SortOrder: 20},
			{ID: 1, FieldStorageID: intPtr(1), Operator: entity.ViewFilterOperatorContains, Exposed: true, ExposedParam: "q", SortOrder: 10},
		}
		got := buildExposedFilterFields(context.Background(), filters.SortBySortOrder(), fieldStorages, fieldStorageLabels, taxonomyOptions, nil)
		if len(got) != 2 || got[0].Param != "q" || got[1].Param != "publisher" {
			t.Fatalf("got %+v, want [q, publisher] in that order", got)
		}
	})
}

// TestQueryFilterValues - a parameter's value; repeated (ticked
// checkboxes) - its non-empty values comma-joined; empty - kept empty.
func TestQueryFilterValues(t *testing.T) {
	q, _ := url.ParseQuery("type=3&type=5&type=&q=tank&empty=&page=2")
	got := queryFilterValues(q)
	want := map[string]string{"type": "3,5", "q": "tank", "empty": "", "page": "2"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, want %q", k, got[k], v)
		}
	}
}

// TestExposedSortField - the exposed criteria as options in their
// order, the chosen one selected (unknown or none - the first); none
// exposed - no field.
func TestExposedSortField(t *testing.T) {
	ctx := context.Background()
	sorts := entity.ViewSortDefs{
		{ID: 1, BuiltinField: entity.ViewSortBuiltinSticky},
		{ID: 2, BuiltinField: entity.ViewSortBuiltinCreatedAt, ExposedKey: "new", ExposedLabel: "Newest"},
		{ID: 3, BuiltinField: entity.ViewSortBuiltinTitle, ExposedKey: "title", ExposedLabel: "By title"},
	}
	for chosen, want := range map[string]string{"": "new", "title": "title", "nonsense": "new"} {
		field, ok := exposedSortField(ctx, sorts, chosen)
		if !ok || field.Kind != exposedFilterKindSort || field.Param != entity.ViewSortParam || field.Value != want {
			t.Errorf("chosen %q: got %+v, %v; want the value %q", chosen, field, ok, want)
		}
		if len(field.Options) != 2 || field.Options[0] != (exposedFilterOption{Value: "new", Label: "Newest"}) || field.Options[1].Value != "title" {
			t.Errorf("chosen %q: options %+v", chosen, field.Options)
		}
	}
	if _, ok := exposedSortField(ctx, sorts[:1], ""); ok {
		t.Error("a field with no criterion exposed")
	}
}
