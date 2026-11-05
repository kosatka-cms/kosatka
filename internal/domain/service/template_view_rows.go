package service

import (
	"html"
	"html/template"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/dateformat"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
)

// rowTemplateFieldToken - any {{field:<machine_name>}} token, for the
// fields with no value left after substitution.
var rowTemplateFieldToken = regexp.MustCompile(`\{\{field:[a-zA-Z0-9_]+\}\}`)

// renderRowTemplate - see the "renderRowTemplate" template func.
func renderRowTemplate(tpl string, url string, fieldTokens map[string]template.HTML) template.HTML {
	out := strings.ReplaceAll(tpl, htmlsanitize.RowTemplateURLToken, html.EscapeString(url))
	for key, tok := range fieldTokens {
		out = strings.ReplaceAll(out, "{{field:"+key+"}}", string(tok))
	}
	return template.HTML(rowTemplateFieldToken.ReplaceAllString(out, ""))
}

// viewGroupLabel - the group heading of a row: for a date value with a date
// format (entity.ViewGroupDate*) - the year, month and year, or the day
// (month names in lang); otherwise the field's rendered output; "" when
// the row has no value. Rows with equal headings form one group.
func viewGroupLabel(value any, dateFormat string, rendered template.HTML, lang string) template.HTML {
	if t, ok := value.(time.Time); ok && dateFormat != "" {
		if t.IsZero() {
			return ""
		}
		switch dateFormat {
		case entity.ViewGroupDateYear:
			return template.HTML(strconv.Itoa(t.Year()))
		case entity.ViewGroupDateMonth:
			month := []rune(dateformat.Month(lang, t.Month()))
			return template.HTML(strings.ToUpper(string(month[:1])) + string(month[1:]) + " " + strconv.Itoa(t.Year()))
		case entity.ViewGroupDateDay:
			return template.HTML(t.Format("02.01.2006"))
		}
	}
	return template.HTML(strings.TrimSpace(string(rendered)))
}

// ViewRowGroup - one group of a View's rows: its heading and rows.
type ViewRowGroup struct {
	Heading template.HTML
	Items   []any
}

// groupViewRowsIn splits items (a slice of View rows with Fields/Values)
// into groups by the value of the field with machine name field: each
// heading once, in order of first appearance, rows in their original
// order. Without a field - one group with all rows (no heading); no rows -
// no groups. Builtin timestamps (Config "timestamp") are shown in a
// time zone (toZone), month names in lang - the request's; render gives
// a non-date field's heading (viewGroupLabel).
func groupViewRowsIn(items any, field, dateFormat, lang string, toZone func(time.Time) time.Time, render func(f entity.FieldInstanceDef, value, item any) (template.HTML, error)) ([]ViewRowGroup, error) {
	rows := reflect.ValueOf(items)
	if rows.Kind() != reflect.Slice || rows.Len() == 0 {
		return nil, nil
	}
	all := make([]any, rows.Len())
	for i := range all {
		all[i] = rows.Index(i).Interface()
	}
	if field == "" {
		return []ViewRowGroup{{Items: all}}, nil
	}
	var groups []ViewRowGroup
	index := map[template.HTML]int{}
	for i, item := range all {
		heading, err := viewRowHeading(rows.Index(i), item, field, dateFormat, lang, toZone, render)
		if err != nil {
			return nil, err
		}
		g, ok := index[heading]
		if !ok {
			g = len(groups)
			index[heading] = g
			groups = append(groups, ViewRowGroup{Heading: heading})
		}
		groups[g].Items = append(groups[g].Items, item)
	}
	return groups, nil
}

// viewRowHeading - the group heading of one row (see groupViewRowsIn).
func viewRowHeading(row reflect.Value, item any, field, dateFormat, lang string, toZone func(time.Time) time.Time, render func(f entity.FieldInstanceDef, value, item any) (template.HTML, error)) (template.HTML, error) {
	row = reflect.Indirect(row)
	if row.Kind() == reflect.Interface {
		row = reflect.Indirect(row.Elem())
	}
	if row.Kind() != reflect.Struct {
		return "", nil
	}
	fv, vv := row.FieldByName("Fields"), row.FieldByName("Values")
	if !fv.IsValid() || !vv.IsValid() {
		return "", nil
	}
	fields, _ := fv.Interface().(entity.FieldInstanceDefs)
	values, _ := vv.Interface().(map[int]any)
	for _, f := range fields {
		if f.MachineName != field {
			continue
		}
		v, ok := values[f.ID]
		if !ok || v == nil {
			return "", nil
		}
		if ts, _ := f.Config["timestamp"].(bool); ts {
			if t, isTime := v.(time.Time); isTime {
				v = toZone(t)
			}
		}
		// A date with a time or an end (several - the first) - grouped by
		// its start, in the page's zone when it has a time.
		var rich any
		switch v.(type) {
		case entity.DateValue, []entity.DateValue:
			rich = v
		}
		if d, ok := entity.AsDateValue(rich); ok && dateFormat != "" {
			start := d.Start
			if d.WithTime {
				start = toZone(start)
			}
			return viewGroupLabel(start, dateFormat, "", lang), nil
		}
		if t, isTime := v.(time.Time); isTime && dateFormat != "" {
			return viewGroupLabel(t, dateFormat, "", lang), nil
		}
		rendered, err := render(f, v, item)
		if err != nil {
			return "", err
		}
		return viewGroupLabel(v, "", rendered, lang), nil
	}
	return "", nil
}

// FieldDate - a date field's value with the pattern of its display
// format: the field_date_* templates print .Formatted; time.Time's
// methods (IsZero, Day, Month, Year, Format) of its start stay
// available for a theme's own date rules. A field with a time or an end
// (entity.DateValue) also has End (zero - none), WithTime (the times
// shown, by TimePattern; Time and End are in the page's zone then) and
// Short (a range shows what its dates share once).
type FieldDate struct {
	time.Time
	End         time.Time
	WithTime    bool
	Pattern     string
	TimePattern string
	Short       bool
	// Lang - the month names' language ("" - the site's default).
	Lang string
	// Past, Ongoing - the value has ended, is going on now (one of a
	// field's several dates: FieldDates).
	Past, Ongoing bool
}

// FieldDates - a date field's several dates to show (fieldDatesFor): in
// the order of the field's layout, and More - how many it leaves out
// (the nearest's "and N more").
type FieldDates struct {
	Dates []FieldDate
	More  int
}

// HasEnd - the value is a range.
func (d FieldDate) HasEnd() bool { return !d.End.IsZero() }

// Formatted - the date (with its time, or the range) by its patterns
// ("" for no date).
func (d FieldDate) Formatted() string {
	if d.IsZero() {
		return ""
	}
	if !d.WithTime && !d.HasEnd() {
		return dateformat.Format(d.Time, d.Pattern, d.Lang)
	}
	return dateformat.Range(d.Time, d.End, d.Pattern, d.TimePattern, d.WithTime, d.Short, d.Lang)
}

// viewRowNodeType - the content type of a View row (a struct with a
// Node entity.Node field).
func viewRowNodeType(item any) (string, bool) {
	v := reflect.Indirect(reflect.ValueOf(item))
	if v.Kind() != reflect.Struct {
		return "", false
	}
	n := v.FieldByName("Node")
	if !n.IsValid() {
		return "", false
	}
	node, ok := n.Interface().(entity.Node)
	return string(node.Type), ok
}
