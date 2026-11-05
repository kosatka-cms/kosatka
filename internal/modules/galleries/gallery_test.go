package galleries

import (
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
)

func TestGalleriesHelpers(t *testing.T) {
	art := &Gallery{ID: 1, Status: true}
	photo := &Gallery{ID: 2, Status: false}
	art2 := &Gallery{ID: 3, Status: false}
	list := Galleries{art, photo, art2}

	if list.Len() != 3 || list.IsEmpty() || !(Galleries{}).IsEmpty() {
		t.Error("Len/IsEmpty")
	}
	if got := list.FilterByStatus(false); len(got) != 2 || got[0] != photo || got[1] != art2 {
		t.Errorf("FilterByStatus(false) = %v", got)
	}
}

func TestGalleryHelpers(t *testing.T) {
	empty := &Gallery{}
	if empty.GetTitle() != "" || empty.GetDescription() != "" || empty.HasItems() || empty.ItemsCount() != 0 ||
		empty.GetFirstItem() != nil || empty.GetCoverImage() != nil || empty.GetCoverItem() != nil {
		t.Error("an empty gallery")
	}

	a := &modulekit.File{URI: "public://a.jpg"}
	b := &modulekit.File{URI: "public://b.jpg"}
	first := &GalleryItem{ID: 1, File: a}
	cover := &GalleryItem{ID: 2, File: b, IsCover: true}
	third := &GalleryItem{ID: 3}
	g := &Gallery{Title: new("Title"), Description: new("Text"), Items: GalleryItems{first, cover, third}}
	if g.GetTitle() != "Title" || g.GetDescription() != "Text" || !g.HasItems() || g.ItemsCount() != 3 || g.GetFirstItem() != first {
		t.Error("a gallery with items")
	}
	if g.GetCoverImage() != b || g.GetCoverItem() != cover {
		t.Error("the item marked as the cover is the cover")
	}
	noCover := &Gallery{Items: GalleryItems{first, third}}
	if noCover.GetCoverImage() != a || noCover.GetCoverItem() != first {
		t.Error("without a marked cover, the first item is the cover")
	}

	if got := g.GetPreviewItems(2); len(got) != 2 || got[0] != first || got[1] != cover {
		t.Errorf("GetPreviewItems(2) = %v", got)
	}
	for _, n := range []int{0, -1, 3, 10} {
		if got := g.GetPreviewItems(n); len(got) != 3 {
			t.Errorf("GetPreviewItems(%d): %d items, want all", n, len(got))
		}
		if got := g.GetRandomItems(n); len(got) != 3 {
			t.Errorf("GetRandomItems(%d): %d items, want all", n, len(got))
		}
	}
	random := g.GetRandomItems(2)
	if len(random) != 2 || random[0] == random[1] {
		t.Errorf("GetRandomItems(2) = %v", random)
	}
	for _, it := range random {
		if !slices.Contains(g.Items, it) {
			t.Errorf("GetRandomItems returned a foreign item %v", it)
		}
	}
	if g.Items[0] != first || g.Items[1] != cover || g.Items[2] != third {
		t.Error("GetRandomItems reordered the gallery's own items")
	}
}

func TestGalleryItemsHelpers(t *testing.T) {
	a := &modulekit.File{URI: "public://a.jpg"}
	items := GalleryItems{{File: a, Title: new("A"), Description: new("About A")}, {}}
	if items.Len() != 2 || items.IsEmpty() || !(GalleryItems{}).IsEmpty() {
		t.Error("Len/IsEmpty")
	}
	if got := items.GetFiles(); len(got) != 1 || got[0] != a {
		t.Errorf("GetFiles = %v (items without a file skipped)", got)
	}
	if got := items.GetURIs(); len(got) != 1 || got[0] != "public://a.jpg" {
		t.Errorf("GetURIs = %v", got)
	}
	if items.GetCover() != a {
		t.Error("GetCover: the first file without a marked cover")
	}

	withFile, bare := items[0], items[1]
	if withFile.GetTitle() != "A" || withFile.GetDescription() != "About A" || !withFile.HasFile() || withFile.GetFileURI() != "public://a.jpg" {
		t.Error("an item with a file")
	}
	if bare.GetTitle() != "" || bare.GetDescription() != "" || bare.HasFile() || bare.GetFileURI() != "" {
		t.Error("an item without a file")
	}
}

// TestGalleryPreviewLimit - the type's previews on the content's page,
// else a row of its grid.
func TestGalleryPreviewLimit(t *testing.T) {
	for _, c := range []struct {
		g    Gallery
		want int
	}{
		{Gallery{}, 5},
		{Gallery{IsPortrait: true}, 6},
		{Gallery{NodePreviewCount: 3}, 3},
		{Gallery{NodePreviewCount: 3, IsPortrait: true}, 3},
	} {
		if got := c.g.PreviewLimit(); got != c.want {
			t.Errorf("%+v: %d, want %d", c.g, got, c.want)
		}
	}
}
