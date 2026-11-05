package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/assets"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TextFilter - a module's processing of HTML text when it's shown (video
// links into players, say), turned on per text format
// (entity.TextFormatDef.Filters): the stored value stays as saved, so
// changing the filter or the format changes the old texts too.
type TextFilter struct {
	// Name - the filter's machine name, stored in the formats' Filters;
	// unique across modules.
	Name string
	// Module - the module the filter belongs to: off with it.
	Module string
	// Label, Description - on the text format page (i18n.N strings).
	Label       string
	Description string
	// Apply returns html processed; ctx carries the page's language
	// (i18n.WithLang). It's called on every show, so it should be cheap:
	// no network, data cached.
	Apply func(ctx context.Context, html string) string
	// EditorButton, EditorPlugin - a TinyMCE toolbar button the editors of
	// the formats with the filter on get, and the module's script
	// registering it (its static/<EditorPlugin>.js - the core's
	// web/static/js/<EditorPlugin>.js -, loaded as a TinyMCE external
	// plugin); empty - none.
	EditorButton string
	EditorPlugin string
}

var (
	textFiltersMu sync.RWMutex
	textFilters   []TextFilter
	// formatFilters - text format ID → its filters' names, kept in sync by
	// the text format service (LoadTextFormatFilters at startup, every
	// save and delete).
	formatFilters = map[int][]string{}
)

// RegisterTextFilter adds a module's text filter; one of the same name
// replaces it (the tests build the app more than once).
func RegisterTextFilter(f TextFilter) {
	textFiltersMu.Lock()
	defer textFiltersMu.Unlock()
	for i := range textFilters {
		if textFilters[i].Name == f.Name {
			textFilters[i] = f
			return
		}
	}
	textFilters = append(textFilters, f)
}

// TextFilters - the filters of the enabled modules, in registration order
// (the text format page's choice).
func TextFilters() []TextFilter {
	textFiltersMu.RLock()
	defer textFiltersMu.RUnlock()
	var out []TextFilter
	for _, f := range textFilters {
		if ModuleEnabled(f.Module) {
			out = append(out, f)
		}
	}
	return out
}

// moduleTextFilters - the names of the module's filters (registered
// whether it's enabled or not).
func moduleTextFilters(module string) []string {
	textFiltersMu.RLock()
	defer textFiltersMu.RUnlock()
	var names []string
	for _, f := range textFilters {
		if f.Module == module {
			names = append(names, f.Name)
		}
	}
	return names
}

// SetTextFormatFilters records one format's filters (nil - none, e.g. a
// deleted format).
func SetTextFormatFilters(formatID int, names []string) {
	textFiltersMu.Lock()
	defer textFiltersMu.Unlock()
	if len(names) == 0 {
		delete(formatFilters, formatID)
		return
	}
	formatFilters[formatID] = append([]string(nil), names...)
}

// LoadTextFormatFilters fills the format → filters map from formats -
// called once at startup; every save keeps it in sync afterwards.
func LoadTextFormatFilters(formats entity.TextFormatDefs) {
	textFiltersMu.Lock()
	formatFilters = map[int][]string{}
	textFiltersMu.Unlock()
	for _, f := range formats {
		SetTextFormatFilters(f.ID, f.FilterNames())
	}
}

// enabledFilters - the filters on for the format whose module is enabled,
// in registration order.
func enabledFilters(formatID int) []TextFilter {
	textFiltersMu.RLock()
	defer textFiltersMu.RUnlock()
	names := formatFilters[formatID]
	if len(names) == 0 {
		return nil
	}
	var out []TextFilter
	for _, f := range textFilters {
		for _, n := range names {
			if f.Name == n && ModuleEnabled(f.Module) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// ApplyTextFilters runs the format's filters over html (formatID 0 - a
// value without a format: none).
func ApplyTextFilters(ctx context.Context, html string, formatID int) string {
	for _, f := range enabledFilters(formatID) {
		if f.Apply != nil {
			html = f.Apply(ctx, html)
		}
	}
	return html
}

// TextFormatEditor - what the editor of a format loads: its own plugins
// and toolbar, plus the buttons of its filters (Toolbar, after the
// format's own) and their plugins' addresses (External, JSON for TinyMCE's
// external_plugins; "" - none). A format without a toolbar of its own
// has TinyMCE's default one, the buttons after it.
type TextFormatEditor struct {
	Plugins  string
	Toolbar  string
	External string
}

// tinymceDefaultToolbar - the toolbar TinyMCE (8) shows when given none:
// a format without its own gets the filters' buttons after it.
const tinymceDefaultToolbar = "undo redo | styles | bold italic | alignleft aligncenter alignright alignjustify | outdent indent"

// EditorFor - the editor of format f with its filters' buttons.
func EditorFor(f entity.TextFormatDef) TextFormatEditor {
	e := TextFormatEditor{Plugins: f.TinyMCEPlugins, Toolbar: f.TinyMCEToolbar}
	external := map[string]string{}
	var buttons []string
	for _, flt := range enabledFilters(f.ID) {
		if flt.EditorButton == "" || flt.EditorPlugin == "" {
			continue
		}
		// A module's static/<plugin>.js; the core's web/static/js/<plugin>.js.
		if flt.Module == "" {
			external[flt.EditorButton] = assets.StaticJS(flt.EditorPlugin)
		} else {
			external[flt.EditorButton] = assets.ModuleURL(flt.Module, flt.EditorPlugin, "js")
		}
		buttons = append(buttons, flt.EditorButton)
	}
	if len(buttons) == 0 {
		return e
	}
	toolbar := e.Toolbar
	if toolbar == "" {
		toolbar = tinymceDefaultToolbar
	}
	e.Toolbar = toolbar + " | " + strings.Join(buttons, " ")
	b, _ := json.Marshal(external) // a map of strings always marshals
	e.External = string(b)
	return e
}
