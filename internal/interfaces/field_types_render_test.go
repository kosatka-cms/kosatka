package interfaces

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
)

// The completeness tests of the content form and page: every field type
// but group has an input, gives a value from the form and shows on the
// page (a reference - by Views there). A new type fails here until the
// form and the page handle it.

// fieldTypeForm - a submitted form holding a value of a field of type ft
// (its ID 1), as the resolvers leave it for dynamicValuesFromForm.
func fieldTypeForm(ft entity.ContentTypeFieldType) (form.DynamicForm, bool) {
	f := form.DynamicForm{FieldValues: map[int]string{}}
	switch ft {
	case entity.FieldTypeText, entity.FieldTypeList, entity.FieldTypeEmail, entity.FieldTypePhone:
		f.FieldValues[1] = "a"
	case entity.FieldTypeNumber:
		f.FieldValues[1] = "2.5"
	case entity.FieldTypeDate:
		f.FieldValues[1] = "2026-10-27"
	case entity.FieldTypeBool:
		f.FieldValues[1] = "1"
	case entity.FieldTypeTaxonomy, entity.FieldTypeReference, entity.FieldTypeImage, entity.FieldTypeFile, entity.FieldTypeUser:
		f.FieldValues[1] = "3"
	case entity.FieldTypeLink:
		f.FieldLinks = map[int][]entity.Link{1: {{URL: "https://example.com"}}}
	case entity.FieldTypeBlocks:
		f.FieldBlocks = map[int][]form.BlockInput{1: {{Key: "n1", Item: entity.BlockItem{Type: "role"}}}}
	default:
		return f, false
	}
	return f, true
}

// TestFieldTypesInForm - every field type is in formFieldTypes, and every
// one but group has its input in the content form (field_input_<type>,
// id="field_<id>") and a value from the submitted form.
func TestFieldTypesInForm(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	for _, ft := range entity.FieldTypes {
		if _, ok := formFieldTypes[ft]; !ok {
			t.Errorf("%s: not in formFieldTypes - decide how the form takes it", ft)
		}
		if ft == entity.FieldTypeGroup {
			continue
		}
		field := entity.FieldInstanceDef{ID: 1, Label: "X", FieldType: ft}
		f, ok := fieldTypeForm(ft)
		if !ok {
			t.Errorf("%s: no submitted form - add one to fieldTypeForm", ft)
			continue
		}
		if values, _ := dynamicValuesFromForm(&f, entity.FieldInstanceDefs{field}); values[1] == nil {
			t.Errorf("%s: no value from the form", ft)
		}
		data := map[string]any{
			"title": "Editing", "contentType": &entity.ContentTypeDef{MachineName: "t", Label: "T"},
			"dc": &entity.DynamicContent{}, "node": &entity.Node{}, "form": form.DynamicForm{Title: "T"}, "timeZone": "UTC",
			"formSections": buildFormSections(entity.FieldInstanceDefs{field}),
		}
		for k, v := range blockInputs(map[int]any{}) {
			data[k] = v
		}
		if out := renderNodeEdit(t, data); !strings.Contains(out, `id="field_1"`) {
			t.Errorf("%s: no input id=\"field_1\" in the form", ft)
		}
	}
}

// TestFieldTypesOnPage - a value of every field type but group and
// reference (Views show a node page's references; a block's - its page)
// shows on the content page, with the field's label.
func TestFieldTypesOnPage(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	values := map[entity.ContentTypeFieldType]any{
		entity.FieldTypeText: "a", entity.FieldTypeNumber: 2.5, entity.FieldTypeDate: time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC),
		entity.FieldTypeBool: true, entity.FieldTypeTaxonomy: 3, entity.FieldTypeImage: 4, entity.FieldTypeFile: 5,
		entity.FieldTypeList: "a", entity.FieldTypeLink: []entity.Link{{URL: "https://example.com"}}, entity.FieldTypeUser: []int{6},
		entity.FieldTypeEmail: "info@example.com", entity.FieldTypePhone: "+7 495 123-45-67",
		entity.FieldTypeBlocks: []entity.BlockItem{{ID: 7, Type: "role"}},
	}
	for _, ft := range entity.FieldTypes {
		if _, ok := pageFieldTypes[ft]; !ok {
			t.Errorf("%s: not in pageFieldTypes - decide how the page shows it", ft)
		}
		if ft == entity.FieldTypeGroup || ft == entity.FieldTypeReference {
			continue
		}
		v, ok := values[ft]
		if !ok {
			t.Errorf("%s: no value - add one to the test", ft)
			continue
		}
		field := entity.FieldInstanceDef{ID: 1, MachineName: "x", Label: "Sample label", FieldType: ft, ShowOnDetail: true, ShowLabel: true}
		dc := &entity.DynamicContent{Fields: entity.FieldInstanceDefs{field}, Values: map[int]any{1: v}}
		terms := map[int]entity.TaxonomyTerms{1: {{ID: 3, Name: "Term"}}}
		images := map[int]string{1: "/a.webp"}
		sections := buildDisplaySections(dc.Fields, dc, terms, images)
		if len(sections) == 0 {
			t.Errorf("%s: no section on the page", ft)
			continue
		}
		ctx := map[string]any{
			"item": dc, "taxonomyFieldTerms": terms, "imageFieldURLs": images, "imageFieldAlts": map[int]string{},
			"imageFieldItems": map[int][]imageItem{}, "fileFieldLinks": map[int]fileLinkInfo{1: {URI: "/f.pdf", Name: "f.pdf"}},
			"userFieldUsers": map[int][]entity.User{1: {{ID: 6, Username: "anna_k", Slug: "anna_k"}}}, "referenceFieldNodes": map[int]entity.Nodes{},
			"blockViews": map[int][]blockView{1: {{ID: 7, Type: "role", Ctx: map[string]any{}}}},
		}
		var buf strings.Builder
		ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
		if err := ts.RenderPageBlock(&buf, "dynamic/page.html", "node_sections", map[string]any{"Ctx": ctx, "Sections": sections, "Block": true}); err != nil {
			t.Errorf("%s: %v", ft, err)
			continue
		}
		if !strings.Contains(buf.String(), "Sample label") {
			t.Errorf("%s: not shown on the page:\n%s", ft, buf.String())
		}
	}
}

// TestSeveralFilesLine - a file field with several files: on the page, a
// list of download links, each with its name and size, in order; with
// one, the one line.
func TestSeveralFilesLine(t *testing.T) {
	field := entity.FieldInstanceDef{ID: 1, MachineName: "field_docs", Label: "Documents", FieldType: entity.FieldTypeFile, Config: map[string]any{"multiple": true}, ShowOnDetail: true, ShowLabel: true}
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	render := func(ids []int, items []fileLinkInfo) string {
		t.Helper()
		dc := &entity.DynamicContent{Fields: entity.FieldInstanceDefs{field}, Values: map[int]any{1: ids}}
		ctx := map[string]any{
			"item": dc, "taxonomyFieldTerms": map[int]entity.TaxonomyTerms{}, "imageFieldURLs": map[int]string{}, "imageFieldAlts": map[int]string{},
			"imageFieldItems": map[int][]imageItem{}, "fileFieldLinks": map[int]fileLinkInfo{1: items[0]}, "fileFieldItems": map[int][]fileLinkInfo{1: items},
			"userFieldUsers": map[int][]entity.User{}, "referenceFieldNodes": map[int]entity.Nodes{},
		}
		var buf strings.Builder
		if err := ts.RenderPageBlock(&buf, "dynamic/page.html", "node_sections", map[string]any{"Ctx": ctx, "Sections": buildDisplaySections(dc.Fields, dc, nil, nil), "Block": true}); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	two := render([]int{4, 5}, []fileLinkInfo{{FileID: 4, URI: "/files/prices.pdf", Name: "prices.pdf", SizeBytes: 2048}, {FileID: 5, URI: "/files/terms.pdf", Name: "terms.pdf", SizeBytes: 1024}})
	prices, terms := strings.Index(two, ">prices.pdf</a>"), strings.Index(two, ">terms.pdf</a>")
	if !strings.Contains(two, `<ul class="field-files">`) || prices < 0 || terms < prices {
		t.Errorf("two files: a list in order wanted:\n%s", two)
	}
	one := render([]int{4}, []fileLinkInfo{{FileID: 4, URI: "/files/prices.pdf", Name: "prices.pdf", SizeBytes: 2048}})
	if strings.Contains(one, "field-files") || !strings.Contains(one, ">prices.pdf</a>") {
		t.Errorf("one file: its line wanted:\n%s", one)
	}
}
