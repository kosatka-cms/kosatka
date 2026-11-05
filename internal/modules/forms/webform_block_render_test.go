package forms

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestWebformBlockRender - the "Form" block is a placeholder loading its
// chosen form; with no form chosen it renders nothing.
func TestWebformBlockRender(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	render := func(cfg map[string]string) string {
		t.Helper()
		id := manifest.PluginBlockWebform
		return testkit.RenderPlacement(t, modulekit.BlockPlacement{ID: 1, PlacementType: modulekit.BlockPlacementPlugin, PluginID: &id, Region: "sidebar", PluginConfig: cfg, VisibilityScope: modulekit.BlockVisibilityAll}, nil)
	}
	if out := render(map[string]string{manifestkit.PluginBlockTargetKey: "12"}); !strings.Contains(out, `data-webform-embed="/node/12/form/embed"`) {
		t.Errorf("chosen form: %s", out)
	}
	if out := render(nil); strings.Contains(out, "webform") {
		t.Errorf("no form chosen: %s", out)
	}
}
