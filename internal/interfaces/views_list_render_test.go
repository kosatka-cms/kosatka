package interfaces

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

func TestRewriteHTMLAndCustomCSSRenderThroughDynamicListHTML(t *testing.T) {
	ts := service.NewTemplateService(false)

	titleField := entity.FieldInstanceDef{
		ID:        1000000,
		FieldType: entity.FieldTypeText,
		Label:     "Title",
		Config: map[string]any{
			"css_class":    "",
			"link_to":      "",
			"rewrite_html": `<div class="title-overlay">{{value}}</div>`,
		},
	}
	yearField := entity.FieldInstanceDef{
		ID:        1000001,
		FieldType: entity.FieldTypeText,
		Label:     "Year",
		Config: map[string]any{
			"css_class": "",
			"link_to":   "",
			// no rewrite_html on purpose - must render unwrapped.
		},
	}

	item := views.ContentListItem{
		Node:   entity.Node{ID: 1, Title: "Secret Wars", Slug: "secret-wars"},
		Fields: entity.FieldInstanceDefs{titleField, yearField},
		Values: map[int]any{1000000: "Secret Wars", 1000001: "1984"},
	}

	data := map[string]any{
		"customCSS":      `#view-list-container .title-overlay { position: absolute; }`,
		"headerText":     "",
		"footerText":     "",
		"isViewList":     true,
		"listFieldsHead": entity.FieldInstanceDefs{titleField, yearField},
		"displayStyle":   "grid",
		"gridColumns":    5,
		"showPagination": false,
		"items":          []views.ContentListItem{item},
	}

	var buf bytes.Buffer
	if err := ts.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatalf("RenderNamed error: %v", err)
	}
	out := buf.String()

	// RewriteHTML wraps the field's already rendered value (its whole
	// <span class="item-field-value">), so styles/badges/links are preserved.
	wantWrapped := `<div class="title-overlay">`
	titleIdx := strings.Index(out, wantWrapped)
	if titleIdx == -1 {
		t.Fatalf("expected RewriteHTML wrapper div for the title field; got:\n%s", out)
	}
	closeIdx := strings.Index(out[titleIdx:], "</div>")
	if closeIdx == -1 {
		t.Fatalf("no closing </div> found for the RewriteHTML wrapper; got:\n%s", out)
	}
	wrappedSpan := out[titleIdx : titleIdx+closeIdx]
	if !strings.Contains(wrappedSpan, `<span class="item-field-value">Secret Wars</span>`) {
		t.Errorf("expected the title field's rendered span inside the RewriteHTML wrapper; got:\n%s", wrappedSpan)
	}
	// The year field has no rewrite_html, so its span must not be wrapped.
	yearIdx := strings.Index(out, `<span class="item-field-value">1984</span>`)
	if yearIdx == -1 {
		t.Fatalf("expected the year field's plain value to render somewhere; got:\n%s", out)
	}
	if yearIdx > titleIdx && yearIdx < titleIdx+closeIdx {
		t.Errorf("year field's value ended up inside the title field's RewriteHTML wrapper; got:\n%s", out)
	}
	if !strings.Contains(out, `<style>#view-list-container .title-overlay { position: absolute; }</style>`) {
		t.Errorf("expected customCSS to render verbatim inside a <style> tag; got:\n%s", out)
	}
}

// TestRewriteHTMLAbsentIsNoOp - a field without "rewrite_html" in Config
// renders through dynamic_list_field_value unchanged.
func TestRewriteHTMLAbsentIsNoOp(t *testing.T) {
	ts := service.NewTemplateService(false)

	field := entity.FieldInstanceDef{
		ID:        2000000,
		FieldType: entity.FieldTypeText,
		Label:     "Name",
		Config:    map[string]any{},
	}
	item := views.ContentListItem{
		Node:   entity.Node{ID: 2, Title: "Item", Slug: "item"},
		Fields: entity.FieldInstanceDefs{field},
		Values: map[int]any{2000000: "Some Item"},
	}
	data := map[string]any{
		"customCSS":      "",
		"headerText":     "",
		"footerText":     "",
		"isViewList":     false,
		"listFieldsHead": entity.FieldInstanceDefs{field},
		"displayStyle":   "grid",
		"gridColumns":    5,
		"showPagination": false,
		"items":          []views.ContentListItem{item},
	}

	var buf bytes.Buffer
	if err := ts.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatalf("RenderNamed error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "<style>") {
		t.Errorf("expected no <style> tag when customCSS is empty; got:\n%s", out)
	}
	if !strings.Contains(out, "Some Item") {
		t.Errorf("expected the field's value to render; got:\n%s", out)
	}
}

// TestListModeRendersHonestListWithoutCards - dynamic/list.html with
// displayStyle "list": same per-field rendering as "grid", but no card
// wrapper and no title highlighting.
// TestListTextEscaping - a text field's value is output as HTML only for
// a RichText field (sanitized on save); plain text, like a node title
// anyone creating content types in, is escaped.
func TestListTextEscaping(t *testing.T) {
	ts := service.NewTemplateService(false)
	plain := entity.FieldInstanceDef{ID: 1, FieldType: entity.FieldTypeText, Label: "Title", Config: map[string]any{}}
	rich := entity.FieldInstanceDef{ID: 2, FieldType: entity.FieldTypeText, Label: "Body", Config: map[string]any{},
		WidgetConfig: map[string]any{"widget": string(entity.WidgetSafeHTML)}}
	item := views.ContentListItem{
		Node:   entity.Node{ID: 3, Title: "x", Slug: "x"},
		Fields: entity.FieldInstanceDefs{plain, rich},
		Values: map[int]any{1: `<img src=x onerror=alert(1)>`, 2: "<b>bold</b>"},
	}
	data := map[string]any{"isViewList": true, "listFieldsHead": entity.FieldInstanceDefs{plain, rich},
		"displayStyle": "list", "items": []views.ContentListItem{item}}
	var buf bytes.Buffer
	if err := ts.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatalf("RenderNamed error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "<img src=x") || !strings.Contains(out, "&lt;img src=x onerror=alert(1)&gt;") {
		t.Errorf("plain text not escaped:\n%s", out)
	}
	if !strings.Contains(out, "<b>bold</b>") {
		t.Errorf("rich text escaped:\n%s", out)
	}
}

func TestListModeRendersHonestListWithoutCards(t *testing.T) {
	ts := service.NewTemplateService(false)

	field := entity.FieldInstanceDef{
		ID:        3000000,
		FieldType: entity.FieldTypeText,
		Label:     "Name",
		Config:    map[string]any{},
	}
	item := views.ContentListItem{
		Node:   entity.Node{ID: 3, Title: "Comic", Slug: "comic"},
		Fields: entity.FieldInstanceDefs{field},
		Values: map[int]any{3000000: "Some Comic"},
	}
	data := map[string]any{
		"customCSS":      "",
		"headerText":     "",
		"footerText":     "",
		"isViewList":     true,
		"listFieldsHead": entity.FieldInstanceDefs{field},
		"displayStyle":   "list",
		"showPagination": false,
		"items":          []views.ContentListItem{item},
	}

	var buf bytes.Buffer
	if err := ts.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatalf("RenderNamed error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Some Comic") {
		t.Errorf("expected the field's value to render; got:\n%s", out)
	}
	if strings.Contains(out, "list-grid-item") || strings.Contains(out, "item-field-title") {
		t.Errorf("expected no card markup in list mode; got:\n%s", out)
	}
	if !strings.Contains(out, `class="view-list-item"`) {
		t.Errorf("expected the plain list-item wrapper; got:\n%s", out)
	}
}

// TestRowTemplateRendersThroughDynamicListHTML - a display's row
// template replaces the per-field markup of grid/list/carousel cards:
// {{field:x}} → the field's rendered output (an excluded field works,
// a field with no value renders empty), {{url}} → the item's path; the
// table ignores it, and without one the fields render as before.
func TestRowTemplateRendersThroughDynamicListHTML(t *testing.T) {
	ts := service.NewTemplateService(false)
	title := entity.FieldInstanceDef{ID: 1000000, MachineName: "title", FieldType: entity.FieldTypeText, Label: "Title", Config: map[string]any{}}
	year := entity.FieldInstanceDef{ID: 1000001, MachineName: "year", FieldType: entity.FieldTypeText, Label: "Year", ExcludeFromDisplay: true, Config: map[string]any{}}
	note := entity.FieldInstanceDef{ID: 1000002, MachineName: "note", FieldType: entity.FieldTypeText, Label: "Note", Config: map[string]any{}}
	fields := entity.FieldInstanceDefs{title, year, note}
	item := views.ContentListItem{
		Node:   entity.Node{ID: 1, Type: "movies", Title: "Secret Wars", Slug: "movies/secret-wars"},
		Fields: fields,
		Values: map[int]any{1000000: "Secret Wars", 1000001: "1984"},
	}
	render := func(t *testing.T, style, rowTemplate string) string {
		t.Helper()
		data := map[string]any{
			"isViewList":     true,
			"listFieldsHead": fields,
			"displayStyle":   style,
			"rowTemplate":    rowTemplate,
			"items":          []views.ContentListItem{item},
		}
		var buf bytes.Buffer
		if err := ts.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
			t.Fatalf("RenderNamed error: %v", err)
		}
		// Whitespace collapsed: a field's rendered output carries the
		// template's own line breaks.
		return strings.Join(strings.Fields(buf.String()), " ")
	}
	tpl := `<a href="{{url}}" class="my-card"><b>{{field:title}}</b> <i>{{field:year}}</i> <u>{{field:note}}</u></a>`
	for _, style := range []string{"grid", "list", "carousel"} {
		out := render(t, style, tpl)
		for _, want := range []string{
			`<a href="/movies/secret-wars" class="my-card">`,
			`<b> <span class="item-field-value">Secret Wars</span> </b>`,
			`<i> <span class="item-field-value">1984</span> </i>`,
			`<u></u>`,
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("%s: missing %q in:\n%s", style, want, out)
			}
		}
		if strings.Contains(out, "{{field:") || strings.Contains(out, `class="item-field item-field-`) || strings.Contains(out, "view-carousel-slide-link") {
			t.Fatalf("%s: the row template must replace the per-field markup:\n%s", style, out)
		}
	}
	if out := render(t, "table", tpl); strings.Contains(out, "my-card") {
		t.Fatalf("table: the row template is not used:\n%s", out)
	}
	if out := render(t, "grid", ""); !strings.Contains(out, `class="item-field item-field-text item-field-name-title item-field-title"`) || strings.Contains(out, "my-card") {
		t.Fatalf("no row template - the fields as before:\n%s", out)
	}
}

// TestRowTemplateInViewPreview - the admin preview (admin/views/preview.html,
// its own grid) applies the draft row template too.
func TestRowTemplateInViewPreview(t *testing.T) {
	ts := service.NewTemplateService(false)
	title := entity.FieldInstanceDef{ID: 1000000, MachineName: "title", FieldType: entity.FieldTypeText, Label: "Title", Config: map[string]any{}}
	fields := entity.FieldInstanceDefs{title}
	item := views.ContentListItem{
		Node:   entity.Node{ID: 1, Type: "movies", Title: "Secret Wars", Slug: "movies/secret-wars"},
		Fields: fields,
		Values: map[int]any{1000000: "Secret Wars"},
	}
	render := func(rowTemplate string) string {
		data := map[string]any{"isViewList": true, "listFieldsHead": fields, "displayStyle": "grid", "rowTemplate": rowTemplate, "items": []views.ContentListItem{item}}
		var buf bytes.Buffer
		if err := ts.RenderNamed(&buf, "view_preview", data, "admin/views/preview.html", "dynamic/list.html", "pagination.html"); err != nil {
			t.Fatalf("RenderNamed error: %v", err)
		}
		return strings.Join(strings.Fields(buf.String()), " ")
	}
	if out := render(`<a href="{{url}}" class="my-card">{{field:title}}</a>`); !strings.Contains(out, `<a href="/movies/secret-wars" class="my-card"> <span class="item-field-value">Secret Wars</span> </a>`) || strings.Contains(out, `class="item-field item-field-`) {
		t.Fatalf("preview with a row template:\n%s", out)
	}
	if out := render(""); !strings.Contains(out, `class="item-field item-field-text item-field-name-title item-field-title"`) {
		t.Fatalf("preview without a row template:\n%s", out)
	}
}

// TestViewPreviewIsThePublishedList - the admin preview renders the list
// as the published page does: the list style, a field excluded from the
// display left out, the items past OverflowAfter in the overflow list.
func TestViewPreviewIsThePublishedList(t *testing.T) {
	ts := service.NewTemplateService(false)
	title := entity.FieldInstanceDef{ID: 1000000, MachineName: "title", FieldType: entity.FieldTypeText, Label: "Title", Config: map[string]any{}}
	hidden := entity.FieldInstanceDef{ID: 1000001, MachineName: "secret", FieldType: entity.FieldTypeText, Label: "Secret", Config: map[string]any{}, ExcludeFromDisplay: true}
	fields := entity.FieldInstanceDefs{title, hidden}
	row := func(id int, name string) views.ContentListItem {
		return views.ContentListItem{Node: entity.Node{ID: id, Type: "movies", Title: name, Slug: "movies/" + name}, Fields: fields, Values: map[int]any{1000000: name, 1000001: "hush-" + name}}
	}
	data := map[string]any{"isViewList": true, "listFieldsHead": fields, "displayStyle": "list", "overflowAfter": 1,
		"items": []views.ContentListItem{row(1, "a"), row(2, "b")}}
	var buf bytes.Buffer
	if err := ts.RenderNamed(&buf, "view_preview", data, "admin/views/preview.html", "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `class="view-list-item`) || !strings.Contains(out, `class="view-overflow-list"`) || strings.Contains(out, "hush-") {
		t.Fatalf("the preview isn't the published list:\n%s", out)
	}
}

// TestGroupingRendersThroughDynamicListHTML - with GroupByField rows are
// grouped by the field's value (dates by month here): each heading once, in
// order of first appearance, every matching row under it regardless of sort;
// one grid per group, a heading row in the table, an <h3> in the list; the
// preview groups too.
func TestGroupingRendersThroughDynamicListHTML(t *testing.T) {
	ts := service.NewTemplateService(false)
	title := entity.FieldInstanceDef{ID: 1000000, MachineName: "title", FieldType: entity.FieldTypeText, Label: "Title", Config: map[string]any{}}
	premiere := entity.FieldInstanceDef{ID: 1000001, MachineName: "premiere", FieldType: entity.FieldTypeDate, Label: "Premiere", Config: map[string]any{}}
	fields := entity.FieldInstanceDefs{title, premiere}
	row := func(id int, name string, d time.Time) views.ContentListItem {
		return views.ContentListItem{Node: entity.Node{ID: id, Type: "movies", Title: name, Slug: "movies/" + name}, Fields: fields, Values: map[int]any{1000000: name, 1000001: d}}
	}
	// Not sorted by the grouping field: b (January) comes after c
	// (March) and still joins the January group.
	items := []views.ContentListItem{
		row(1, "a", time.Date(2027, 1, 5, 0, 0, 0, 0, time.UTC)),
		row(3, "c", time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)),
		row(2, "b", time.Date(2027, 1, 20, 0, 0, 0, 0, time.UTC)),
	}
	render := func(t *testing.T, tmpl, style string, files ...string) string {
		t.Helper()
		data := map[string]any{"isViewList": true, "listFieldsHead": fields, "displayStyle": style, "groupByField": "premiere", "groupDateFormat": "month", "items": items}
		var buf bytes.Buffer
		if err := ts.RenderNamed(&buf, tmpl, data, files...); err != nil {
			t.Fatalf("RenderNamed error: %v", err)
		}
		return strings.Join(strings.Fields(buf.String()), " ")
	}
	check := func(t *testing.T, name, out, heading string, grids int) {
		t.Helper()
		if strings.Count(out, heading) != 2 || !strings.Contains(out, "January 2027") || !strings.Contains(out, "March 2027") || strings.Count(out, "January 2027") != 1 {
			t.Fatalf("%s: expected one heading per month:\n%s", name, out)
		}
		if i, j := strings.Index(out, "January 2027"), strings.Index(out, "March 2027"); i > j || !strings.Contains(out[i:j], ">a<") || !strings.Contains(out[i:j], ">b<") || strings.Contains(out[j:], ">b<") || !strings.Contains(out[j:], ">c<") || strings.Index(out[i:j], ">a<") > strings.Index(out[i:j], ">b<") {
			t.Fatalf("%s: rows under the wrong heading:\n%s", name, out)
		}
		if grids >= 0 && strings.Count(out, `class="list-grid list-grid-fields"`) != grids {
			t.Fatalf("%s: expected %d grids:\n%s", name, grids, out)
		}
	}
	list := []string{"dynamic/list.html", "pagination.html"}
	check(t, "grid", render(t, "content", "grid", list...), `<h3 class="view-group-heading">`, 2)
	check(t, "list", render(t, "content", "list", list...), `<h3 class="view-group-heading">`, 0)
	check(t, "table", render(t, "content", "table", list...), `<tr class="view-group-heading">`, -1)
	preview := []string{"admin/views/preview.html", "dynamic/list.html", "pagination.html"}
	check(t, "preview grid", render(t, "view_preview", "grid", preview...), `<h3 class="view-group-heading">`, 2)
	check(t, "preview table", render(t, "view_preview", "table", preview...), `<tr class="view-group-heading">`, -1)

	// No grouping: one grid, no headings.
	data := map[string]any{"isViewList": true, "listFieldsHead": fields, "displayStyle": "grid", "items": items}
	var buf bytes.Buffer
	if err := ts.RenderNamed(&buf, "content", data, list...); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); strings.Contains(out, "view-group-heading") || strings.Count(out, `class="list-grid list-grid-fields"`) != 1 {
		t.Fatalf("no grouping:\n%s", out)
	}
}

type fakeListDateFormats struct{}

func (fakeListDateFormats) DateFormats(context.Context) ([]entity.DateFormat, error) {
	return []entity.DateFormat{{ID: 1, MachineName: "short", Pattern: "d.m.Y"}, {ID: 2, MachineName: "medium", Pattern: "j F Y"}}, nil
}

func (fakeListDateFormats) DateFormatByID(context.Context, int) (*entity.DateFormat, error) {
	return nil, nil
}

func (fakeListDateFormats) CreateDateFormat(context.Context, entity.DateFormat) (int, error) {
	return 0, nil
}

func (fakeListDateFormats) UpdateDateFormat(context.Context, entity.DateFormat) error { return nil }

func (fakeListDateFormats) DeleteDateFormat(context.Context, int) error { return nil }

func (fakeListDateFormats) DateFormatUsage(context.Context, string) ([]string, error) {
	return nil, nil
}

func (fakeListDateFormats) InstanceDateFormats(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

// TestDateFormatInViewList - a date field with a Views date format renders
// in that format in the cell, in the row template's token and in a group
// heading that uses the field's format.
func TestDateFormatInViewList(t *testing.T) {
	ts := service.NewTemplateService(false)
	ts.(*service.DefaultTemplateService).SetDateFormats(service.NewDateFormatService(fakeListDateFormats{}, slog.New(slog.NewTextHandler(io.Discard, nil))))
	premiere := entity.FieldInstanceDef{ID: 1000001, FieldStorageID: 1000001, MachineName: "premiere", FieldType: entity.FieldTypeDate, Label: "Premiere", Config: map[string]any{"date_format": "medium"}}
	fields := entity.FieldInstanceDefs{premiere}
	item := views.ContentListItem{Node: entity.Node{ID: 1, Type: "movies", Slug: "movies/x"}, Fields: fields, Values: map[int]any{1000001: time.Date(2027, 3, 5, 0, 0, 0, 0, time.UTC)}}
	render := func(extra map[string]any) string {
		data := map[string]any{"isViewList": true, "listFieldsHead": fields, "displayStyle": "grid", "items": []views.ContentListItem{item}}
		maps.Copy(data, extra)
		var buf bytes.Buffer
		if err := ts.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
			t.Fatalf("RenderNamed error: %v", err)
		}
		return buf.String()
	}
	if out := render(nil); !strings.Contains(out, "5 March 2027") || strings.Contains(out, "05.03.2027") {
		t.Fatalf("cell:\n%s", out)
	}
	if out := render(map[string]any{"rowTemplate": `<i>{{field:premiere}}</i>`}); !strings.Contains(out, "5 March 2027") {
		t.Fatalf("row template token:\n%s", out)
	}
	if out := render(map[string]any{"groupByField": "premiere"}); strings.Count(out, "5 March 2027") != 2 {
		t.Fatalf("group heading (field format) + cell:\n%s", out)
	}
}

// TestCarouselInViewPreview - the admin preview shows a carousel display
// as the same carousel markup as the published page (view_carousel), not
// as a grid.
func TestCarouselInViewPreview(t *testing.T) {
	ts := service.NewTemplateService(false)
	title := entity.FieldInstanceDef{ID: 1000000, MachineName: "title", FieldType: entity.FieldTypeText, Label: "Title", Config: map[string]any{}}
	fields := entity.FieldInstanceDefs{title}
	item := views.ContentListItem{
		Node:   entity.Node{ID: 1, Type: "movies", Title: "Secret Wars", Slug: "movies/secret-wars"},
		Fields: fields,
		Values: map[int]any{1000000: "Secret Wars"},
	}
	data := map[string]any{
		"isViewList": true, "listFieldsHead": fields, "displayStyle": "carousel", "items": []views.ContentListItem{item},
		"carouselSlidesPerView": 3, "carouselShowNavigation": true, "carouselEffect": "fade",
	}
	render := func(name string, files ...string) string {
		var buf bytes.Buffer
		if err := ts.RenderNamed(&buf, name, data, files...); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return strings.Join(strings.Fields(buf.String()), " ")
	}
	for _, out := range []string{
		render("view_preview", "admin/views/preview.html", "dynamic/list.html", "pagination.html"),
		render("view_list_body", "dynamic/list.html", "pagination.html"),
	} {
		for _, want := range []string{`data-view-carousel="1"`, `data-slides-per-view="3"`, `data-effect="fade"`, `class="swiper-slide view-carousel-slide"`, `href="/movies/secret-wars"`, `swiper-button-next`} {
			if !strings.Contains(out, want) {
				t.Errorf("lacks %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "list-grid-item") {
			t.Errorf("rendered as a grid:\n%s", out)
		}
	}
}

// fakeFragmentViews - the View's fields.
type fakeFragmentViews struct {
	views.Service
	fields entity.ViewFieldDefs
}

type fakeFragmentTypes struct {
	entity.ContentTypeRepository
	storages map[int]entity.FieldStorageDef
	values   map[int]map[int]any // node ID → storage ID → value
}

func (f fakeFragmentTypes) FieldStorageByID(_ context.Context, id int) (*entity.FieldStorageDef, error) {
	fs, ok := f.storages[id]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	return &fs, nil
}

func (f fakeFragmentTypes) FieldInstanceLabelsByFieldStorageIDs(context.Context, []int) (map[int]string, error) {
	return nil, nil
}

func (f fakeFragmentTypes) RichTextFieldStorageIDs(context.Context, []int) (map[int]bool, error) {
	return nil, nil
}

func (f fakeFragmentTypes) ContentTypes(context.Context) (entity.ContentTypeDefs, error) {
	return nil, nil
}

func (f fakeFragmentTypes) FieldValuesForNodes(_ context.Context, nodeIDs []int, fields entity.FieldInstanceDefs) (map[int]map[int]any, error) {
	out := map[int]map[int]any{}
	for _, nid := range nodeIDs {
		out[nid] = map[int]any{}
		for _, fd := range fields {
			if v, ok := f.values[nid][fd.FieldStorageID]; ok {
				out[nid][fd.ID] = v
			}
		}
	}
	return out, nil
}

type fakeFragmentNodes struct {
	entity.NodesRepository
	alt map[int]string
}

func (f fakeFragmentNodes) AltTitles(context.Context, entity.Nodes) (map[int]string, error) {
	return f.alt, nil
}

// TestContentViewFragment - a contextual View block (a fragment on a
// node's page) builds its list as the View's page and its cached block
// do (views.BuildContentList): the nodes' alternative titles, a list
// field's values, its own container id and its pager.
func TestContentViewFragment(t *testing.T) {
	kind := 31
	c := &DynamicContentController{
		BaseController: &BaseController{templateService: service.NewTemplateService(false), logger: slog.New(slog.DiscardHandler)},
		viewService: fakeFragmentViews{fields: entity.ViewFieldDefs{
			{ID: 1, BuiltinField: entity.ViewFieldBuiltinTitle, SortOrder: 1},
			{ID: 2, FieldStorageID: &kind, Label: "Kind", ShowLabel: true, SortOrder: 2},
		}},
		contentTypeRepo: fakeFragmentTypes{
			storages: map[int]entity.FieldStorageDef{kind: {ID: kind, MachineName: "kind", FieldType: entity.FieldTypeList}},
			values:   map[int]map[int]any{2: {kind: "Sloop"}},
		},
		nodeRepo: fakeFragmentNodes{alt: map[int]string{1: "The Whale"}},
	}
	nodes := entity.Nodes{{ID: 1, Type: "article", Title: "Moby", Slug: "article/moby"}, {ID: 2, Type: "article", Title: "Pequod", Slug: "article/pequod"}}
	cfg := views.Config{View: entity.ViewDisplayDef{ID: 9, DisplayStyle: entity.ViewDisplayStyleList}, Fields: c.viewService.(fakeFragmentViews).fields}
	html, err := c.renderContentViewFragment(t.Context(), cfg, nodes,
		pagination.New(1, 4, 2, "/article/moby", nil))
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(string(html)), " ")
	for _, want := range []string{
		`id="view-list-container-block-9"`,
		`<span class="item-field-value">The Whale</span>`,
		`<span class="item-field-value">Pequod</span>`,
		`<span class="item-field-label">Kind:</span> <span class="item-field-value"><span class="list-values"><span class="list-value list-value-0" data-list-value="Sloop">Sloop</span></span></span>`,
		`page=2`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the fragment has no %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, ">Moby<") {
		t.Errorf("the fragment shows the title, not the alternative title:\n%s", out)
	}
}

// TestContentViewFragmentStatus - a list with the status, update date and
// edit link fields: each item's status in words (published, unpublished,
// on moderation), its last save's date, a link to its edit form.
func TestContentViewFragmentStatus(t *testing.T) {
	fields := entity.ViewFieldDefs{
		{ID: 1, BuiltinField: entity.ViewFieldBuiltinTitle, SortOrder: 1},
		{ID: 2, BuiltinField: entity.ViewFieldBuiltinStatus, SortOrder: 2},
		{ID: 3, BuiltinField: entity.ViewFieldBuiltinUpdatedAt, SortOrder: 3},
		{ID: 4, BuiltinField: entity.ViewFieldBuiltinEditLink, SortOrder: 4},
	}
	c := &DynamicContentController{
		BaseController:  &BaseController{templateService: service.NewTemplateService(false), logger: slog.New(slog.DiscardHandler)},
		viewService:     fakeFragmentViews{fields: fields},
		contentTypeRepo: fakeFragmentTypes{},
		nodeRepo:        fakeFragmentNodes{},
	}
	updated := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	nodes := entity.Nodes{
		{ID: 1, Type: "article", Title: "Out", Slug: "article/out", Status: 1, UpdatedAt: updated},
		{ID: 2, Type: "article", Title: "Draft", Slug: "article/draft", Status: 0, UpdatedAt: updated},
		{ID: 3, Type: "article", Title: "Waiting", Slug: "article/waiting", Status: 1, OnModeration: 1, UpdatedAt: updated},
	}
	cfg := views.Config{View: entity.ViewDisplayDef{ID: 9, DisplayStyle: entity.ViewDisplayStyleList}, Fields: fields}
	html, err := c.renderContentViewFragment(t.Context(), cfg, nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(string(html)), " ")
	for _, want := range []string{
		`<span class="item-field-value">Published</span>`,
		`<span class="item-field-value">Unpublished</span>`,
		`<span class="item-field-value">On moderation</span>`,
		`<span class="item-field-value">09.11.2026</span>`,
		`<a href="/node/2/edit" class="item-field-link"> <span class="item-field-value">Edit</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the fragment has no %s:\n%s", want, out)
		}
	}
}

// TestGridCardTitle - a grid card's title is the built-in title when the
// View shows it, though a field before it has a value (a verdict over a
// review's title); a list field's options carry their places.
func TestGridCardTitle(t *testing.T) {
	ts := service.NewTemplateService(false)
	verdict := entity.FieldInstanceDef{ID: 5, MachineName: "verdict", FieldType: entity.FieldTypeList, Label: "Verdict",
		Config: map[string]any{"allowed_values": []any{map[string]any{"value": "top", "label": "Top"}, map[string]any{"value": "fine", "label": "Fine"}}}}
	title := entity.FieldInstanceDef{ID: -1, MachineName: "title", FieldType: entity.FieldTypeText, Label: "Title", Config: map[string]any{}}
	fields := entity.FieldInstanceDefs{verdict, title}
	item := views.ContentListItem{
		Node:   entity.Node{ID: 1, Type: "review", Title: "Secret Wars", Slug: "review/secret-wars"},
		Fields: fields,
		Values: map[int]any{5: verdict.ListItems("fine"), -1: "Secret Wars"},
	}
	data := map[string]any{"isViewList": true, "listFieldsHead": fields, "displayStyle": "grid", "items": []views.ContentListItem{item}}
	var buf bytes.Buffer
	if err := ts.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		`class="item-field item-field-list item-field-name-verdict">`,
		`<span class="list-value list-value-2" data-list-value="fine">Fine</span>`,
		`class="item-field item-field-text item-field-name-title item-field-title"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in:\n%s", want, out)
		}
	}
}

// TestStickyRowClass - a sticky item's row and grid card are marked (a
// theme makes it lead the list), the others not.
func TestStickyRowClass(t *testing.T) {
	ts := service.NewTemplateService(false)
	title := entity.FieldInstanceDef{ID: -1, MachineName: "title", FieldType: entity.FieldTypeText, Label: "Title", Config: map[string]any{}}
	fields := entity.FieldInstanceDefs{title}
	items := []views.ContentListItem{
		{Node: entity.Node{ID: 1, Type: "news", Title: "Pinned", Slug: "news/pinned", Sticky: true}, Fields: fields, Values: map[int]any{-1: "Pinned"}},
		{Node: entity.Node{ID: 2, Type: "news", Title: "Plain", Slug: "news/plain"}, Fields: fields, Values: map[int]any{-1: "Plain"}},
	}
	for style, class := range map[string]string{"list": "view-list-item", "grid": "list-grid-item"} {
		data := map[string]any{"isViewList": true, "listFieldsHead": fields, "displayStyle": style, "items": items}
		var buf bytes.Buffer
		if err := ts.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if n := strings.Count(out, class+"-sticky"); n != 1 {
			t.Errorf("%s: %d sticky rows, want 1:\n%s", style, n, out)
		}
	}
}

// TestExposedCheckboxesRender - an in filter's options render as
// checkboxes named by its parameter (sent repeated), the current ones
// ticked, under the filter's label; a select stays a select.
func TestExposedCheckboxesRender(t *testing.T) {
	ts := service.NewTemplateService(false)
	data := map[string]any{"viewURLPath": "games", "exposedFilters": []exposedFilterFormField{
		choiceFilterField("Genre", "genre", entity.ViewFilterOperatorIn, "3,5",
			[]exposedFilterOption{{Value: "3", Label: "RPG"}, {Value: "4", Label: "Racing"}, {Value: "5", Label: "Puzzle"}}),
		choiceFilterField("Platform", "platform", entity.ViewFilterOperatorEq, "7", []exposedFilterOption{{Value: "7", Label: "PC"}}),
	}}
	var buf bytes.Buffer
	if err := ts.RenderNamed(&buf, "view_exposed_filters", data, "dynamic/list.html"); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		`<legend class="form-label">Genre</legend>`,
		`type="checkbox" name="genre" id="ef-genre-0" value="3" checked>`,
		`type="checkbox" name="genre" id="ef-genre-1" value="4">`,
		`type="checkbox" name="genre" id="ef-genre-2" value="5" checked>`,
		`<select name="platform" id="ef-platform"`,
		`<option value="7" selected>PC</option>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in:\n%s", want, out)
		}
	}
}

// TestExposedSortRender - the visitors' choice of sort is a select with
// no "All" option: one of the criteria always applies.
func TestExposedSortRender(t *testing.T) {
	ts := service.NewTemplateService(false)
	data := map[string]any{"viewURLPath": "games", "exposedFilters": []exposedFilterFormField{
		{Label: "Sort by", Param: "sort", Kind: exposedFilterKindSort, Value: "title",
			Options: []exposedFilterOption{{Value: "new", Label: "Newest"}, {Value: "title", Label: "By title"}}},
	}}
	var buf bytes.Buffer
	if err := ts.RenderNamed(&buf, "view_exposed_filters", data, "dynamic/list.html"); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		`<label for="ef-sort" class="form-label">Sort by</label>`,
		`<select name="sort" id="ef-sort" class="form-select form-select-sm"> <option value="new" >Newest</option>`,
		`<option value="title" selected>By title</option>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in:\n%s", want, out)
		}
	}
}
