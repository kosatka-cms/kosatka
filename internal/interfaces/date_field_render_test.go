package interfaces

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
)

// TestDateRangeOnNodePage - a date field with a time and an end shows
// its range on the content page: one day with its times once, several
// days shortened.
func TestDateRangeOnNodePage(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	field := entity.FieldInstanceDef{ID: 1, MachineName: "when", Label: "When", FieldType: entity.FieldTypeDate, ShowOnDetail: true, ShowLabel: true,
		DateFormat: "missing", Config: map[string]any{"date_time": "optional", "date_end": "optional"}}
	at := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, time.UTC) }
	for _, tc := range []struct {
		v    entity.DateValue
		want string
	}{
		{entity.DateValue{Start: at(12, 19), End: at(12, 21), WithTime: true}, "12.11.2026, 19:00-21:00"},
		{entity.DateValue{Start: at(12, 0), End: at(15, 0)}, "12-15.11.2026"},
	} {
		dc := &entity.DynamicContent{Fields: entity.FieldInstanceDefs{field}, Values: map[int]any{1: tc.v}}
		if out := renderNodeSections(t, dc); !strings.Contains(out, `<span class="field-list">When:</span> `+tc.want) {
			t.Errorf("no %q in %s", tc.want, out)
		}
	}
}

// TestDateRangeInForm - the content form's date field with a time and an
// end: the start's date and time, the end's; one without an end has no
// end inputs, a plain one stays one date input.
func TestDateRangeInForm(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	render := func(field entity.FieldInstanceDef, value any) string {
		data := map[string]any{
			"title": "Editing: Show", "contentType": &entity.ContentTypeDef{MachineName: "event", Label: "Event"},
			"dc": &entity.DynamicContent{}, "node": &entity.Node{}, "form": form.DynamicForm{Title: "Show"},
			"formSections": buildFormSections(entity.FieldInstanceDefs{field}), "values": map[int]any{field.ID: value}, "timeZone": "Europe/Moscow",
		}
		for _, k := range []string{"taxonomyOptions", "taxonomyHierarchyLabels", "multiTaxonomyFieldTerms", "referenceFieldNodes",
			"imageFieldURLs", "imageFieldAlts", "imageFieldItems", "fileFieldLinks", "textFormatIDs", "userFieldUsers", "tagPickers", "blockFields"} {
			data[k] = map[int]any{}
		}
		return renderNodeEdit(t, data)
	}
	both := entity.FieldInstanceDef{ID: 4, Label: "When", FieldType: entity.FieldTypeDate, Config: map[string]any{"date_time": "required", "date_end": "optional"}}
	out := render(both, form.DateInput{Date: "2026-11-12", Time: "19:00", EndDate: "2026-11-12", EndTime: "21:30"})
	for _, want := range []string{
		`id="field_4" name="field_4" value="2026-11-12"`,
		`name="field_4_time" value="19:00"`,
		`required`,
		`name="field_4_end" value="2026-11-12"`,
		`name="field_4_end_time" value="21:30"`,
		`Time zone: Europe/Moscow.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	timeOnly := entity.FieldInstanceDef{ID: 5, Label: "At", FieldType: entity.FieldTypeDate, Config: map[string]any{"date_time": "optional"}}
	if out := render(timeOnly, nil); strings.Contains(out, "field_5_end") || !strings.Contains(out, `name="field_5_time" value=""`) || !strings.Contains(out, "No time - the whole day.") {
		t.Errorf("time only: %s", out)
	}
	plain := entity.FieldInstanceDef{ID: 6, Label: "Day", FieldType: entity.FieldTypeDate}
	if out := render(plain, "2026-11-12"); !strings.Contains(out, `<input type="date" id="field_6" name="field_6" value="2026-11-12" class="form-control">`) || strings.Contains(out, "field_6_time") {
		t.Errorf("plain: %s", out)
	}
}

// TestDateListOnNodePage - a field with several dates on the content
// page: all in order, the ended marked "past", the current "now on"; the
// nearest not ended and how many more; the upcoming only.
func TestDateListOnNodePage(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	dates := []entity.DateValue{
		{Start: today.AddDate(0, 0, -10)},                             // ended
		{Start: today.AddDate(0, 0, 30)},                              // to come, later
		{Start: today.AddDate(0, 0, -1), End: today.AddDate(0, 0, 1)}, // going on
		{Start: today.AddDate(0, 0, 5)},                               // to come, nearer
	}
	field := entity.FieldInstanceDef{ID: 1, MachineName: "shows", Label: "Shows", FieldType: entity.FieldTypeDate, ShowOnDetail: true, ShowLabel: true,
		Config: map[string]any{"multiple": true}}
	format := func(d time.Time) string { return d.Format("02.01.2006") }
	render := func(layout entity.FieldLayout) string {
		f := field
		f.DetailLayout = layout
		return renderNodeSections(t, &entity.DynamicContent{Fields: entity.FieldInstanceDefs{f}, Values: map[int]any{1: dates}})
	}

	out := render(entity.FieldLayoutAll)
	for _, want := range []string{
		`<li class="field-dates-item is-past">` + format(dates[0].Start) + ` <span class="field-dates-badge">past</span></li>`,
		`<li class="field-dates-item is-ongoing">`,
		`<span class="field-dates-badge">now on</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("all: no %s in %s", want, out)
		}
	}
	if strings.Count(out, "field-dates-item") != 4 {
		t.Errorf("all: want 4 dates: %s", out)
	}

	out = render(entity.FieldLayoutNearest)
	if strings.Count(out, "field-dates-item") != 1 || !strings.Contains(out, "is-ongoing") || !strings.Contains(out, "and 2 more dates") {
		t.Errorf("nearest: %s", out)
	}
	out = render(entity.FieldLayoutUpcoming)
	if strings.Count(out, "field-dates-item") != 3 || strings.Contains(out, "is-past") || strings.Index(out, format(dates[3].Start)) > strings.Index(out, format(dates[1].Start)) {
		t.Errorf("upcoming (in order of their starts): %s", out)
	}
}

// TestDateListInForm - a field with several dates in the content form: a
// row each with its inputs and ↑/↓/✕, "Add a date" up to the limit.
func TestDateListInForm(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	field := entity.FieldInstanceDef{ID: 7, Label: "Shows", FieldType: entity.FieldTypeDate,
		Config: map[string]any{"multiple": true, "date_time": "optional"}, WidgetConfig: map[string]any{"max_values": float64(4)}}
	data := map[string]any{
		"title": "Editing: Show", "contentType": &entity.ContentTypeDef{MachineName: "event", Label: "Event"},
		"dc": &entity.DynamicContent{}, "node": &entity.Node{}, "form": form.DynamicForm{Title: "Show"}, "timeZone": "UTC",
		"formSections": buildFormSections(entity.FieldInstanceDefs{field}),
		"values":       map[int]any{7: []form.DateInput{{Date: "2026-11-12", Time: "19:00"}, {Date: "2026-11-14"}}},
	}
	for _, k := range []string{"taxonomyOptions", "taxonomyHierarchyLabels", "multiTaxonomyFieldTerms", "referenceFieldNodes",
		"imageFieldURLs", "imageFieldAlts", "imageFieldItems", "fileFieldLinks", "textFormatIDs", "userFieldUsers", "tagPickers", "blockFields"} {
		data[k] = map[int]any{}
	}
	out := renderNodeEdit(t, data)
	for _, want := range []string{
		`data-field-rows data-max="4"`,
		`id="field_7" name="field_7" value="2026-11-12"`,
		`name="field_7_time" value="19:00"`,
		`name="field_7" value="2026-11-14"`,
		`data-field-row-remove`,
		`<template data-field-row-template>`,
		`data-field-row-add>+ Add a date</button>`,
		`At most 4 dates.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}
