package service

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestPluginBlockDispatch_Copyright - the copyright block: "© <year>
// <the site's name>" by default; its settings give the holder and the
// first year; the holder is escaped.
func TestPluginBlockDispatch_Copyright(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)
	pluginID := entity.PluginBlockCopyright
	year := strconv.Itoa(time.Now().Year())
	render := func(config map[string]string, siteName string) string {
		bp := map[entity.Region][]entity.BlockWithPlacement{
			entity.Region("footer_bottom"): {{
				Placement: entity.BlockPlacementDef{PlacementType: entity.BlockPlacementPlugin, PluginID: &pluginID, VisibilityScope: entity.BlockVisibilityAll, PluginConfig: config},
			}},
		}
		data := map[string]any{
			"Region": entity.Region("footer_bottom"),
			"Ctx":    map[string]any{"blockPlacements": bp, "isHomepage": false, "siteName": siteName},
		}
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "region_blocks", data, zzBlockTemplateFiles...); err != nil {
			t.Fatalf("render error: %v", err)
		}
		return buf.String()
	}
	for _, tc := range []struct {
		config   map[string]string
		siteName string
		want     string
	}{
		{nil, "Example Site", "© " + year + " Example Site</p>"},
		{map[string]string{"holder": "Example & Co", "since": "2008"}, "Example Site", "© 2008-" + year + " Example &amp; Co</p>"},
		{map[string]string{"holder": "", "since": "0"}, "", "© " + year + "</p>"},
	} {
		if out := render(tc.config, tc.siteName); !strings.Contains(out, `class="site-footer-credit block-copyright">`+tc.want) {
			t.Errorf("%v: want %q in %s", tc.config, tc.want, out)
		}
	}
}

// TestPluginPlacementEditRenders_TextSetting - a text setting is a text
// input with its value and length limit.
func TestPluginPlacementEditRenders_TextSetting(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)
	pluginID := entity.PluginBlockCopyright
	data := map[string]any{
		"title":     "System block",
		"placement": entity.BlockPlacementDef{ID: 7, PlacementType: entity.BlockPlacementPlugin, PluginID: &pluginID, Region: entity.Region("footer_bottom"), VisibilityScope: entity.BlockVisibilityAll},
		"pluginSettings": []struct {
			entity.PluginBlockSetting
			Value string
		}{{entity.PluginBlockSetting{Key: "holder", Label: "Holder", Type: entity.PluginBlockSettingText, Max: 200}, "Example & Co"}},
		"allRegions":   zzFixtureThemeRegions(t).RegionIDsWithDisabled(),
		"regionLabels": regionLabelsForTest,
	}
	var buf bytes.Buffer
	if err := svc.RenderNamed(&buf, "content", data, "admin/blocks/plugin_placement_edit.html", "admin/alerts.html", "admin/blocks/visibility_content_types.html", "admin/blocks/visibility_roles.html", "admin/blocks/visibility_scope.html"); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if want := `<input type="text" id="plugin-setting-holder" name="setting_holder" class="form-control" value="Example &amp; Co" maxlength="200">`; !strings.Contains(buf.String(), want) {
		t.Errorf("want %q in %s", want, buf.String())
	}
}
