package service

import (
	"html/template"
	"strings"
	"testing"
)

// TestViewPreviewHeadUsesSiteTheme - the Views preview frame gets the
// public pages' stylesheets with the active theme, and the engine script
// that starts carousels.
func TestViewPreviewHeadUsesSiteTheme(t *testing.T) {
	zzChdirToProjectRoot(t)
	zzUseFixtureTheme(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	var buf strings.Builder
	if err := svc.RenderNamed(&buf, "view_preview_head", map[string]any{}, "admin/views/preview.html", "partials/html.html", "partials/theme_head.html"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"/static/themes/fixture/css/styles", "/static/css/bootswatch-cosmo.min.css", "swiper-bundle.min.js", "/static/css/swiper-bundle.min.css", "/static/js/app"} {
		if !strings.Contains(out, want) {
			t.Errorf("preview head lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "cdn.jsdelivr.net/npm/swiper") {
		t.Error("Swiper's CSS comes from the CDN, not web/static/css")
	}
	if strings.Contains(out, "Preview - shows") {
		t.Error("the admin notice belongs outside the frame")
	}
}
