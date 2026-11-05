package videoembed

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/manifest"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestSettingsPage - the settings page lists every service with its
// switch as saved, and the two loading switches.
func TestSettingsPage(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	rows := make([]providerRow, len(providers))
	for i, p := range providers {
		rows[i] = providerRow{Provider: p, Checked: p.Name == "vk"}
	}
	out := testkit.RenderPageContent(t, "admin/video_embed.html", map[string]any{"providers": rows, "settings": Settings{LocalPreviews: true}})
	for _, want := range []string{
		`id="provider_vk" name="providers" value="vk" class="form-check-input" checked>`,
		`id="provider_youtube" name="providers" value="youtube" class="form-check-input">`,
		`id="click_to_play" name="click_to_play" value="1" class="form-check-input">`,
		`id="local_previews" name="local_previews" value="1" class="form-check-input" checked>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}
