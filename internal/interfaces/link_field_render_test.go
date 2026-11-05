package interfaces

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
)

// withLinkFormatter - a module "linkfmt" (on while on) showing a link
// field as "[players:<links>:<layout>]".
func withLinkFormatter(t *testing.T, on bool) {
	t.Helper()
	service.SetModules(map[string]bool{"linkfmt": on})
	t.Cleanup(func() { service.SetModules(map[string]bool{"linkfmt": false}) })
	service.RegisterFieldFormatter(service.FieldFormatter{Name: "linkfmt_player", Module: "linkfmt", Label: "Player", FieldType: entity.FieldTypeLink,
		Render: func(_ context.Context, v any, layout entity.FieldLayout) template.HTML {
			return template.HTML(fmt.Sprintf("[players:%d:%s]", len(entity.Links(v)), layout))
		}})
}

// renderNodeSections - the fields of a node page (dynamic/page.html's
// node_sections) of dc, from the repository's root.
func renderNodeSections(t *testing.T, dc *entity.DynamicContent) string {
	t.Helper()
	sections := buildDisplaySections(dc.Fields, dc, nil, nil)
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	ctx := map[string]any{"item": dc, "taxonomyFieldTerms": map[int]entity.TaxonomyTerms{}, "fileFieldLinks": map[int]fileLinkInfo{}, "referenceFieldNodes": map[int]entity.Nodes{}, "userFieldUsers": map[int][]entity.User{}}
	data := map[string]any{"Ctx": ctx, "Sections": sections}
	if err := ts.RenderPageBlock(&buf, "dynamic/page.html", "node_sections", data); err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// TestLinkFieldOnNodePage - a link field shows its links in the compact
// lines (one - a link with its title, several - a list; "first" - only
// the first); a module's formatter shows it in a block of its own while
// the module is on, as links when it's off.
func TestLinkFieldOnNodePage(t *testing.T) {
	links := []entity.Link{{URL: "https://video.example/1", Title: "Trailer"}, {URL: "https://video.example/2"}}
	field := entity.FieldInstanceDef{ID: 1, MachineName: "trailer", Label: "Trailer", FieldType: entity.FieldTypeLink, ShowOnDetail: true, ShowLabel: true,
		Config: map[string]any{"multiple": true}}
	dc := &entity.DynamicContent{Fields: entity.FieldInstanceDefs{field}, Values: map[int]any{1: links}}
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)

	out := renderNodeSections(t, dc)
	for _, want := range []string{
		`<span class="field-list">Trailer:</span>`,
		`<ul class="field-links"><li><a href="https://video.example/1" class="field-link" rel="nofollow noopener">Trailer</a></li><li><a href="https://video.example/2" class="field-link" rel="nofollow noopener">https://video.example/2</a></li></ul>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("links: no %s in\n%s", want, out)
		}
	}

	dc.Fields[0].DetailLayout = entity.FieldLayoutFirst
	if out := renderNodeSections(t, dc); strings.Contains(out, "video.example/2") || !strings.Contains(out, `<a href="https://video.example/1" class="field-link"`) {
		t.Errorf("first only: %s", out)
	}

	withLinkFormatter(t, true)
	dc.Fields[0].DetailFormatter, dc.Fields[0].DetailLayout = "linkfmt_player", entity.FieldLayoutFeatured
	out = renderNodeSections(t, dc)
	if !strings.Contains(out, `<div class="dynamic-field dynamic-field-formatted dynamic-field-name-trailer"> <h3>Trailer</h3> [players:2:featured] </div>`) {
		t.Errorf("formatter: %s", out)
	}
	service.SetModules(map[string]bool{"linkfmt": false})
	if out := renderNodeSections(t, dc); strings.Contains(out, "players") || !strings.Contains(out, `class="field-links"`) {
		t.Errorf("the formatter's module off, not links: %s", out)
	}
}

// TestLinkFieldInView - a View's link field: its formatter and layout
// (the field's config), links without one; never wrapped in a link of
// its own.
func TestLinkFieldInView(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	withLinkFormatter(t, true)
	render := func(config map[string]any) string {
		field := entity.FieldInstanceDef{ID: 3000000, FieldType: entity.FieldTypeLink, Label: "Trailer", MachineName: "trailer", Config: config}
		item := views.ContentListItem{
			Node:   entity.Node{ID: 3, Title: "Film", Slug: "film"},
			Fields: entity.FieldInstanceDefs{field},
			Values: map[int]any{3000000: []entity.Link{{URL: "https://video.example/1"}, {URL: "https://video.example/2"}}},
		}
		data := map[string]any{"isViewList": true, "listFieldsHead": item.Fields, "displayStyle": "grid", "gridColumns": 3, "items": []views.ContentListItem{item}}
		var buf bytes.Buffer
		if err := service.NewTemplateService(false).RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	if out := render(map[string]any{"formatter": "linkfmt_player", "layout": "first", "link_to": "content"}); !strings.Contains(out, "[players:1:first]") ||
		strings.Contains(out, `class="item-field-link"`) || !strings.Contains(out, "item-field item-field-links item-field-name-trailer") {
		t.Errorf("formatter: %s", out)
	}
	if out := render(map[string]any{"formatter": "", "layout": ""}); !strings.Contains(out, `<a href="https://video.example/2" class="field-link"`) {
		t.Errorf("links: %s", out)
	}
}

// TestLinkFieldInForm - the content form's link field: a row per link
// (its address and, a field with titles, its title), an empty row
// without links, ↑/↓/✕ and "Add a link" with the field's limit for
// several; one row, no buttons for one.
func TestLinkFieldInForm(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	render := func(field entity.FieldInstanceDef, value any) string {
		data := map[string]any{
			"title":        "Editing: Film",
			"contentType":  &entity.ContentTypeDef{MachineName: "film", Label: "Film"},
			"dc":           &entity.DynamicContent{},
			"node":         &entity.Node{},
			"form":         form.DynamicForm{Title: "Film"},
			"formSections": buildFormSections(entity.FieldInstanceDefs{field}),
			"values":       map[int]any{field.ID: value},
		}
		for _, k := range []string{"taxonomyOptions", "taxonomyHierarchyLabels", "multiTaxonomyFieldTerms", "referenceFieldNodes",
			"imageFieldURLs", "imageFieldAlts", "imageFieldItems", "fileFieldLinks", "textFormatIDs", "userFieldUsers", "tagPickers", "blockFields"} {
			data[k] = map[int]any{}
		}
		return renderNodeEdit(t, data)
	}
	several := entity.FieldInstanceDef{ID: 7, Label: "Trailer", FieldType: entity.FieldTypeLink,
		Config: map[string]any{"multiple": true, "link_title": "optional", "allowed_hosts": []any{"video.example"}}, WidgetConfig: map[string]any{"max_values": float64(3)}}
	out := render(several, []entity.Link{{URL: "https://video.example/1", Title: "Main"}, {URL: "https://video.example/2"}})
	for _, want := range []string{
		`data-field-links data-field-rows data-max="3"`,
		`id="field_7" name="field_7_url" value="https://video.example/1"`,
		`name="field_7_title" value="Main"`,
		`name="field_7_url" value="https://video.example/2"`,
		`data-field-row-remove`,
		`<template data-field-row-template>`,
		`data-field-row-add>+ Add a link</button>`,
		`Links to: video.example · at most 3 links`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("several: no %s in\n%s", want, out)
		}
	}

	one := entity.FieldInstanceDef{ID: 8, Label: "Site", FieldType: entity.FieldTypeLink}
	out = render(one, nil)
	if !strings.Contains(out, `id="field_8" name="field_8_url" value=""`) || strings.Contains(out, "field_8_title") ||
		strings.Contains(out, "data-field-row-add") || strings.Contains(out, "data-field-row-remove") {
		t.Errorf("one: %s", out)
	}
}
