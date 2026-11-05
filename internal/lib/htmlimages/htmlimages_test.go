package htmlimages

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

func TestProcessImages_dimensionsFromStyle(t *testing.T) {
	cp := NewContentProcessor(imagestyle.NewResizeService(imagestyle.NewImagingProcessor(), filestore.NewLocal(t.TempDir()), "/uploads/", ""))

	in := `<p><img src="/uploads/u2793/invincible.jpg" style="float:right; height:455px; margin:2px 5px; width:300px" alt=""></p>`
	out := cp.ProcessImages(in)

	if !strings.Contains(out, `width="300"`) {
		t.Fatalf("expected width attribute, got: %s", out)
	}
	if !strings.Contains(out, `height="455"`) {
		t.Fatalf("expected height attribute, got: %s", out)
	}
}

func TestProcessImages_existingAttrsAndResize(t *testing.T) {
	base := t.TempDir()
	imgPath := filepath.Join(base, "foo.jpg")
	img := imaging.New(400, 600, color.White)
	if err := imaging.Save(img, imgPath, imaging.JPEGQuality(85)); err != nil {
		t.Fatal(err)
	}

	cp := NewContentProcessor(imagestyle.NewResizeService(imagestyle.NewImagingProcessor(), filestore.NewLocal(base), "/uploads/", ""))
	src := "/uploads/foo.jpg"
	in := `<img src="` + src + `" width="220" height="335" alt="test">`
	out := cp.ProcessImages(in)

	if !strings.Contains(out, `width="220"`) || !strings.Contains(out, `height="335"`) {
		t.Fatalf("expected width/height attributes, got: %s", out)
	}
	if !strings.Contains(out, `foo-220x335.webp`) {
		t.Fatalf("expected resized src, got: %s", out)
	}
	if !strings.Contains(out, `class="content-image-link"`) {
		t.Fatalf("expected lightbox link wrapper, got: %s", out)
	}

	resizedPath := filepath.Join(base, "resize", "foo-220x335.webp")
	if _, err := os.Stat(resizedPath); err != nil {
		t.Fatalf("resized file not created: %v", err)
	}
}

// TestProcessImages_onlyWidthDerivesHeightFromAspectRatio: an image resized
// in the editor (only width="300", no height) keeps its width and gets the
// height from the original's aspect ratio instead of the full native size.
func TestProcessImages_onlyWidthDerivesHeightFromAspectRatio(t *testing.T) {
	base := t.TempDir()
	imgPath := filepath.Join(base, "foo.jpg")
	img := imaging.New(400, 600, color.White) // native aspect ratio 400:600 = 2:3
	if err := imaging.Save(img, imgPath, imaging.JPEGQuality(85)); err != nil {
		t.Fatal(err)
	}

	cp := NewContentProcessor(imagestyle.NewResizeService(imagestyle.NewImagingProcessor(), filestore.NewLocal(base), "/uploads/", ""))
	src := "/uploads/foo.jpg"
	in := `<img src="` + src + `" width="300" alt="test">`
	out := cp.ProcessImages(in)

	if !strings.Contains(out, `width="300"`) {
		t.Fatalf("expected the given width to survive, got: %s", out)
	}
	if !strings.Contains(out, `height="450"`) {
		t.Fatalf("expected height derived from the original 400x600 aspect ratio (300*600/400=450), got: %s", out)
	}
}

func TestProcessImages_onlyHeightDerivesWidthFromAspectRatio(t *testing.T) {
	base := t.TempDir()
	imgPath := filepath.Join(base, "foo.jpg")
	img := imaging.New(400, 600, color.White)
	if err := imaging.Save(img, imgPath, imaging.JPEGQuality(85)); err != nil {
		t.Fatal(err)
	}

	cp := NewContentProcessor(imagestyle.NewResizeService(imagestyle.NewImagingProcessor(), filestore.NewLocal(base), "/uploads/", ""))
	src := "/uploads/foo.jpg"
	in := `<img src="` + src + `" height="300" alt="test">`
	out := cp.ProcessImages(in)

	if !strings.Contains(out, `height="300"`) {
		t.Fatalf("expected the given height to survive, got: %s", out)
	}
	if !strings.Contains(out, `width="200"`) {
		t.Fatalf("expected width derived from the original 400x600 aspect ratio (300*400/600=200), got: %s", out)
	}
}

// TestProcessImages_lightboxOnlyForLarger - an image shown at its
// original's size (or larger) gets its preview but no lightbox link:
// there is nothing more to see; a smaller one keeps the link.
func TestProcessImages_lightboxOnlyForLarger(t *testing.T) {
	base := t.TempDir()
	if err := imaging.Save(imaging.New(600, 260, color.White), filepath.Join(base, "same.jpg"), imaging.JPEGQuality(85)); err != nil {
		t.Fatal(err)
	}
	cp := NewContentProcessor(imagestyle.NewResizeService(imagestyle.NewImagingProcessor(), filestore.NewLocal(base), "/uploads/", ""))
	for _, tc := range []struct {
		in   string
		link bool
	}{
		{`<img src="/uploads/same.jpg" width="600" height="260" alt="">`, false},
		{`<img src="/uploads/same.jpg" style="width: 600px; height: 260px;" alt="">`, false},
		{`<img src="/uploads/same.jpg" width="300" alt="">`, true},
	} {
		out := cp.ProcessImages(tc.in)
		if strings.Contains(out, "content-image-link") != tc.link || !strings.Contains(out, ".webp") {
			t.Errorf("%s → %s (link wanted: %v)", tc.in, out, tc.link)
		}
	}
}

// TestProcessImages_lightbox - with SetLightbox the link opens what it
// gives (a large copy, its size); where it declines, the original.
func TestProcessImages_lightbox(t *testing.T) {
	base := t.TempDir()
	if err := imaging.Save(imaging.New(3000, 2000, color.White), filepath.Join(base, "big.jpg"), imaging.JPEGQuality(85)); err != nil {
		t.Fatal(err)
	}
	cp := NewContentProcessor(imagestyle.NewResizeService(imagestyle.NewImagingProcessor(), filestore.NewLocal(base), "/uploads/", ""))
	var seen string
	cp.SetLightbox(func(src string, w, h int) (string, int, int, bool) {
		seen = fmt.Sprintf("%s %dx%d", src, w, h)
		return "/uploads/styles/lightbox/big.webp", 2048, 1365, true
	})
	out := cp.ProcessImages(`<img src="/uploads/big.jpg" width="600" alt="">`)
	if seen != "/uploads/big.jpg 3000x2000" || !strings.Contains(out, `href="/uploads/styles/lightbox/big.webp"`) || !strings.Contains(out, `data-pswp-width="2048"`) || !strings.Contains(out, `data-pswp-height="1365"`) {
		t.Errorf("asked %q, got %s", seen, out)
	}
	cp = NewContentProcessor(imagestyle.NewResizeService(imagestyle.NewImagingProcessor(), filestore.NewLocal(base), "/uploads/", ""))
	cp.SetLightbox(func(string, int, int) (string, int, int, bool) { return "", 0, 0, false })
	if out := cp.ProcessImages(`<img src="/uploads/big.jpg" width="600" alt="">`); !strings.Contains(out, `href="/uploads/big.jpg"`) || !strings.Contains(out, `data-pswp-width="3000"`) {
		t.Errorf("declined: %s", out)
	}
}

func TestProcessImages_noDimensionsUnchanged(t *testing.T) {
	cp := NewContentProcessor(imagestyle.NewResizeService(imagestyle.NewImagingProcessor(), filestore.NewLocal(t.TempDir()), "/uploads/", ""))

	in := `<img src="/uploads/foo.jpg" alt="test">`
	out := cp.ProcessImages(in)

	if strings.Contains(out, `width="`) || strings.Contains(out, `height="`) {
		t.Fatalf("expected no dimension attributes, got: %s", out)
	}
	if strings.Contains(out, `content-image-link`) {
		t.Fatalf("expected no link wrapper, got: %s", out)
	}
}

// TestProcessImagesKeepsEmbedsAndBreakMarker - the render-time pass keeps
// what the save-time sanitizer let through: an iframe embed and the
// <!--break--> cut.
func TestProcessImagesKeepsEmbedsAndBreakMarker(t *testing.T) {
	in := `<p>Teaser</p><!--break--><p><iframe width="560" height="315" src="https://www.youtube.com/embed/abc" allowfullscreen=""></iframe></p>`
	out := (&ContentProcessor{}).ProcessImages(in)
	for _, want := range []string{`<!--break-->`, `<iframe width="560" height="315" src="https://www.youtube.com/embed/abc" allowfullscreen=""></iframe>`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}

// TestProcessImages_storeURL - with a store serving its files itself, the
// preview and the link to the original use its address.
func TestProcessImages_storeURL(t *testing.T) {
	base := t.TempDir()
	if err := imaging.Save(imaging.New(400, 600, color.White), filepath.Join(base, "foo.jpg"), imaging.JPEGQuality(85)); err != nil {
		t.Fatal(err)
	}
	cp := NewContentProcessor(imagestyle.NewResizeService(imagestyle.NewImagingProcessor(), filestore.NewLocal(base), "/uploads/", "https://cdn.example.com/f/"))
	out := cp.ProcessImages(`<img src="/uploads/foo.jpg" width="200" height="300">`)
	if !strings.Contains(out, `src="https://cdn.example.com/f/resize/foo-200x300.webp"`) || !strings.Contains(out, `href="https://cdn.example.com/f/foo.jpg"`) {
		t.Fatalf("got: %s", out)
	}
}

// TestProcessImagesKeepsTheText - a text rewritten once is answered again
// without the store (its images gone meanwhile change nothing) until
// Forget; a text whose image failed isn't kept: once the image is there,
// the next call rewrites it.
func TestProcessImagesKeepsTheText(t *testing.T) {
	base := t.TempDir()
	imgPath := filepath.Join(base, "foo.jpg")
	if err := imaging.Save(imaging.New(400, 600, color.White), imgPath, imaging.JPEGQuality(85)); err != nil {
		t.Fatal(err)
	}
	cp := NewContentProcessor(imagestyle.NewResizeService(imagestyle.NewImagingProcessor(), filestore.NewLocal(base), "/uploads/", ""))
	in := `<p><img src="/uploads/foo.jpg" width="200"></p>`
	first := cp.ProcessImages(in)
	if !strings.Contains(first, "foo-200x300.webp") {
		t.Fatalf("rewritten: %s", first)
	}
	if err := os.RemoveAll(base); err != nil {
		t.Fatal(err)
	}
	if again := cp.ProcessImages(in); again != first {
		t.Errorf("kept: %s, want %s", again, first)
	}
	cp.Forget()
	if forgotten := cp.ProcessImages(in); strings.Contains(forgotten, "200x300") {
		t.Errorf("after Forget, without the image: %s", forgotten)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := imaging.Save(imaging.New(400, 600, color.White), imgPath, imaging.JPEGQuality(85)); err != nil {
		t.Fatal(err)
	}
	if later := cp.ProcessImages(in); !strings.Contains(later, "foo-200x300.webp") {
		t.Errorf("the failed text was kept: %s", later)
	}
}
