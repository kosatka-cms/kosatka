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

// renderNodeEdit - the content of the content form (node/edit_dynamic.html),
// from the repository's root.
func renderNodeEdit(t *testing.T, data map[string]any) string {
	t.Helper()
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	if err := ts.RenderPageBlock(&buf, "node/edit_dynamic.html", "content", data); err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// TestNodeEditLayout - the content form in two columns: the title and the
// ungrouped fields on one card, a form group on a card of its own (in
// the type's order, the fields after it on a new card); the side column
// with the state (a link to the history when the type keeps it) and the
// save button, then, for who may edit them, the publishing card and the
// settings as lines with their values. Without that right only the save
// card.
func TestNodeEditLayout(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	group := 3
	fields := entity.FieldInstanceDefs{
		{ID: 1, Label: "Body", MachineName: "body", FieldType: entity.FieldTypeText, SortOrder: 1},
		{ID: 3, Label: "Extra", MachineName: "extra", FieldType: entity.FieldTypeGroup, SortOrder: 2},
		{ID: 4, Label: "Source", MachineName: "source", FieldType: entity.FieldTypeText, SortOrder: 3, FormGroupID: &group},
		{ID: 5, Label: "Note", MachineName: "note", FieldType: entity.FieldTypeText, SortOrder: 4},
	}
	node := entity.Node{ID: 9, Type: "article", Slug: "article/moby", UpdatedAt: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)}
	data := map[string]any{
		"title":               "Editing: Moby",
		"contentType":         &entity.ContentTypeDef{MachineName: "article", Label: "Article", Revisions: true},
		"dc":                  &entity.DynamicContent{Node: node},
		"node":                &node,
		"form":                form.DynamicForm{NodeID: 9, Title: "Moby", Slug: "article/moby", Status: 1},
		"formSections":        buildFormSections(fields),
		"values":              map[int]any{},
		"canEditNodeSettings": true,
	}
	for _, k := range []string{"taxonomyOptions", "taxonomyHierarchyLabels", "multiTaxonomyFieldTerms", "referenceFieldNodes",
		"imageFieldURLs", "imageFieldAlts", "imageFieldItems", "fileFieldLinks", "textFormatIDs", "userFieldUsers", "tagPickers", "blockFields"} {
		data[k] = map[int]any{}
	}
	out := renderNodeEdit(t, data)
	for _, want := range []string{
		`<h1 class="node-edit-title">Editing: Moby</h1>`,
		`class="node-form node-edit-layout"`,
		`<details class="node-edit-card node-edit-group"> <summary class="node-edit-card-title">Extra</summary>`,
		`<span class="node-edit-pill node-edit-pill--published">Published</span>`,
		`<a href="/node/9/revisions">History</a>`,
		`<button type="submit" class="btn btn-primary">Save</button>`,
		`<h2 class="node-edit-card-title" id="node-edit-publishing-title">Publishing</h2>`,
		`<span class="node-settings-value">/article/moby</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the form has no %s:\n%s", want, out)
		}
	}
	body, group1, note := strings.Index(out, `name="field_1"`), strings.Index(out, "node-edit-group"), strings.Index(out, `name="field_5"`)
	if body < 0 || group1 < body || note < group1 || !strings.Contains(out[group1:note], `</details> <div class="node-edit-card">`) {
		t.Errorf("the fields out of order or the field after the group not on a new card:\n%s", out)
	}

	if strings.Contains(out, "node-edit-preview") {
		t.Error("a Preview button without formPreviewURL")
	}

	data["canEditNodeSettings"] = false
	data["formPreviewURL"] = FormPreviewPath("article", 9)
	out = renderNodeEdit(t, data)
	if strings.Contains(out, "node-settings") || strings.Contains(out, "node-edit-publishing") || !strings.Contains(out, "node-edit-save") {
		t.Errorf("without the right to the settings: only the save card:\n%s", out)
	}
	if !strings.Contains(out, `class="btn btn-outline-secondary node-edit-preview" formaction="/node/9/edit/preview" formtarget="_blank" formnovalidate>Preview`) {
		t.Errorf("no Preview button sending the form to its preview in a new tab:\n%s", out)
	}
}

// TestFormPreviewPath - a new item's form previews at its type's create
// address, an item's at its edit address.
func TestFormPreviewPath(t *testing.T) {
	if got := FormPreviewPath("article", 0); got != "/create/article/preview" {
		t.Errorf("new: %s", got)
	}
	if got := FormPreviewPath("article", 9); got != "/node/9/edit/preview" {
		t.Errorf("edit: %s", got)
	}
}
