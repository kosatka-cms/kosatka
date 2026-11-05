package galleries

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// TestGalleryPreviewSize - a gallery takes its portrait grid and preview
// size from its grid preview style; a style the site lacks leaves them
// unknown.
func TestGalleryPreviewSize(t *testing.T) {
	refs := coreRefs{styles: map[string]modulekit.ImageStyleDef{
		"portraits": {Name: "portraits", Width: 150, Height: 225},
		"stills":    {Name: "stills", Width: 215, Height: 144},
	}}
	g := ToGalleryEntity(&generated.Gallery{PreviewSmallStyle: "portraits", PreviewMediumStyle: "stills"}, refs)
	if !g.IsPortrait || g.PreviewWidth != 150 || g.PreviewHeight != 225 || g.PreviewMediumStyle != "stills" {
		t.Errorf("portrait style: %+v", g)
	}
	g = ToGalleryEntity(&generated.Gallery{PreviewSmallStyle: "stills"}, refs)
	if g.IsPortrait || g.PreviewWidth != 215 {
		t.Errorf("landscape style: %+v", g)
	}
	g = ToGalleryEntity(&generated.Gallery{PreviewSmallStyle: "deleted"}, refs)
	if g.IsPortrait || g.PreviewWidth != 0 {
		t.Errorf("a style the site lacks: %+v", g)
	}
}
