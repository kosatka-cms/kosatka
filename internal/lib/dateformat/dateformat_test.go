package dateformat

import (
	"os"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/i18ntest"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestMain loads month names from the engine's catalogs.
func TestMain(m *testing.M) {
	tr, err := i18n.Load(i18ntest.FS)
	if err != nil {
		panic(err)
	}
	i18n.SetTranslator(tr)
	os.Exit(m.Run())
}

func TestFormat(t *testing.T) {
	d := time.Date(2027, 12, 3, 9, 5, 7, 0, time.UTC)
	for pattern, want := range map[string]string{
		"d.m.Y":       "03.12.2027",
		"j F Y":       "3 декабря 2027",
		"f Y":         "декабрь 2027",
		"j M y":       "3 дек 27",
		"n/j":         "12/3",
		"d.m.Y H:i:s": "03.12.2027 09:05:07",
		"G:i":         "9:05",
		`j F Y \г.`:   "3 декабря 2027 г.",
		`\d\a\y d`:    "day 03",
		"":            "",
		`Y\`:          "2027",
	} {
		if got := Format(d, pattern, "ru"); got != want {
			t.Errorf("Format(%q) = %q, want %q", pattern, got, want)
		}
	}
	if got := Format(time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC), "F f M", "ru"); got != "января январь янв" {
		t.Errorf("January: %q", got)
	}
	if got := Format(time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC), "j F, f, M", "en"); got != "1 May, May, May" {
		t.Errorf("English: %q", got)
	}
}

// TestFormatWeekdayAnd12Hours - the weekday (its name, short name, ISO
// number) and the 12-hour clock with AM/PM, midnight and noon as 12.
func TestFormatWeekdayAnd12Hours(t *testing.T) {
	friday := time.Date(2027, 12, 31, 19, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		t             time.Time
		pattern, lang string
		want          string
	}{
		{friday, "D, j F", "ru", "пт, 31 декабря"},
		{friday, "l", "ru", "пятница"},
		{friday, "l, D", "en", "Friday, Fri"},
		{friday, "N", "ru", "5"},
		{time.Date(2027, 12, 26, 0, 0, 0, 0, time.UTC), "N", "ru", "7"},
		{friday, "g:i a", "en", "7:30 pm"},
		{friday, "h:i A", "en", "07:30 PM"},
		{time.Date(2027, 1, 1, 0, 5, 0, 0, time.UTC), "g:i A", "en", "12:05 AM"},
		{time.Date(2027, 1, 1, 12, 0, 0, 0, time.UTC), "h a", "en", "12 pm"},
		{friday, `\l\D`, "ru", "lD"},
	} {
		if got := Format(tc.t, tc.pattern, tc.lang); got != tc.want {
			t.Errorf("Format(%v, %q, %s) = %q, want %q", tc.t, tc.pattern, tc.lang, got, tc.want)
		}
	}
	if full, short := WeekdayNames("ru"); len(full) != 7 || full[1] != "понедельник" || short[0] != "вс" {
		t.Errorf("WeekdayNames: %v %v", full, short)
	}
}

func TestLettersIn(t *testing.T) {
	for _, l := range LettersIn("ru") {
		switch l.Letter {
		case "F":
			if l.Example != "декабря" || l.Meaning != "месяц словом, родительный падеж" {
				t.Errorf("F: %+v", l)
			}
		case "M":
			if l.Example != "дек" {
				t.Errorf("M: %+v", l)
			}
		case "D":
			if l.Example != "пт" {
				t.Errorf("D: %+v", l)
			}
		}
	}
}

// TestRange - a date with its time; a range of one day with the times
// once; a range without times shortened to what the dates share (the
// day, the month and the year in that order) or whole; with times or
// another year - whole.
func TestRange(t *testing.T) {
	at := func(y int, m time.Month, d, h, mi int) time.Time { return time.Date(y, m, d, h, mi, 0, 0, time.UTC) }
	day := func(y int, m time.Month, d int) time.Time { return at(y, m, d, 0, 0) }
	for _, tc := range []struct {
		start, end      time.Time
		date            string
		withTime, short bool
		want            string
	}{
		{day(2026, 11, 12), time.Time{}, "j F Y", false, true, "12 ноября 2026"},
		{at(2026, 11, 12, 19, 0), time.Time{}, "j F Y", true, true, "12 ноября 2026, 19:00"},
		{at(2026, 11, 12, 19, 0), at(2026, 11, 12, 21, 30), "D, j F Y", true, true, "чт, 12 ноября 2026, 19:00-21:30"},
		{day(2026, 11, 12), day(2026, 11, 15), "j F Y", false, true, "12-15 ноября 2026"},
		{day(2026, 11, 28), day(2026, 12, 3), "j F Y", false, true, "28 ноября - 3 декабря 2026"},
		{day(2026, 12, 28), day(2027, 1, 3), "j F Y", false, true, "28 декабря 2026 - 3 января 2027"},
		{day(2026, 11, 12), day(2026, 11, 15), "d.m.Y", false, true, "12-15.11.2026"},
		{day(2026, 11, 28), day(2026, 12, 3), "d.m.Y", false, true, "28.11 - 03.12.2026"},
		{day(2026, 11, 12), day(2026, 11, 15), "D, j F Y", false, true, "чт, 12 - вс, 15 ноября 2026"},
		{day(2026, 11, 12), day(2026, 11, 15), "Y-m-d", false, true, "2026-11-12 - 2026-11-15"},
		{day(2026, 11, 12), day(2026, 11, 15), "j F Y", false, false, "12 ноября 2026 - 15 ноября 2026"},
		{at(2026, 11, 12, 22, 0), at(2026, 11, 13, 2, 0), "j F Y", true, true, "12 ноября 2026, 22:00 - 13 ноября 2026, 02:00"},
		{day(2026, 11, 12), day(2026, 11, 12), "j F Y", false, true, "12 ноября 2026"},
		{day(2026, 11, 12), day(2026, 11, 15), `j F Y \г.`, false, true, "12-15 ноября 2026 г."},
	} {
		if got := Range(tc.start, tc.end, tc.date, "H:i", tc.withTime, tc.short, "ru"); got != tc.want {
			t.Errorf("Range(%v, %v, %q) = %q, want %q", tc.start, tc.end, tc.date, got, tc.want)
		}
	}
}
