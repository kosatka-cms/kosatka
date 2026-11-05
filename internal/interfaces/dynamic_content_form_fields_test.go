package interfaces

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
)

// TestTextValueSanitized - a text's HTML is sanitized whatever its field:
// a text format's, or the legacy widgets' (safe_html, wysiwyg); a plain
// text is kept as typed (it is escaped on output).
func TestTextValueSanitized(t *testing.T) {
	raw := `<p>Hi</p><script>alert(1)</script>`
	for _, tc := range []struct {
		widget entity.FieldWidget
		want   string
	}{
		{entity.WidgetWYSIWYG, `<p>Hi</p>`},
		{entity.WidgetSafeHTML, `<p>Hi</p>`},
		{entity.WidgetTextarea, raw},
	} {
		field := entity.FieldInstanceDef{ID: 1, FieldType: entity.FieldTypeText, WidgetConfig: map[string]any{"widget": string(tc.widget)}}
		got, _, ok := textValue(&form.DynamicForm{FieldValues: map[int]string{1: raw}}, field)
		if !ok || got != tc.want {
			t.Errorf("%s: %q, want %q", tc.widget, got, tc.want)
		}
	}
}
