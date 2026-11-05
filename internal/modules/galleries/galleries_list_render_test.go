package galleries

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestGalleriesListCover - the node's galleries list takes the cover
// style's proportions: a style taller than wide gives the portrait grid,
// the image its size; no style - neither.
func TestGalleriesListCover(t *testing.T) {
	testkit.UseEngineRoot(t)
	withModule(t, true)
	render := func(style *modulekit.ImageStyleDef) string {
		return testkit.RenderPageBlock(t, "node/galleries_list.html", "content", map[string]any{"node": &modulekit.Node{ID: 4},
			"galleries": []*Gallery{{ID: 2, NodeID: 4, Cover: &modulekit.File{URI: "uploads/a.jpg"}}}, "coverStyle": style})
	}
	out := render(&modulekit.ImageStyleDef{Name: CoverListStyle, Width: 300, Height: 450})
	for _, want := range []string{`class="galleries-grid galleries-grid--portrait" style="--gallery-cover-w: 300; --gallery-cover-h: 450"`, `width="300" height="450"`} {
		if !strings.Contains(out, want) {
			t.Errorf("portrait style: no %s in %s", want, out)
		}
	}
	if out := render(&modulekit.ImageStyleDef{Name: CoverListStyle, Width: 600, Height: 200}); strings.Contains(out, "galleries-grid--portrait") || !strings.Contains(out, `--gallery-cover-w: 600; --gallery-cover-h: 200`) {
		t.Errorf("landscape style: %s", out)
	}
	if out := render(nil); !strings.Contains(out, `<div class="galleries-grid">`) || strings.Contains(out, `width="`) {
		t.Errorf("no style: %s", out)
	}
}
