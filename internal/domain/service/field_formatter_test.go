package service

import (
	"context"
	"fmt"
	"html/template"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// withFieldFormatters - the registered field formatters are these for the
// test.
func withFieldFormatters(t *testing.T, formatters ...FieldFormatter) {
	t.Helper()
	reset := func() {
		fieldFormattersMu.Lock()
		fieldFormatters = nil
		fieldFormattersMu.Unlock()
	}
	reset()
	t.Cleanup(reset)
	for _, f := range formatters {
		RegisterFieldFormatter(f)
	}
}

// countLinks - a formatter showing how many links it got and the layout.
func countLinks(name, module string) FieldFormatter {
	return FieldFormatter{Name: name, Module: module, Label: name, FieldType: entity.FieldTypeLink,
		Render: func(_ context.Context, v any, layout entity.FieldLayout) template.HTML {
			return template.HTML(fmt.Sprintf("%s:%d:%s", name, len(entity.Links(v)), layout))
		}}
}

// TestFieldFormatters - a field's formatter shows it while its module is
// on and for fields of its type; the "first" layout hands it the first
// link only; off, unknown or "" - the core shows the field.
func TestFieldFormatters(t *testing.T) {
	withModules(t, map[string]bool{"video": true, "off": false})
	withFieldFormatters(t, countLinks("player", "video"), countLinks("hidden", "off"))
	links := []entity.Link{{URL: "https://a.example"}, {URL: "https://b.example"}}
	ctx := context.Background()

	if got := FieldFormatters(entity.FieldTypeLink); len(got) != 1 || got[0].Name != "player" {
		t.Errorf("formatters %v, want the enabled module's only", got)
	}
	if got := FieldFormatters(entity.FieldTypeText); len(got) != 0 {
		t.Errorf("text formatters %v, want none", got)
	}
	if out, ok := FormatField(ctx, entity.FieldTypeLink, "player", links, entity.FieldLayoutFeatured); !ok || out != "player:2:featured" {
		t.Errorf("featured: %q %v", out, ok)
	}
	if out, ok := FormatField(ctx, entity.FieldTypeLink, "player", links, entity.FieldLayoutFirst); !ok || out != "player:1:first" {
		t.Errorf("first: %q %v", out, ok)
	}
	for _, name := range []string{"", "hidden", "unknown"} {
		if _, ok := FormatField(ctx, entity.FieldTypeLink, name, links, entity.FieldLayoutAll); ok {
			t.Errorf("%q formatted the field", name)
		}
	}
	if _, ok := FormatField(ctx, entity.FieldTypeText, "player", links, entity.FieldLayoutAll); ok {
		t.Error("a link formatter formatted a text field")
	}
	if got := moduleFieldFormatters("off"); len(got) != 1 || got[0] != "hidden" {
		t.Errorf("the off module's formatters %v", got)
	}
}
