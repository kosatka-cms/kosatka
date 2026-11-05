// Package dateformat formats dates by PHP date()-style patterns
// (entity.DateFormat). Month names come from the interface catalogs
// (pkg/i18n) in three forms, each its own message context: the name
// inside a date (F; e.g. a genitive form in some languages), the
// standalone name (f) and the abbreviation (M); weekday names in two
// (l, D).
package dateformat

import (
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Message contexts of the month names.
const (
	ContextMonthInDate  = "Month name in a date"
	ContextMonth        = "Standalone month name"
	ContextMonthShort   = "Abbreviated month name"
	ContextWeekday      = "Weekday name"
	ContextWeekdayShort = "Abbreviated weekday name"
)

// The weekdays from Sunday, as time.Weekday counts them.
var weekdays = [...]string{
	i18n.NC("Weekday name", "Sunday"), i18n.NC("Weekday name", "Monday"),
	i18n.NC("Weekday name", "Tuesday"), i18n.NC("Weekday name", "Wednesday"),
	i18n.NC("Weekday name", "Thursday"), i18n.NC("Weekday name", "Friday"),
	i18n.NC("Weekday name", "Saturday"),
}

var weekdaysShort = [...]string{
	i18n.NC("Abbreviated weekday name", "Sun"), i18n.NC("Abbreviated weekday name", "Mon"),
	i18n.NC("Abbreviated weekday name", "Tue"), i18n.NC("Abbreviated weekday name", "Wed"),
	i18n.NC("Abbreviated weekday name", "Thu"), i18n.NC("Abbreviated weekday name", "Fri"),
	i18n.NC("Abbreviated weekday name", "Sat"),
}

var monthsInDate = [...]string{
	i18n.NC("Month name in a date", "January"), i18n.NC("Month name in a date", "February"),
	i18n.NC("Month name in a date", "March"), i18n.NC("Month name in a date", "April"),
	i18n.NC("Month name in a date", "May"), i18n.NC("Month name in a date", "June"),
	i18n.NC("Month name in a date", "July"), i18n.NC("Month name in a date", "August"),
	i18n.NC("Month name in a date", "September"), i18n.NC("Month name in a date", "October"),
	i18n.NC("Month name in a date", "November"), i18n.NC("Month name in a date", "December"),
}

var monthsStandalone = [...]string{
	i18n.NC("Standalone month name", "January"), i18n.NC("Standalone month name", "February"),
	i18n.NC("Standalone month name", "March"), i18n.NC("Standalone month name", "April"),
	i18n.NC("Standalone month name", "May"), i18n.NC("Standalone month name", "June"),
	i18n.NC("Standalone month name", "July"), i18n.NC("Standalone month name", "August"),
	i18n.NC("Standalone month name", "September"), i18n.NC("Standalone month name", "October"),
	i18n.NC("Standalone month name", "November"), i18n.NC("Standalone month name", "December"),
}

var monthsShort = [...]string{
	i18n.NC("Abbreviated month name", "Jan"), i18n.NC("Abbreviated month name", "Feb"),
	i18n.NC("Abbreviated month name", "Mar"), i18n.NC("Abbreviated month name", "Apr"),
	i18n.NC("Abbreviated month name", "May"), i18n.NC("Abbreviated month name", "Jun"),
	i18n.NC("Abbreviated month name", "Jul"), i18n.NC("Abbreviated month name", "Aug"),
	i18n.NC("Abbreviated month name", "Sep"), i18n.NC("Abbreviated month name", "Oct"),
	i18n.NC("Abbreviated month name", "Nov"), i18n.NC("Abbreviated month name", "Dec"),
}

// MonthInDate - month m's name inside a date in lang.
func MonthInDate(lang string, m time.Month) string {
	return i18n.Current().Text(lang, ContextMonthInDate, monthsInDate[m-1])
}

// Month - month m's standalone name in lang.
func Month(lang string, m time.Month) string {
	return i18n.Current().Text(lang, ContextMonth, monthsStandalone[m-1])
}

// MonthShort - month m's abbreviated name in lang.
func MonthShort(lang string, m time.Month) string {
	return i18n.Current().Text(lang, ContextMonthShort, monthsShort[m-1])
}

// Weekday - weekday d's name in lang.
func Weekday(lang string, d time.Weekday) string {
	return i18n.Current().Text(lang, ContextWeekday, weekdays[d])
}

// WeekdayShort - weekday d's abbreviated name in lang.
func WeekdayShort(lang string, d time.Weekday) string {
	return i18n.Current().Text(lang, ContextWeekdayShort, weekdaysShort[d])
}

// WeekdayNames - the seven names of each form in lang from Sunday, for
// scripts that format dates the same way.
func WeekdayNames(lang string) (full, short []string) {
	for d := time.Sunday; d <= time.Saturday; d++ {
		full = append(full, Weekday(lang, d))
		short = append(short, WeekdayShort(lang, d))
	}
	return full, short
}

// MonthNames - the twelve names of each form in lang, for scripts that
// format dates the same way (the admin's live example).
func MonthNames(lang string) (inDate, standalone, short []string) {
	for m := time.January; m <= time.December; m++ {
		inDate = append(inDate, MonthInDate(lang, m))
		standalone = append(standalone, Month(lang, m))
		short = append(short, MonthShort(lang, m))
	}
	return inDate, standalone, short
}

// Letter - a pattern letter Format understands, for the admin help:
// Meaning is an interface string (translated where shown); a month
// letter's example is its month form, the others' Example as is.
type Letter struct {
	Letter, Meaning, Example string
}

// Letters - the pattern letters, with Friday, 31 December 2027 as the
// example date.
var Letters = []Letter{
	{"d", i18n.N("day of the month, with a leading zero"), "01-31"},
	{"j", i18n.N("day of the month, without a leading zero"), "1-31"},
	{"l", i18n.N("day of the week"), ""},
	{"D", i18n.N("abbreviated day of the week"), ""},
	{"N", i18n.N("number of the day of the week, 1 (Monday) to 7 (Sunday)"), "1-7"},
	{"m", i18n.N("month number, with a leading zero"), "01-12"},
	{"n", i18n.N("month number, without a leading zero"), "1-12"},
	{"F", i18n.N("month name inside a date"), ""},
	{"f", i18n.N("month name on its own"), ""},
	{"M", i18n.N("abbreviated month name"), ""},
	{"Y", i18n.N("year, 4 digits"), "2027"},
	{"y", i18n.N("year, 2 digits"), "27"},
	{"H", i18n.N("hours, with a leading zero"), "00-23"},
	{"G", i18n.N("hours, without a leading zero"), "0-23"},
	{"h", i18n.N("hours of a 12-hour clock, with a leading zero"), "01-12"},
	{"g", i18n.N("hours of a 12-hour clock, without a leading zero"), "1-12"},
	{"A", i18n.N("before or after noon, upper case"), "AM, PM"},
	{"a", i18n.N("before or after noon, lower case"), "am, pm"},
	{"i", i18n.N("minutes"), "00-59"},
	{"s", i18n.N("seconds"), "00-59"},
	{`\x`, i18n.N("the character x as is"), `\x → x`},
}

// LettersIn - Letters with the meanings translated to lang and the
// month letters' examples in lang.
func LettersIn(lang string) []Letter {
	out := make([]Letter, len(Letters))
	for i, l := range Letters {
		l.Meaning = i18n.Current().Text(lang, "", l.Meaning)
		switch l.Letter {
		case "F":
			l.Example = MonthInDate(lang, time.December)
		case "f":
			l.Example = Month(lang, time.December)
		case "M":
			l.Example = MonthShort(lang, time.December)
		case "l":
			l.Example = Weekday(lang, time.Friday)
		case "D":
			l.Example = WeekdayShort(lang, time.Friday)
		}
		out[i] = l
	}
	return out
}

// Format renders t by pattern in lang (month names): the letters above
// are replaced, a backslash outputs the next character as is, anything
// else is copied.
func Format(t time.Time, pattern, lang string) string {
	if lang == "" {
		lang = i18n.Default()
	}
	var b strings.Builder
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '\\':
			if i+1 < len(runes) {
				i++
				b.WriteRune(runes[i])
			}
		case 'd':
			b.WriteString(pad2(t.Day()))
		case 'j':
			b.WriteString(strconv.Itoa(t.Day()))
		case 'm':
			b.WriteString(pad2(int(t.Month())))
		case 'n':
			b.WriteString(strconv.Itoa(int(t.Month())))
		case 'F':
			b.WriteString(MonthInDate(lang, t.Month()))
		case 'f':
			b.WriteString(Month(lang, t.Month()))
		case 'M':
			b.WriteString(MonthShort(lang, t.Month()))
		case 'Y':
			b.WriteString(strconv.Itoa(t.Year()))
		case 'y':
			b.WriteString(pad2(t.Year() % 100))
		case 'H':
			b.WriteString(pad2(t.Hour()))
		case 'G':
			b.WriteString(strconv.Itoa(t.Hour()))
		case 'h':
			b.WriteString(pad2(hour12(t.Hour())))
		case 'g':
			b.WriteString(strconv.Itoa(hour12(t.Hour())))
		case 'A':
			b.WriteString(map[bool]string{false: "AM", true: "PM"}[t.Hour() >= 12])
		case 'a':
			b.WriteString(map[bool]string{false: "am", true: "pm"}[t.Hour() >= 12])
		case 'l':
			b.WriteString(Weekday(lang, t.Weekday()))
		case 'D':
			b.WriteString(WeekdayShort(lang, t.Weekday()))
		case 'N':
			b.WriteString(strconv.Itoa((int(t.Weekday())+6)%7 + 1))
		case 'i':
			b.WriteString(pad2(t.Minute()))
		case 's':
			b.WriteString(pad2(t.Second()))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// hour12 - hour h (0-23) on a 12-hour clock: 12, 1…11, 12, 1…11.
func hour12(h int) int {
	if h%12 == 0 {
		return 12
	}
	return h % 12
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// Separators of a range: between two days of one month (12-15 May),
// between two dates, and between a date and its time.
const (
	sepDays  = "-"
	sepDates = " - "
	sepTime  = ", "
)

// Range - a date (or a range from start to end; end zero - none) by
// datePattern, its times (withTime) by timePattern: "12 May 2027, 19:00",
// "12 May 2027, 19:00-21:30" (one day), "12 May 2027, 19:00 - 13 May
// 2027, 02:00". Without times, short shows what the two dates share once -
// "12-15 May 2027", "28 May - 3 June 2027" - when datePattern puts the
// day, the month and the year in that order; otherwise the dates are
// whole. The times are in the zone they come in.
func Range(start, end time.Time, datePattern, timePattern string, withTime, short bool, lang string) string {
	date := func(t time.Time) string { return Format(t, datePattern, lang) }
	at := func(t time.Time) string {
		if !withTime {
			return date(t)
		}
		return date(t) + sepTime + Format(t, timePattern, lang)
	}
	if end.IsZero() {
		return at(start)
	}
	sy, sm, sd := start.Date()
	ey, em, ed := end.Date()
	switch {
	case sy == ey && sm == em && sd == ed:
		if withTime {
			return at(start) + sepDays + Format(end, timePattern, lang)
		}
		return date(start)
	case withTime || !short:
	case sy == ey && sm == em:
		if head, ok := prefixThrough(datePattern, 'd'); ok {
			first := Format(start, head, lang)
			sep := sepDays
			if strings.Trim(first, "0123456789") != "" {
				sep = " " + sepDays + " "
			}
			return first + sep + date(end)
		}
	case sy == ey:
		if head, ok := prefixThrough(datePattern, 'm'); ok {
			return Format(start, head, lang) + sepDates + date(end)
		}
	}
	return at(start) + sepDates + at(end)
}

// letterKind - what a pattern letter shows: 'd' - the day (or the
// weekday), 'm' - the month, 'y' - the year, 't' - the time; 0 - not a
// letter.
func letterKind(r rune) rune {
	switch r {
	case 'd', 'j', 'l', 'D', 'N':
		return 'd'
	case 'm', 'n', 'F', 'f', 'M':
		return 'm'
	case 'Y', 'y':
		return 'y'
	case 'H', 'G', 'h', 'g', 'A', 'a', 'i', 's':
		return 't'
	}
	return 0
}

// prefixThrough - the start of pattern through its last letter of kind
// ('d' - the day, 'm' - the month): what the first date of a shortened
// range shows. false - the pattern doesn't put the day, the month and the
// year in that order (or shows a time): it can't be shortened.
func prefixThrough(pattern string, kind rune) (string, bool) {
	runes := []rune(pattern)
	order := map[rune]int{'d': 0, 'm': 1, 'y': 2}
	last, end, seen := -1, -1, map[rune]bool{}
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' {
			i++
			continue
		}
		k := letterKind(runes[i])
		switch {
		case k == 0:
			continue
		case k == 't':
			return "", false
		case order[k] < last:
			return "", false
		}
		last = order[k]
		seen[k] = true
		if k == kind {
			end = i + 1
		}
	}
	if !seen['d'] || !seen['m'] || !seen['y'] || end < 0 {
		return "", false
	}
	return string(runes[:end]), true
}
