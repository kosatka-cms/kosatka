package service

import (
	"bytes"
	"context"
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestTextFiltersRender - a body section's HTML value and a block's body
// go through their format's text filters; a value of another format or
// none doesn't.
func TestTextFiltersRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	withModules(t, nil)
	mark := TextFilter{Name: "mark", Module: "m", Apply: func(_ context.Context, html string) string {
		return strings.ReplaceAll(html, "VIDEO", `<div class="filtered">player</div>`)
	}}
	withTextFilters(t, map[int][]string{5: {"mark"}}, mark)

	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("dynamic/page.html", []string{"dynamic", "page.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("text-filters-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	body, err := set.New("text_filters_body").Parse(`{{template "node_body_value" .}}`)
	if err != nil {
		t.Fatal(err)
	}
	rich := &entity.FieldInstanceDef{FieldType: entity.FieldTypeText, WidgetConfig: map[string]any{"allowed_text_formats": []any{float64(5)}}}
	for _, tc := range []struct {
		data     map[string]any
		filtered bool
	}{
		{map[string]any{"Field": rich, "Value": "<p>VIDEO</p>", "Format": 5}, true},
		{map[string]any{"Field": rich, "Value": "<p>VIDEO</p>", "Format": 6}, false},
		{map[string]any{"Field": rich, "Value": "<p>VIDEO</p>"}, false},
	} {
		var buf strings.Builder
		if err := body.Execute(&buf, tc.data); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(buf.String(), `class="filtered"`); got != tc.filtered {
			t.Errorf("format %v: filtered %v, want %v: %s", tc.data["Format"], got, tc.filtered, buf.String())
		}
	}

	format := 5
	blocks := map[entity.Region][]entity.BlockWithPlacement{entity.Region("sidebar"): {{
		Block:     entity.BlockDef{Title: "Trailer", Body: "<p>VIDEO</p>", TextFormatID: &format},
		Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementCustom, VisibilityScope: entity.BlockVisibilityAll},
	}}}
	var buf bytes.Buffer
	data := map[string]any{"Region": entity.Region("sidebar"), "Ctx": map[string]any{"blockPlacements": blocks}}
	if err := NewTemplateService(false).RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `<div class="filtered">player</div>`) {
		t.Errorf("block body: %s", buf.String())
	}
}

// TestTextFormatEditFilters - the format page lists the enabled modules'
// filters, the format's own ticked; none - no section.
func TestTextFormatEditFilters(t *testing.T) {
	zzChdirToProjectRoot(t)
	withModules(t, nil)
	withTextFilters(t, nil, TextFilter{Name: "video", Module: "m", Label: "Video links as players", Description: "On its own line."}, TextFilter{Name: "other", Module: "m", Label: "Other"})
	render := func(data map[string]any) string {
		var buf bytes.Buffer
		if err := NewTemplateService(false).RenderNamed(&buf, "content", data, "admin/text-formats/edit.html", "admin/upload_limits.html", "admin/alerts.html"); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	out := render(map[string]any{"format": &entity.TextFormatDef{ID: 2, Filters: "video"}, "textFilters": TextFilters()})
	for _, want := range []string{`id="filter_video" name="filters" value="video" checked`, `id="filter_other" name="filters" value="other">`, "Video links as players", "On its own line."} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if out := render(map[string]any{"format": &entity.TextFormatDef{ID: 2}}); strings.Contains(out, `name="filters"`) {
		t.Error("no filters: the section must not render")
	}
}
