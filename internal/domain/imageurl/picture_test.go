package imageurl

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

// useStyleSizes gives the styles their sizes for the test.
func useStyleSizes(t *testing.T, sizes map[string]StyleSize) {
	t.Helper()
	SetStyleSizes(sizes)
	t.Cleanup(func() { SetStyleSizes(nil) })
}

// TestStyledPicture - a style's picture: its size as the style makes it
// of the original; a HiDPI style's 2x derivative in srcset, unless the
// original is no larger than the usual one.
func TestStyledPicture(t *testing.T) {
	useStyleSizes(t, map[string]StyleSize{
		"card":  {Width: 720, Height: 405, Mode: imagestyle.ResizeModeFill, HiDPI: true},
		"wide":  {Width: 1200, Mode: imagestyle.ResizeModeFit, HiDPI: true},
		"box":   {Width: 400, Height: 400, Mode: imagestyle.ResizeModeFit, HiDPI: true},
		"plain": {Width: 300, Height: 200, Mode: imagestyle.ResizeModeFill},
	})
	SetStyleVersion("card", "ab12")
	t.Cleanup(func() { SetStyleVersion("card", "") })
	photo := entity.File{URI: "public://content/a.jpg", Width: 2000, Height: 1000}

	want := Picture{
		Src:    "/uploads/styles/card/content/a.webp?v=ab12",
		Srcset: "/uploads/styles/card/content/a.webp?v=ab12 1x, /uploads/styles/card/@2x/content/a.webp?v=ab12 2x",
		Width:  720, Height: 405,
	}
	if got := StyledPicture(photo, "card"); got != want {
		t.Errorf("card:\n got %+v\nwant %+v", got, want)
	}
	if got := StyledPicture(photo, "wide"); got.Width != 1200 || got.Height != 600 || got.Srcset == "" {
		t.Errorf("wide: %+v", got)
	}
	// A fit box: the derivative fits it, keeping the proportions.
	if got := StyledPicture(photo, "box"); got.Width != 400 || got.Height != 200 || got.Srcset == "" {
		t.Errorf("box: %+v", got)
	}
	// An original no larger than the usual derivative: no 2x.
	small := entity.File{URI: "public://content/b.jpg", Width: 300, Height: 200}
	if got := StyledPicture(small, "box"); got.Srcset != "" || got.Width != 300 || got.Height != 200 {
		t.Errorf("a small original in a box: %+v", got)
	}
	if got := StyledPicture(entity.File{URI: "public://content/b.jpg", Width: 720, Height: 405}, "card"); got.Srcset != "" {
		t.Errorf("an original the card's size: %+v", got)
	}
	// The original's size unknown: the 2x offered.
	if got := StyledPicture(entity.File{URI: "public://content/c.jpg"}, "box"); got.Srcset == "" || got.Width != 0 {
		t.Errorf("box of an unknown original: %+v", got)
	}
	if got := StyledPicture(photo, "plain"); got.Srcset != "" || got.Width != 300 || got.Height != 200 {
		t.Errorf("a style without 2x: %+v", got)
	}
	if got := StyledPicture(photo, "unknown"); got.Src != "/uploads/styles/unknown/content/a.webp" || got.Width != 0 {
		t.Errorf("an unknown style: %+v", got)
	}
	if got := StyledPicture(entity.File{}, "card"); got != (Picture{}) {
		t.Errorf("no image: %+v", got)
	}
	if got := OriginalPicture(photo); got != (Picture{Src: "/uploads/content/a.jpg", Width: 2000, Height: 1000}) {
		t.Errorf("OriginalPicture = %+v", got)
	}
}

// TestPrivateStyledDouble - a private image's 2x derivative: under the
// style's @2x/, read back as such.
func TestPrivateStyledDouble(t *testing.T) {
	useStyleSizes(t, map[string]StyleSize{"card": {Width: 100, Height: 50, Mode: imagestyle.ResizeModeFill, HiDPI: true}})
	got := StyledPicture(entity.File{URI: "private://content/a.png", Width: 400, Height: 200}, "card")
	if got.Srcset != "/files/private/styles/card/content/a.png 1x, /files/private/styles/card/@2x/content/a.png 2x" {
		t.Fatalf("srcset = %q", got.Srcset)
	}
	if style, double, key, ok := ParsePrivateStyled("styles/card/@2x/content/a.png"); !ok || style != "card" || !double || key != "content/a.png" {
		t.Errorf("ParsePrivateStyled = %q %v %q %v", style, double, key, ok)
	}
}

// TestStyledFocal - in a style cropping at the focal point, an image
// with a point gets an address with it (f=<x>-<y>), its 2x too; in
// another style, or without a point - none.
func TestStyledFocal(t *testing.T) {
	useStyleSizes(t, map[string]StyleSize{
		"card":  {Width: 720, Height: 405, Mode: imagestyle.ResizeModeFill, HiDPI: true, Focal: true},
		"plain": {Width: 300, Height: 200, Mode: imagestyle.ResizeModeFill},
	})
	SetStyleVersion("card", "ab12")
	t.Cleanup(func() { SetStyleVersion("card", "") })
	photo := entity.File{URI: "public://content/a.jpg", Width: 2000, Height: 1000, Focal: &entity.FocalPoint{X: 30, Y: 70}}
	want := "/uploads/styles/card/content/a.webp?v=ab12&f=30-70 1x, /uploads/styles/card/@2x/content/a.webp?v=ab12&f=30-70 2x"
	if got := StyledPicture(photo, "card"); got.Srcset != want {
		t.Errorf("srcset:\n got %s\nwant %s", got.Srcset, want)
	}
	if got := Styled(photo, "plain").URI; got != "/uploads/styles/plain/content/a.webp" {
		t.Errorf("a style without the point: %s", got)
	}
	photo.Focal = nil
	if got := Styled(photo, "card").URI; got != "/uploads/styles/card/content/a.webp?v=ab12" {
		t.Errorf("an image without a point: %s", got)
	}
}

// TestLightboxPicture - the lightbox opens a large photo's copy in the
// lightbox style at its fitted size, a small one's at its own size; the
// original where there is no such style, its size is unknown, or it is
// a GIF or an SVG.
func TestLightboxPicture(t *testing.T) {
	photo := entity.File{URI: "public://content/a.jpg", Width: 4000, Height: 3000}
	if got := LightboxPicture(photo); got.Src != "/uploads/content/a.jpg" || got.Width != 4000 {
		t.Errorf("no style: %+v", got)
	}
	useStyleSizes(t, map[string]StyleSize{entity.LightboxStyle: {Width: 2048, Height: 2048, Mode: imagestyle.ResizeModeFit, HiDPI: true}})
	if got := LightboxPicture(photo); got.Src != "/uploads/styles/lightbox/content/a.webp" || got.Width != 2048 || got.Height != 1536 || got.Srcset != "" {
		t.Errorf("large: %+v", got)
	}
	small := entity.File{URI: "public://content/b.png", Width: 600, Height: 260}
	if got := LightboxPicture(small); got.Src != "/uploads/styles/lightbox/content/b.webp" || got.Width != 600 || got.Height != 260 {
		t.Errorf("small: %+v", got)
	}
	for _, f := range []entity.File{
		{URI: "public://content/c.gif", Width: 4000, Height: 3000},
		{URI: "public://content/d.svg", Width: 4000, Height: 3000},
		{URI: "public://content/e.jpg"},
	} {
		if got := LightboxPicture(f); got.Src != Original(f).URI {
			t.Errorf("%s: %+v, want the original", f.URI, got)
		}
	}
	if got := LightboxPicture(entity.File{}); got != (Picture{}) {
		t.Errorf("no image: %+v", got)
	}
}
