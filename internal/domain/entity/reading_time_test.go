package entity

import (
	"strings"
	"testing"
)

// TestReadingMinutes - the words of HTML or plain text at 200 a minute,
// the characters of a script written without spaces at 500, rounded up;
// the tags, a lone dash and an empty text aren't read.
func TestReadingMinutes(t *testing.T) {
	words := func(n int) string { return strings.Repeat("word ", n) }
	for _, tc := range []struct {
		name  string
		texts []string
		want  int
	}{
		{"nothing", nil, 0},
		{"empty and tags only", []string{"", "<p><img src=\"a.jpg\"></p>"}, 0},
		{"one word", []string{"<p>Hello</p>"}, 1},
		{"200 words", []string{words(200)}, 1},
		{"201 words", []string{words(201)}, 2},
		{"two texts", []string{words(150), "<p>" + words(150) + "</p>"}, 2},
		{"dashes aren't words", []string{words(200) + " - - -"}, 1},
		{"Han characters", []string{strings.Repeat("字", 500)}, 1},
		{"Han characters and words", []string{strings.Repeat("字", 250) + " " + words(100)}, 1},
		{"a little over", []string{strings.Repeat("字", 251) + " " + words(100)}, 2},
	} {
		if got := ReadingMinutes(tc.texts...); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
}

// TestReadingMinutesOf - only the long text fields shown on the content
// page are read: not a one-line text, a hidden one or another type.
func TestReadingMinutesOf(t *testing.T) {
	long := func(id int, widget FieldWidget, shown bool) FieldInstanceDef {
		return FieldInstanceDef{ID: id, FieldType: FieldTypeText, WidgetConfig: map[string]any{"widget": string(widget)}, ShowOnDetail: shown}
	}
	fields := FieldInstanceDefs{
		long(1, WidgetWYSIWYG, true),
		long(2, WidgetTextarea, true),
		long(3, WidgetInput, true),
		long(4, WidgetWYSIWYG, false),
		{ID: 5, FieldType: FieldTypeNumber, ShowOnDetail: true},
		long(6, WidgetSafeHTML, true),
	}
	text := strings.Repeat("word ", 150)
	values := map[int]any{1: text, 2: []any{text, text}, 3: text, 4: text, 5: 7, 6: []string{text}}
	if got := ReadingMinutesOf(fields, func(f FieldInstanceDef) any { return values[f.ID] }); got != 3 {
		t.Errorf("%d minutes, want 3 (600 words in fields 1, 2 and 6)", got)
	}
	if got := ReadingMinutesOf(fields, func(FieldInstanceDef) any { return nil }); got != 0 {
		t.Errorf("no values: %d, want 0", got)
	}
}
