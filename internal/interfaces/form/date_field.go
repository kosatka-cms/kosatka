package form

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// DateInput - the inputs of a date field with a time or an end
// (entity.FieldInstanceDef.DateRich) in the content form: the start's
// date and time, the end's ("" - empty).
type DateInput struct {
	Date, Time, EndDate, EndTime string
}

// DateInputOf - the inputs of value d, its times in zone.
func DateInputOf(d entity.DateValue, zone *time.Location) DateInput {
	if d.Start.IsZero() {
		return DateInput{}
	}
	start, end := d.Start, d.End
	if d.WithTime {
		start = start.In(zone)
		if d.HasEnd() {
			end = end.In(zone)
		}
	}
	in := DateInput{Date: start.Format("2006-01-02")}
	if d.WithTime {
		in.Time = start.Format("15:04")
	}
	if d.HasEnd() {
		in.EndDate = end.Format("2006-01-02")
		if d.WithTime {
			in.EndTime = end.Format("15:04")
		}
	}
	return in
}

// ParseDateInput - a date field's inputs as submitted: field_<id> (the
// start's date), field_<id>_time, field_<id>_end, field_<id>_end_time.
func ParseDateInput(r *http.Request, fieldID int) DateInput {
	name := fmt.Sprintf("field_%d", fieldID)
	return DateInput{
		Date:    strings.TrimSpace(r.FormValue(name)),
		Time:    strings.TrimSpace(r.FormValue(name + "_time")),
		EndDate: strings.TrimSpace(r.FormValue(name + "_end")),
		EndTime: strings.TrimSpace(r.FormValue(name + "_end_time")),
	}
}

// ParseDateInputs - the rows of a date field with several dates as
// submitted: the same inputs as ParseDateInput, one of each per row in
// the form's order (the parts the field has); an empty row left out.
func ParseDateInputs(r *http.Request, field entity.FieldInstanceDef) []DateInput {
	name := fmt.Sprintf("field_%d", field.ID)
	at := func(list []string, i int) string {
		if i < len(list) {
			return strings.TrimSpace(list[i])
		}
		return ""
	}
	dates, times, ends, endTimes := r.Form[name], r.Form[name+"_time"], r.Form[name+"_end"], r.Form[name+"_end_time"]
	var out []DateInput
	for i := range dates {
		in := DateInput{Date: at(dates, i), Time: at(times, i), EndDate: at(ends, i), EndTime: at(endTimes, i)}
		if in != (DateInput{}) {
			out = append(out, in)
		}
	}
	return out
}

// DateValueOf - the value of a date field's inputs in (times in zone):
// false - none (no start); a message - what's wrong ("" - nothing). A
// part the field doesn't have is ignored; an end time without an end
// date ends on the start's day. A plain field's (no time, no end) - its
// date.
func DateValueOf(ctx context.Context, field entity.FieldInstanceDef, in DateInput, zone *time.Location) (entity.DateValue, bool, string) {
	if field.DateTime() == entity.DatePartNone {
		in.Time, in.EndTime = "", ""
	}
	if field.DateEnd() == entity.DatePartNone {
		in.EndDate, in.EndTime = "", ""
	}
	if in.EndDate == "" && in.EndTime != "" {
		in.EndDate = in.Date
	}
	if in.Date == "" {
		if in.Time != "" || in.EndDate != "" {
			return entity.DateValue{}, false, i18n.T(ctx, "@field: enter the start date.", "@field", field.Label)
		}
		return entity.DateValue{}, false, ""
	}
	withTime := in.Time != ""
	switch {
	case field.DateTime() == entity.DatePartRequired && !withTime:
		return entity.DateValue{}, true, i18n.T(ctx, "@field: enter the time.", "@field", field.Label)
	case field.DateEnd() == entity.DatePartRequired && in.EndDate == "":
		return entity.DateValue{}, true, i18n.T(ctx, "@field: enter the end.", "@field", field.Label)
	case withTime && in.EndDate != "" && in.EndTime == "":
		return entity.DateValue{}, true, i18n.T(ctx, "@field: enter the end time.", "@field", field.Label)
	case !withTime && in.EndTime != "":
		return entity.DateValue{}, true, i18n.T(ctx, "@field: enter the start time.", "@field", field.Label)
	}
	parse := func(date, clock string) (time.Time, error) {
		if !withTime {
			return time.Parse("2006-01-02", date)
		}
		return time.ParseInLocation("2006-01-02 15:04", date+" "+clock, zone)
	}
	start, err := parse(in.Date, in.Time)
	if err != nil {
		return entity.DateValue{}, true, i18n.T(ctx, "Invalid date format (use YYYY-MM-DD)")
	}
	d := entity.DateValue{Start: start.UTC(), WithTime: withTime}
	if in.EndDate != "" {
		end, err := parse(in.EndDate, in.EndTime)
		if err != nil {
			return entity.DateValue{}, true, i18n.T(ctx, "Invalid date format (use YYYY-MM-DD)")
		}
		if end.Before(start) {
			return entity.DateValue{}, true, i18n.T(ctx, "@field: the end is before the start.", "@field", field.Label)
		}
		if !end.Equal(start) {
			d.End = end.UTC()
		}
	}
	return d, true, ""
}
