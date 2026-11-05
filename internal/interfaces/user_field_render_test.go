package interfaces

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
)

// testUsers - two made-up users, one with an avatar.
var testUsers = []entity.User{
	{ID: 4, Username: "anna_k", Slug: "anna_k", Avatar: entity.File{URI: "/files/a.jpg"}},
	{ID: 7, Username: "petrov", Slug: "petrov"},
}

// TestUserFieldOnPages - a user field's people on the content page and in
// a View's row: an avatar (or the placeholder) and the name, a link to the
// profile; never inside a link to the content.
func TestUserFieldOnPages(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	field := entity.FieldInstanceDef{ID: 2, MachineName: "members", Label: "Members", FieldType: entity.FieldTypeUser, ShowOnDetail: true, ShowLabel: true,
		Config: map[string]any{"multiple": true}}
	dc := &entity.DynamicContent{Fields: entity.FieldInstanceDefs{field}, Values: map[int]any{2: []int{4, 7}}}
	sections := buildDisplaySections(dc.Fields, dc, nil, nil)
	ctx := map[string]any{"item": dc, "taxonomyFieldTerms": map[int]entity.TaxonomyTerms{}, "fileFieldLinks": map[int]fileLinkInfo{}, "referenceFieldNodes": map[int]entity.Nodes{},
		"userFieldUsers": map[int][]entity.User{2: testUsers}}
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	if err := ts.RenderPageBlock(&buf, "dynamic/page.html", "node_sections", map[string]any{"Ctx": ctx, "Sections": sections}); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		`<span class="field-list">Members:</span>`,
		`<a href="/users/anna_k" class="field-users-link"><img src="/files/a.jpg" alt="" class="field-users-avatar" loading="lazy"><span>anna_k</span></a>`,
		`<span>petrov</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("page: no %s in %s", want, out)
		}
	}

	listField := entity.FieldInstanceDef{ID: 5000000, FieldType: entity.FieldTypeUser, Label: "Members", MachineName: "members", Config: map[string]any{"link_to": "content"}}
	item := views.ContentListItem{Node: entity.Node{ID: 9, Title: "Project", Slug: "project"}, Fields: entity.FieldInstanceDefs{listField},
		Values: map[int]any{5000000: []int{4}}, UserFieldUsers: map[int][]entity.User{5000000: testUsers[:1]}}
	var list bytes.Buffer
	data := map[string]any{"isViewList": true, "listFieldsHead": item.Fields, "displayStyle": "grid", "gridColumns": 3, "items": []views.ContentListItem{item}}
	if err := service.NewTemplateService(false).RenderNamed(&list, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatal(err)
	}
	if got := list.String(); !strings.Contains(got, `href="/users/anna_k" class="field-users-link"`) || strings.Contains(got, `class="item-field-link"`) {
		t.Errorf("list: %s", got)
	}
}

// TestUserFieldInForm - the content form's user field: the picker and
// the hidden input with the chosen users' IDs in order.
func TestUserFieldInForm(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	field := entity.FieldInstanceDef{ID: 3, Label: "Members", FieldType: entity.FieldTypeUser,
		Config: map[string]any{"multiple": true}, WidgetConfig: map[string]any{"max_values": float64(5)}}
	data := map[string]any{
		"title": "Editing: Project", "contentType": &entity.ContentTypeDef{MachineName: "project", Label: "Project"},
		"dc": &entity.DynamicContent{}, "node": &entity.Node{}, "form": form.DynamicForm{Title: "Project"}, "timeZone": "UTC",
		"formSections": buildFormSections(entity.FieldInstanceDefs{field}), "values": map[int]any{3: []int{7, 4}},
		"userFieldUsers": map[int][]entity.User{3: {testUsers[1], testUsers[0]}},
	}
	for _, k := range []string{"taxonomyOptions", "taxonomyHierarchyLabels", "multiTaxonomyFieldTerms", "referenceFieldNodes",
		"imageFieldURLs", "imageFieldAlts", "imageFieldItems", "fileFieldLinks", "textFormatIDs", "tagPickers", "blockFields"} {
		data[k] = map[int]any{}
	}
	out := renderNodeEdit(t, data)
	for _, want := range []string{
		`id="field_3-display" class="form-control" placeholder="Start typing a name…"`,
		`<input type="hidden" id="field_3" name="field_3" value="7,4">`,
		`At most 5 people.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}
