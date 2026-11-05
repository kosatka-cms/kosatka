package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestFeatureSwitchesInTemplates - the back to top button and the error
// report modal are rendered only while their switch is on; the search
// block follows its switch.
func TestFeatureSwitchesInTemplates(t *testing.T) {
	zzChdirToProjectRoot(t)
	withModules(t, nil)
	render := func(name string, files ...string) string {
		t.Helper()
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, name, map[string]any{}, files...); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	check := func(on bool) {
		t.Helper()
		if got := strings.Contains(render("html_end", "partials/html.html"), `id="back-to-top"`); got != on {
			t.Errorf("back to top rendered = %v, want %v", got, on)
		}
		if got := strings.Contains(render("error_report_modal", "partials/error_report_modal.html"), `id="error-report-modal"`); got != on {
			t.Errorf("error report modal rendered = %v, want %v", got, on)
		}
		if got := PluginBlockEnabled(entity.PluginBlockSearchForm); got != on {
			t.Errorf("search block enabled = %v, want %v", got, on)
		}
		if got := PluginBlockEnabled(entity.PluginBlockID("error_report_hint")); got != on {
			t.Errorf("error report hint block enabled = %v, want %v", got, on)
		}
	}
	check(true)
	SetModules(map[string]bool{entity.FeatureBackToTop: false, "error_reports": false, entity.FeatureSearch: false})
	check(false)
}
