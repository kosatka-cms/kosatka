package entity

import (
	"encoding/json"
	"slices"
	"time"
)

// DateValue - the value of a date field with a time or an end
// (FieldInstanceDef.DateRich): its start, its end (zero - none) and
// whether they have a time (WithTime: instants in UTC, shown in the
// page's zone; without one, dates at UTC midnight, shown as they are).
// A plain date field's value is a time.Time, a field with several
// dates' - a []DateValue in the editor's order (DateStart, AsDateValue,
// DateValues read them all).
type DateValue struct {
	Start    time.Time
	End      time.Time
	WithTime bool
}

// HasEnd - the value has an end.
func (v DateValue) HasEnd() bool { return !v.End.IsZero() }

// dateValueJSON - DateValue in a revision's snapshot.
type dateValueJSON struct {
	Start    string `json:"start"`
	End      string `json:"end,omitempty"`
	WithTime bool   `json:"time,omitempty"`
}

// MarshalJSON - a date without a time and an end as a plain date field's
// (the instant as a string: the snapshots of a field that gained the
// settings stay comparable), else an object.
func (v DateValue) MarshalJSON() ([]byte, error) {
	start := v.Start.UTC().Format(time.RFC3339Nano)
	if !v.WithTime && !v.HasEnd() {
		return json.Marshal(start)
	}
	out := dateValueJSON{Start: start, WithTime: v.WithTime}
	if v.HasEnd() {
		out.End = v.End.UTC().Format(time.RFC3339Nano)
	}
	return json.Marshal(out)
}

// UnmarshalJSON reads both forms of MarshalJSON.
func (v *DateValue) UnmarshalJSON(raw []byte) error {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return err
		}
		*v = DateValue{Start: t}
		return nil
	}
	var o dateValueJSON
	if err := json.Unmarshal(raw, &o); err != nil {
		return err
	}
	start, err := time.Parse(time.RFC3339Nano, o.Start)
	if err != nil {
		return err
	}
	*v = DateValue{Start: start, WithTime: o.WithTime}
	if o.End != "" {
		if v.End, err = time.Parse(time.RFC3339Nano, o.End); err != nil {
			return err
		}
	}
	return nil
}

// DateStart - the start of a date field's value (a time.Time, a
// DateValue, the first of a []DateValue); false - none.
func DateStart(v any) (time.Time, bool) {
	d, ok := AsDateValue(v)
	return d.Start, ok
}

// AsDateValue - a date field's value as a DateValue (a plain date - its
// start; several - the first); false - none.
func AsDateValue(v any) (DateValue, bool) {
	switch d := v.(type) {
	case time.Time:
		return DateValue{Start: d}, !d.IsZero()
	case DateValue:
		return d, !d.Start.IsZero()
	case []DateValue:
		if len(d) > 0 {
			return d[0], !d[0].Start.IsZero()
		}
	}
	return DateValue{}, false
}

// DateValues - a date field's value as its dates (one or several, in
// the editor's order); none for anything else.
func DateValues(v any) []DateValue {
	if list, ok := v.([]DateValue); ok {
		return list
	}
	if d, ok := AsDateValue(v); ok {
		return []DateValue{d}
	}
	return nil
}

// Today - the date of now in zone, at UTC midnight: what a date without
// a time (DateValue.WithTime false) compares with.
func Today(now time.Time, zone *time.Location) time.Time {
	y, m, d := now.In(zone).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// last - the value's last day or moment: its end, else its start.
func (v DateValue) last() time.Time {
	if v.HasEnd() {
		return v.End
	}
	return v.Start
}

// Ended - the value is over at now (the date in zone for a value without
// a time): its end (else its start) has passed - a date without a time at
// the end of that day, one with a time at that moment.
func (v DateValue) Ended(now time.Time, zone *time.Location) bool {
	if v.WithTime {
		return !v.last().After(now)
	}
	return v.last().Before(Today(now, zone))
}

// Ongoing - the value has started and not ended at now.
func (v DateValue) Ongoing(now time.Time, zone *time.Location) bool {
	if v.Ended(now, zone) {
		return false
	}
	if v.WithTime {
		return !v.Start.After(now)
	}
	return !v.Start.After(Today(now, zone))
}

// DatePart - whether a date field asks for a part of its value: the time
// or the end.
type DatePart string

const (
	// DatePartNone - it doesn't: no time (a date), no end (one day).
	DatePartNone DatePart = ""
	// DatePartOptional - a value may have it.
	DatePartOptional DatePart = "optional"
	// DatePartRequired - every value has it.
	DatePartRequired DatePart = "required"
)

// DateParts - the choices of a DatePart setting, in the order shown.
var DateParts = []DatePart{DatePartNone, DatePartOptional, DatePartRequired}

// ParseDatePart - s as a DatePart; unknown - none.
func ParseDatePart(s string) DatePart {
	if slices.Contains(DateParts, DatePart(s)) {
		return DatePart(s)
	}
	return DatePartNone
}

// DateTime - FieldTypeDate: whether its values have a time (storage
// config "date_time").
func (f FieldInstanceDef) DateTime() DatePart {
	s, _ := f.Config["date_time"].(string)
	return ParseDatePart(s)
}

// DateEnd - FieldTypeDate: whether its values have an end (storage
// config "date_end").
func (f FieldInstanceDef) DateEnd() DatePart {
	s, _ := f.Config["date_end"].(string)
	return ParseDatePart(s)
}

// DateRich - a date field whose values may have a time or an end: its
// value is a DateValue (several - a []DateValue, whatever the parts), a
// plain one's a time.Time.
func (f FieldInstanceDef) DateRich() bool {
	return f.FieldType == FieldTypeDate && (f.DateTime() != DatePartNone || f.DateEnd() != DatePartNone)
}

// DateList - a date field with several dates: its value is a []DateValue.
func (f FieldInstanceDef) DateList() bool {
	return f.FieldType == FieldTypeDate && f.Multiple()
}

// TimeFormat - FieldTypeDate: the date format machine name its times are
// shown in (instance config "time_format"); "" - hours and minutes.
func (f FieldInstanceDef) TimeFormat() string {
	s, _ := f.WidgetConfig["time_format"].(string)
	return s
}

// DateRangeFull - FieldTypeDate: a range shows both dates whole, never
// shortened to the common month and year once (instance config
// "date_range_full").
func (f FieldInstanceDef) DateRangeFull() bool {
	b, _ := f.WidgetConfig["date_range_full"].(bool)
	return b
}
