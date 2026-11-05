package service

import (
	"context"
	"html/template"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// FieldFormatter - a module's way of showing a field's value (a video
// player for a link, say), chosen per field: on the content type's
// "Detail page" tab (entity.FieldInstanceDef.DetailFormatter) and per
// View field (entity.ViewFieldDef.Formatter). A field whose formatter's
// module is off shows as the core shows it (links).
type FieldFormatter struct {
	// Name - the formatter's machine name, stored in the fields'
	// settings; unique across modules.
	Name string
	// Module - the module the formatter belongs to: off with it.
	Module string
	// Label - in the formatter choice (an i18n.N string).
	Label string
	// FieldType - the type of the fields it shows (entity.FieldTypeLink).
	FieldType entity.ContentTypeFieldType
	// Render returns value shown in layout (entity.FieldLayoutFirst is
	// applied before: the value holds the first only). value is the
	// field's as stored (a link field's - []entity.Link); a part it can't
	// show (a link to a site it doesn't know) it shows as the core does.
	// ctx carries the page's language. Called on every show: cheap, no
	// network, data cached.
	Render func(ctx context.Context, value any, layout entity.FieldLayout) template.HTML
	// Hosts - the sites whose links it shows (lower case, without
	// "www."): the field's settings offer them as its allowed sites; nil -
	// none to offer.
	Hosts func(ctx context.Context) []string
	// EditorScript - the module's script (static/<EditorScript>.js) the
	// content form loads when a field shows with the formatter on the
	// detail page (a preview of a pasted link, say); "" - none.
	EditorScript string
}

var (
	fieldFormattersMu sync.RWMutex
	fieldFormatters   []FieldFormatter
)

// RegisterFieldFormatter adds a module's field formatter; one of the same
// name replaces it (the tests build the app more than once).
func RegisterFieldFormatter(f FieldFormatter) {
	fieldFormattersMu.Lock()
	defer fieldFormattersMu.Unlock()
	for i := range fieldFormatters {
		if fieldFormatters[i].Name == f.Name {
			fieldFormatters[i] = f
			return
		}
	}
	fieldFormatters = append(fieldFormatters, f)
}

// FieldFormatters - the formatters of the enabled modules for fields of
// type t, in registration order (the formatter choice).
func FieldFormatters(t entity.ContentTypeFieldType) []FieldFormatter {
	fieldFormattersMu.RLock()
	defer fieldFormattersMu.RUnlock()
	var out []FieldFormatter
	for _, f := range fieldFormatters {
		if f.FieldType == t && ModuleEnabled(f.Module) {
			out = append(out, f)
		}
	}
	return out
}

// FieldFormatterFor - the formatter name of a field of type t, its module
// enabled; false - none ("" - the core's own way), unknown or off.
func FieldFormatterFor(t entity.ContentTypeFieldType, name string) (FieldFormatter, bool) {
	if name == "" {
		return FieldFormatter{}, false
	}
	for _, f := range FieldFormatters(t) {
		if f.Name == name {
			return f, true
		}
	}
	return FieldFormatter{}, false
}

// moduleFieldFormatters - the names of the module's formatters
// (registered whether it's enabled or not).
func moduleFieldFormatters(module string) []string {
	fieldFormattersMu.RLock()
	defer fieldFormattersMu.RUnlock()
	var names []string
	for _, f := range fieldFormatters {
		if f.Module == module {
			names = append(names, f.Name)
		}
	}
	return names
}

// FirstValue - value with only its first item when layout says so
// (entity.FieldLayoutFirst), a link field's; otherwise value.
func FirstValue(value any, layout entity.FieldLayout) any {
	if links := entity.Links(value); layout == entity.FieldLayoutFirst && len(links) > 1 {
		return links[:1]
	}
	return value
}

// FormatField - value of a field of type t shown by its formatter name in
// layout; false - the core shows it (no such formatter, or its module is
// off).
func FormatField(ctx context.Context, t entity.ContentTypeFieldType, name string, value any, layout entity.FieldLayout) (template.HTML, bool) {
	f, ok := FieldFormatterFor(t, name)
	if !ok || f.Render == nil {
		return "", false
	}
	return f.Render(ctx, FirstValue(value, layout), layout), true
}
