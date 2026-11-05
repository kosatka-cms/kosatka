package form

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestDateValueOf - a date field's inputs: a time in the zone (the value
// in UTC), no time - the whole day; an end on another day, an end time
// alone - the same day; what the field asks for missing, an end before
// the start, a part without its pair; the parts the field doesn't have
// ignored; the inputs back from a value.
func TestDateValueOf(t *testing.T) {
	ctx := context.Background()
	moscow := time.FixedZone("MSK", 3*3600)
	both := entity.FieldInstanceDef{Label: "When", FieldType: entity.FieldTypeDate, Config: map[string]any{"date_time": "optional", "date_end": "optional"}}
	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }
	for name, tc := range map[string]struct {
		field entity.FieldInstanceDef
		in    DateInput
		want  entity.DateValue
		ok    bool
		msg   string
	}{
		"time in the zone": {both, DateInput{Date: "2026-11-12", Time: "19:00", EndTime: "21:30"},
			entity.DateValue{Start: day(12).Add(16 * time.Hour), End: day(12).Add(18*time.Hour + 30*time.Minute), WithTime: true}, true, ""},
		"days":          {both, DateInput{Date: "2026-11-12", EndDate: "2026-11-15"}, entity.DateValue{Start: day(12), End: day(15)}, true, ""},
		"one day":       {both, DateInput{Date: "2026-11-12", EndDate: "2026-11-12"}, entity.DateValue{Start: day(12)}, true, ""},
		"none":          {both, DateInput{}, entity.DateValue{}, false, ""},
		"no start":      {both, DateInput{EndDate: "2026-11-15"}, entity.DateValue{}, false, "When: enter the start date."},
		"end before":    {both, DateInput{Date: "2026-11-15", EndDate: "2026-11-12"}, entity.DateValue{}, true, "When: the end is before the start."},
		"no end time":   {both, DateInput{Date: "2026-11-12", Time: "19:00", EndDate: "2026-11-13"}, entity.DateValue{}, true, "When: enter the end time."},
		"no start time": {both, DateInput{Date: "2026-11-12", EndTime: "21:00"}, entity.DateValue{}, true, "When: enter the start time."},
		"time required": {entity.FieldInstanceDef{Label: "When", FieldType: entity.FieldTypeDate, Config: map[string]any{"date_time": "required"}},
			DateInput{Date: "2026-11-12"}, entity.DateValue{}, true, "When: enter the time."},
		"end required": {entity.FieldInstanceDef{Label: "When", FieldType: entity.FieldTypeDate, Config: map[string]any{"date_end": "required"}},
			DateInput{Date: "2026-11-12"}, entity.DateValue{}, true, "When: enter the end."},
		"parts ignored": {entity.FieldInstanceDef{Label: "When", FieldType: entity.FieldTypeDate, Config: map[string]any{"date_end": "optional"}},
			DateInput{Date: "2026-11-12", Time: "19:00", EndDate: "2026-11-13", EndTime: "20:00"}, entity.DateValue{Start: day(12), End: day(13)}, true, ""},
	} {
		got, ok, msg := DateValueOf(ctx, tc.field, tc.in, moscow)
		if ok != tc.ok || msg != tc.msg || !got.Start.Equal(tc.want.Start) || !got.End.Equal(tc.want.End) || got.WithTime != tc.want.WithTime {
			t.Errorf("%s: %+v %v %q, want %+v %v %q", name, got, ok, msg, tc.want, tc.ok, tc.msg)
		}
	}
	in := DateInputOf(entity.DateValue{Start: day(12).Add(16 * time.Hour), End: day(12).Add(18 * time.Hour), WithTime: true}, moscow)
	if in != (DateInput{Date: "2026-11-12", Time: "19:00", EndDate: "2026-11-12", EndTime: "21:00"}) {
		t.Errorf("DateInputOf = %+v", in)
	}
}

// TestParseDateInputs - a field with several dates: a row each, its
// inputs paired by their order; an empty row left out.
func TestParseDateInputs(t *testing.T) {
	form := url.Values{
		"field_3":          {"2026-11-12", "", "2026-11-14"},
		"field_3_time":     {"19:00", "", ""},
		"field_3_end":      {"", "", "2026-11-15"},
		"field_3_end_time": {"21:00", "", ""},
	}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	got := ParseDateInputs(r, entity.FieldInstanceDef{ID: 3})
	want := []DateInput{{Date: "2026-11-12", Time: "19:00", EndTime: "21:00"}, {Date: "2026-11-14", EndDate: "2026-11-15"}}
	if !slices.Equal(got, want) {
		t.Errorf("ParseDateInputs = %+v, want %+v", got, want)
	}
}
