package entity

import (
	"encoding/json"
	"testing"
	"time"
)

// TestDateValueJSON - a date without a time and an end is a plain date's
// string in a snapshot (comparable with the field's older snapshots), one
// with them an object; both read back.
func TestDateValueJSON(t *testing.T) {
	day := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		v    DateValue
		want string
	}{
		{DateValue{Start: day}, `"2026-11-12T00:00:00Z"`},
		{DateValue{Start: day.Add(19 * time.Hour), End: day.Add(21 * time.Hour), WithTime: true}, `{"start":"2026-11-12T19:00:00Z","end":"2026-11-12T21:00:00Z","time":true}`},
		{DateValue{Start: day, End: day.AddDate(0, 0, 3)}, `{"start":"2026-11-12T00:00:00Z","end":"2026-11-15T00:00:00Z"}`},
	} {
		raw, err := json.Marshal(tc.v)
		if err != nil || string(raw) != tc.want {
			t.Errorf("Marshal(%+v) = %s %v, want %s", tc.v, raw, err, tc.want)
		}
		var back DateValue
		if err := json.Unmarshal(raw, &back); err != nil || !back.Start.Equal(tc.v.Start) || !back.End.Equal(tc.v.End) || back.WithTime != tc.v.WithTime {
			t.Errorf("back %+v %v, want %+v", back, err, tc.v)
		}
	}
}

// TestDateFieldValues - a date field with a time or an end holds a
// DateValue (snapshots of it decode so, its old plain ones too), a plain
// one a time.Time (an old range's start); DateStart reads both.
func TestDateFieldValues(t *testing.T) {
	plain := FieldInstanceDef{ID: 1, MachineName: "premiere", FieldType: FieldTypeDate}
	rich := FieldInstanceDef{ID: 1, MachineName: "premiere", FieldType: FieldTypeDate, Config: map[string]any{"date_end": "optional"}}
	if plain.DateRich() || !rich.DateRich() {
		t.Fatal("DateRich")
	}
	day := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	rng := DateValue{Start: day, End: day.AddDate(0, 0, 3)}
	snap := SnapshotOf(DynamicContent{Fields: FieldInstanceDefs{rich}, Values: map[int]any{1: rng}})
	values, _ := snap.Values(FieldInstanceDefs{rich})
	if got, ok := values[1].(DateValue); !ok || !got.End.Equal(rng.End) {
		t.Errorf("rich back: %#v", values[1])
	}
	values, _ = snap.Values(FieldInstanceDefs{plain})
	if got, ok := values[1].(time.Time); !ok || !got.Equal(day) {
		t.Errorf("plain back from a range: %#v", values[1])
	}
	old := SnapshotOf(DynamicContent{Fields: FieldInstanceDefs{plain}, Values: map[int]any{1: day}})
	values, _ = old.Values(FieldInstanceDefs{rich})
	if got, ok := values[1].(DateValue); !ok || !got.Start.Equal(day) || got.HasEnd() {
		t.Errorf("rich back from a plain date: %#v", values[1])
	}
	for _, v := range []any{day, rng} {
		if start, ok := DateStart(v); !ok || !start.Equal(day) {
			t.Errorf("DateStart(%#v) = %v %v", v, start, ok)
		}
	}
	if _, ok := DateStart("2026-11-12"); ok {
		t.Error("DateStart of a string")
	}
}

// TestDateValueState - a date without a time ends with its last day (in
// the zone's today), one with a time at its end (else its start); going
// on - started and not ended.
func TestDateValueState(t *testing.T) {
	msk := time.FixedZone("MSK", 3*3600)
	// 23:30 UTC on the 12th - already the 13th in Moscow.
	now := time.Date(2026, 11, 12, 23, 30, 0, 0, time.UTC)
	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }
	at := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, time.UTC) }
	for name, tc := range map[string]struct {
		v              DateValue
		ended, ongoing bool
	}{
		"yesterday's date":    {DateValue{Start: day(12)}, true, false},
		"today's date":        {DateValue{Start: day(13)}, false, true},
		"tomorrow's date":     {DateValue{Start: day(14)}, false, false},
		"days through today":  {DateValue{Start: day(10), End: day(13)}, false, true},
		"days ended":          {DateValue{Start: day(10), End: day(12)}, true, false},
		"a time passed":       {DateValue{Start: at(12, 20), WithTime: true}, true, false},
		"a time to come":      {DateValue{Start: at(13, 1), WithTime: true}, false, false},
		"times going on":      {DateValue{Start: at(12, 22), End: at(13, 1), WithTime: true}, false, true},
		"times ended at last": {DateValue{Start: at(12, 22), End: at(12, 23), WithTime: true}, true, false},
	} {
		if got := tc.v.Ended(now, msk); got != tc.ended {
			t.Errorf("%s: Ended = %v", name, got)
		}
		if got := tc.v.Ongoing(now, msk); got != tc.ongoing {
			t.Errorf("%s: Ongoing = %v", name, got)
		}
	}
}

// TestDateListValues - several dates go into a snapshot as a list and
// come back so; a single date read for a field that now has several is
// one of them, and DateValues reads every form of a value.
func TestDateListValues(t *testing.T) {
	list := FieldInstanceDef{ID: 1, MachineName: "shows", FieldType: FieldTypeDate, Config: map[string]any{"multiple": true}}
	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }
	dates := []DateValue{{Start: day(12)}, {Start: day(14), End: day(15)}}
	snap := SnapshotOf(DynamicContent{Fields: FieldInstanceDefs{list}, Values: map[int]any{1: dates}})
	values, _ := snap.Values(FieldInstanceDefs{list})
	if got, ok := values[1].([]DateValue); !ok || len(got) != 2 || !got[1].End.Equal(day(15)) {
		t.Errorf("list back: %#v", values[1])
	}
	old := SnapshotOf(DynamicContent{Fields: FieldInstanceDefs{{ID: 1, MachineName: "shows", FieldType: FieldTypeDate}}, Values: map[int]any{1: day(12)}})
	values, _ = old.Values(FieldInstanceDefs{list})
	if got, ok := values[1].([]DateValue); !ok || len(got) != 1 || !got[0].Start.Equal(day(12)) {
		t.Errorf("a date read as several: %#v", values[1])
	}
	for _, v := range []any{day(12), DateValue{Start: day(12)}, dates} {
		if got := DateValues(v); len(got) == 0 || !got[0].Start.Equal(day(12)) {
			t.Errorf("DateValues(%#v) = %v", v, got)
		}
	}
	if !list.DateList() || !FieldTypeDate.AllowsMultiple() {
		t.Error("a date field with several dates")
	}
}
