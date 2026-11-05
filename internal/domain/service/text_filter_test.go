package service

import (
	"context"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// withTextFilters registers filters (and the formats' filters) for the
// test, none after it.
func withTextFilters(t *testing.T, formats map[int][]string, filters ...TextFilter) {
	t.Helper()
	reset := func() {
		textFiltersMu.Lock()
		textFilters, formatFilters = nil, map[int][]string{}
		textFiltersMu.Unlock()
	}
	reset()
	t.Cleanup(reset)
	for _, f := range filters {
		RegisterTextFilter(f)
	}
	for id, names := range formats {
		SetTextFormatFilters(id, names)
	}
}

func suffixFilter(name, module, suffix string) TextFilter {
	return TextFilter{Name: name, Module: module, Label: name, Apply: func(_ context.Context, html string) string { return html + suffix }}
}

// TestApplyTextFilters - a format's filters run in registration order,
// with the page's language; a format without filters, a value without a
// format and a disabled module's filter change nothing.
func TestApplyTextFilters(t *testing.T) {
	withModules(t, map[string]bool{"off": false})
	lang := TextFilter{Name: "lang", Module: "on", Apply: func(ctx context.Context, html string) string { return html + "[" + i18n.FromContext(ctx) + "]" }}
	withTextFilters(t, map[int][]string{1: {"b", "a", "lang"}, 2: {"disabled"}},
		suffixFilter("a", "on", "+a"), suffixFilter("b", "on", "+b"), lang, suffixFilter("disabled", "off", "+x"))
	ctx := i18n.WithLang(context.Background(), "ru")
	if got := ApplyTextFilters(ctx, "<p>t</p>", 1); got != "<p>t</p>+a+b[ru]" {
		t.Errorf("format 1: %s", got)
	}
	for _, id := range []int{0, 2, 3} {
		if got := ApplyTextFilters(ctx, "<p>t</p>", id); got != "<p>t</p>" {
			t.Errorf("format %d: %s", id, got)
		}
	}
	if names := filterNames(TextFilters()); names != "a b lang" {
		t.Errorf("offered: %s, want the enabled modules' ones", names)
	}
	// One of the same name replaces it.
	RegisterTextFilter(suffixFilter("a", "on", "+A"))
	if got := ApplyTextFilters(ctx, "", 1); !strings.HasPrefix(got, "+A+b") {
		t.Errorf("replaced: %s", got)
	}
	// Loading the formats starts over.
	LoadTextFormatFilters(entity.TextFormatDefs{{ID: 2, Filters: "a"}})
	if ApplyTextFilters(ctx, "", 1) != "" || ApplyTextFilters(ctx, "", 2) != "+A" {
		t.Error("LoadTextFormatFilters: want format 1 none, format 2 a")
	}
}

func filterNames(fs []TextFilter) string {
	var names []string
	for _, f := range fs {
		names = append(names, f.Name)
	}
	return strings.Join(names, " ")
}

// TestEditorFor - a format with a filter that has a button gets it after
// its own toolbar (TinyMCE's default one when it has none) and its plugin
// as an external one; without such filters the editor is the format's.
func TestEditorFor(t *testing.T) {
	withModules(t, nil)
	video := TextFilter{Name: "video", Module: "m", EditorButton: "kvideo", EditorPlugin: "plugin"}
	withTextFilters(t, map[int][]string{1: {"video", "plain"}, 2: {"video"}, 3: {"plain"}}, video, suffixFilter("plain", "m", ""))
	e := EditorFor(entity.TextFormatDef{ID: 1, TinyMCEPlugins: "link", TinyMCEToolbar: "undo redo | bold link"})
	if e.Plugins != "link" || e.Toolbar != "undo redo | bold link | kvideo" || !strings.Contains(e.External, `{"kvideo":"/static/modules/m/plugin.min.js?v=`) {
		t.Errorf("with a button: %+v", e)
	}
	if e := EditorFor(entity.TextFormatDef{ID: 2}); e.Toolbar != tinymceDefaultToolbar+" | kvideo" || e.External == "" {
		t.Errorf("no toolbar of its own: %+v", e)
	}
	if e := EditorFor(entity.TextFormatDef{ID: 3, TinyMCEToolbar: "bold"}); e.Toolbar != "bold" || e.External != "" {
		t.Errorf("a filter without a button: %+v", e)
	}
}
