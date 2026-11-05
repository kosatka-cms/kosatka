package galleries

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestEditGalleryUploadFormats - the upload's file picker and hint follow
// the upload policy's image types; unknown, any image and no formats.
func TestEditGalleryUploadFormats(t *testing.T) {
	testkit.UseEngineRoot(t)
	withModule(t, true)
	data := func(accept, formats string) map[string]any {
		d := map[string]any{"node": &modulekit.Node{ID: 4}, "gallery": &Gallery{ID: 2, NodeID: 4}}
		if formats != "" {
			d["uploadAccept"], d["uploadFormats"] = accept, formats
		}
		return d
	}
	out := testkit.RenderPageBlock(t, "node/edit_gallery.html", "content", data("image/jpeg,image/png", "JPEG, PNG"))
	for _, want := range []string{`accept="image/jpeg,image/png"`, "Formats: JPEG, PNG."} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s", want)
		}
	}
	out = testkit.RenderPageBlock(t, "node/edit_gallery.html", "content", data("", ""))
	if !strings.Contains(out, `accept="image/*"`) || strings.Contains(out, "Formats:") {
		t.Error("the policy unknown: want any image, no formats")
	}
}
