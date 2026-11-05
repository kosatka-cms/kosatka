package service

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// zzLayoutThemes runs fn with no theme and the fixture theme (its own
// layout.html and theme_framework).
func zzLayoutThemes(t *testing.T, fn func(t *testing.T)) {
	t.Run("engine", fn)
	t.Run(zzFixtureTheme, func(t *testing.T) { zzUseFixtureTheme(t); fn(t) })
}

// TestLayoutSidebar - the engine's and a theme's layouts show the
// sidebar column when its region has blocks for the page and it isn't a
// service page; the core says only which page it is (servicePage), not
// which regions to drop.
func TestLayoutSidebar(t *testing.T) {
	zzChdirToProjectRoot(t)
	logo := entity.PluginBlockLogo
	withBlock := map[entity.Region][]entity.BlockWithPlacement{
		"sidebar": {{Placement: entity.BlockPlacementDef{ID: 1, PlacementType: entity.BlockPlacementPlugin, PluginID: &logo, Region: "sidebar", VisibilityScope: entity.BlockVisibilityAll}}},
	}
	render := func(t *testing.T, data map[string]any) string {
		t.Helper()
		var buf strings.Builder
		ts := NewTemplateService(false).(*DefaultTemplateService)
		if err := ts.RenderPageBlock(&buf, "errors/not_found.html", "layout.html", data); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	zzLayoutThemes(t, func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			data    map[string]any
			sidebar bool
		}{
			{"blocks", map[string]any{"blockPlacements": withBlock}, true},
			{"service page", map[string]any{"blockPlacements": withBlock, "servicePage": true}, false},
			{"no blocks", map[string]any{"blockPlacements": map[entity.Region][]entity.BlockWithPlacement{}}, false},
		} {
			out := render(t, tc.data)
			if got := strings.Contains(out, "col-lg-3") && strings.Contains(out, "col-lg-9"); got != tc.sidebar {
				t.Errorf("%s: sidebar column %v, want %v", tc.name, got, tc.sidebar)
			}
		}
	})
}

// TestLayoutThemeFramework - the public page takes its Bootstrap build
// from the active theme's theme_framework (the engine's without a theme),
// once and from the site (web/static/css: Bootstrap or its Bootswatch
// skin), not a CDN; the theme starts no carousel of its own (the engine's
// initViewCarousels does, from data-view-carousel).
func TestLayoutThemeFramework(t *testing.T) {
	zzChdirToProjectRoot(t)
	zzLayoutThemes(t, func(t *testing.T) {
		var buf strings.Builder
		ts := NewTemplateService(false).(*DefaultTemplateService)
		if err := ts.RenderPageBlock(&buf, "errors/not_found.html", "layout.html", map[string]any{"blockPlacements": map[entity.Region][]entity.BlockWithPlacement{}}); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if n := strings.Count(out, "/static/css/bootstrap.min.css") + strings.Count(out, "/static/css/bootswatch-"); n != 1 {
			t.Errorf("Bootstrap CSS %d times, want once", n)
		}
		if strings.Contains(out, "cdn.jsdelivr.net/npm/boot") {
			t.Error("Bootstrap from the CDN, not web/static/css")
		}
		if strings.Contains(out, "new Swiper(") {
			t.Error("an inline Swiper init in the page")
		}
	})
}
