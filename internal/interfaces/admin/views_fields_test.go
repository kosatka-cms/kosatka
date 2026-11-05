package admin

import (
	"context"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

// TestViewFieldsListShowsMachineNames - the fields list's "Field" column
// names each field and shows its machine name (a storage field's, or a
// builtin's key).
func TestViewFieldsListShowsMachineNames(t *testing.T) {
	fsID := 173
	data := map[string]any{
		"view": &entity.ViewDisplayDef{ID: 1},
		"fields": entity.ViewFieldDefs{
			{ID: 1, FieldStorageID: &fsID, Label: "Section"},
			{ID: 2, BuiltinField: entity.ViewFieldBuiltinTitle, Label: "Title"},
		},
		"fieldStorageLabels":       map[int]string{173: "News type"},
		"fieldStorageMachineNames": map[int]string{173: "type_tag"},
		"fieldStorageTypes":        map[int]entity.ContentTypeFieldType{173: entity.FieldTypeTaxonomy},
		"fieldBuiltinLabels":       views.FieldBuiltinLabels(context.Background()),
	}
	var buf strings.Builder
	if err := service.NewTemplateService(false).RenderNamed(&buf, "view_fields_list", data, viewFragmentFiles...); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`News type <code class="small text-muted">type_tag</code>`, `<code class="small text-muted">title</code>`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
}
