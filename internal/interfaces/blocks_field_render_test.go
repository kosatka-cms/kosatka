package interfaces

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
)

// blockInputs - a block's (or the page's) fieldInputData keys, empty.
func blockInputs(values map[int]any) map[string]any {
	in := map[string]any{"values": values}
	for _, k := range []string{"taxonomyOptions", "taxonomyHierarchyLabels", "multiTaxonomyFieldTerms", "referenceFieldNodes",
		"imageFieldURLs", "imageFieldAlts", "imageFieldItems", "fileFieldLinks", "textFormatIDs", "userFieldUsers", "tagPickers", "blockFields"} {
		in[k] = map[int]any{}
	}
	in["fileFieldLinks"] = map[int]fileLinkInfo{}
	in["blockFields"] = map[int]blocksFieldForm{}
	return in
}

// TestBlocksFieldInForm - the content form's blocks field: a card per
// block (its set, its summary, ↑/↓/✕), its key in the order input, its
// set and ID, its fields' inputs after its prefix; a template of a new
// block per set, its key __KEY__; "Add: <set>".
func TestBlocksFieldInForm(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	character := entity.FieldInstanceDef{ID: 21, Label: "Character", FieldType: entity.FieldTypeText}
	cast := entity.FieldInstanceDef{ID: 5, Label: "Cast", FieldType: entity.FieldTypeBlocks, Config: map[string]any{"block_types": []any{"role"}}}
	set := entity.ContentTypeDef{MachineName: "role", Label: "Role", Embedded: true}
	data := map[string]any{
		"title": "Editing: Film", "contentType": &entity.ContentTypeDef{MachineName: "film", Label: "Film"},
		"dc": &entity.DynamicContent{}, "node": &entity.Node{}, "form": form.DynamicForm{Title: "Film"}, "timeZone": "UTC",
		"formSections": buildFormSections(entity.FieldInstanceDefs{cast}),
	}
	for k, v := range blockInputs(map[int]any{}) {
		data[k] = v
	}
	data["blockFields"] = map[int]blocksFieldForm{5: {
		Items: []blockItemForm{{Key: "e9", ID: 9, Set: set, Fields: entity.FieldInstanceDefs{character}, Summary: "Stalker", Inputs: blockInputs(map[int]any{21: "Stalker"})}},
		New:   []blockItemForm{{Key: newBlockKey, Set: set, Fields: entity.FieldInstanceDefs{character}, Inputs: blockInputs(map[int]any{})}},
	}}
	out := renderNodeEdit(t, data)
	for _, want := range []string{
		`<details class="field-block" data-field-block>`,
		`<span class="field-block-set">Role</span> <span class="field-block-summary" data-field-block-summary>Stalker</span>`,
		`<input type="hidden" name="field_5_blocks" value="e9">`,
		`<input type="hidden" name="field_5_e9_type" value="role">`,
		`<input type="hidden" name="field_5_e9_id" value="9">`,
		`id="field_5_e9_field_21" name="field_5_e9_field_21" value="Stalker"`,
		`<template data-field-block-template="role">`,
		`name="field_5___KEY___field_21"`,
		`data-field-block-add="role">+ Add: Role</button>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
}

// TestBlocksFieldOfSeveralSets - a blocks field of several sets: "Add:"
// and a button per set, a template per set.
func TestBlocksFieldOfSeveralSets(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	text := entity.FieldInstanceDef{ID: 21, Label: "Text", FieldType: entity.FieldTypeText}
	content := entity.FieldInstanceDef{ID: 5, Label: "Content", FieldType: entity.FieldTypeBlocks, Config: map[string]any{"block_types": []any{"text_block", "quote"}}}
	data := map[string]any{
		"title": "Editing: Page", "contentType": &entity.ContentTypeDef{MachineName: "page", Label: "Page"},
		"dc": &entity.DynamicContent{}, "node": &entity.Node{}, "form": form.DynamicForm{Title: "Page"}, "timeZone": "UTC",
		"formSections": buildFormSections(entity.FieldInstanceDefs{content}),
	}
	for k, v := range blockInputs(map[int]any{}) {
		data[k] = v
	}
	block := func(name, label string) blockItemForm {
		return blockItemForm{Key: newBlockKey, Set: entity.ContentTypeDef{MachineName: name, Label: label, Embedded: true}, Fields: entity.FieldInstanceDefs{text}, Inputs: blockInputs(map[int]any{})}
	}
	data["blockFields"] = map[int]blocksFieldForm{5: {New: []blockItemForm{block("text_block", "Text"), block("quote", "Quote")}}}
	out := renderNodeEdit(t, data)
	for _, want := range []string{
		`<span class="field-blocks-add-label">+ Add:</span>`,
		`data-field-block-add="text_block">Text</button>`,
		`data-field-block-add="quote">Quote</button>`,
		`<template data-field-block-template="quote">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
}

// TestBlocksOnPage - a blocks field on the content page: its blocks by
// their set's display (component_default), cards in a grid.
func TestBlocksOnPage(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	character := entity.FieldInstanceDef{ID: 21, MachineName: "character", Label: "Character", FieldType: entity.FieldTypeText, ShowOnDetail: true}
	cast := entity.FieldInstanceDef{ID: 5, MachineName: "cast", Label: "Cast", FieldType: entity.FieldTypeBlocks, ShowOnDetail: true, ShowLabel: true, DetailLayout: entity.FieldLayoutGrid}
	item := entity.BlockItem{ID: 9, Type: "role", Fields: entity.FieldInstanceDefs{character}, Values: map[int]any{21: "Stalker"}}
	dc := &entity.DynamicContent{Fields: entity.FieldInstanceDefs{cast}, Values: map[int]any{5: []entity.BlockItem{item}}}
	itemDC := &entity.DynamicContent{Fields: item.Fields, Values: item.Values}
	views := map[int][]blockView{5: {{ID: 9, Type: "role", Sections: buildDisplaySections(item.Fields, itemDC, nil, nil), Ctx: map[string]any{
		"item": itemDC, "taxonomyFieldTerms": map[int]entity.TaxonomyTerms{}, "fileFieldLinks": map[int]fileLinkInfo{},
		"userFieldUsers": map[int][]entity.User{}, "referenceFieldNodes": map[int]entity.Nodes{}}}}}
	ctx := map[string]any{"item": dc, "blockViews": views}
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	if err := ts.RenderPageBlock(&buf, "dynamic/page.html", "node_sections", map[string]any{"Ctx": ctx, "Sections": buildDisplaySections(dc.Fields, dc, nil, nil)}); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		`<div class="dynamic-field dynamic-field-blocks dynamic-field-name-cast"> <h3>Cast</h3>`,
		`<div class="field-blocks-view field-blocks-view--grid">`,
		`<div class="component component--role">`,
		`Stalker`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
}

// TestPrefixedRequest - a block's inputs as a node's: only those of its
// prefix, the prefix cut off.
func TestPrefixedRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Form = url.Values{"field_5_n1_field_21": {"Stalker"}, "field_5_n1_delete_field_22": {"1"}, "field_5_n2_field_21": {"Writer"}, "title": {"Film"}}
	sub := prefixedRequest(r, blockInputPrefix(5, "n1"))
	if got := sub.FormValue("field_21"); got != "Stalker" {
		t.Errorf("field_21 = %q, want Stalker", got)
	}
	if got := sub.FormValue("delete_field_22"); got != "1" {
		t.Errorf("delete_field_22 = %q, want 1", got)
	}
	if got := sub.FormValue("title"); got != "" {
		t.Errorf("title = %q: another input came through", got)
	}
}

// countingNodes - referenceNodes counting its queries.
type countingNodes struct {
	referenceNodes
	calls int
}

func (n *countingNodes) ByIDs(ctx context.Context, ids []int) (map[int]entity.Node, error) {
	n.calls++
	out, err := n.referenceNodes.ByIDs(ctx, ids)
	for id, node := range out {
		node.Status, node.Slug = 1, "actor/"+strconv.Itoa(id)
		out[id] = node
	}
	return out, err
}

// TestBlockViewsReadOnce - the blocks of a field read the content they
// name in one query for all of them, each block given its own.
func TestBlockViewsReadOnce(t *testing.T) {
	nodes := &countingNodes{}
	c := &DynamicContentController{nodeRepo: nodes}
	actor := entity.FieldInstanceDef{ID: 3, MachineName: "actor", Label: "Actor", FieldType: entity.FieldTypeReference, ShowOnDetail: true}
	cast := entity.FieldInstanceDef{ID: 5, FieldType: entity.FieldTypeBlocks}
	var items []entity.BlockItem
	for i, target := range []int{7, 8, 9} {
		items = append(items, entity.BlockItem{ID: 100 + i, Type: "role", Fields: entity.FieldInstanceDefs{actor}, Values: map[int]any{3: target}})
	}
	views := c.blockViews(context.Background(), entity.FieldInstanceDefs{cast}, map[int]any{5: items})[5]
	if nodes.calls != 1 {
		t.Errorf("%d queries for the blocks' content, want 1", nodes.calls)
	}
	if len(views) != 3 {
		t.Fatalf("%d blocks, want 3", len(views))
	}
	for i, v := range views {
		refs := v.Ctx["referenceFieldNodes"].(map[int]entity.Nodes)[3]
		if len(refs) != 1 || refs[0].ID != 7+i {
			t.Errorf("block %d: content %+v, want node %d", i, refs, 7+i)
		}
	}
}
