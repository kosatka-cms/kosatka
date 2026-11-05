package imagestyle

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/gen2brain/webp"
)

func TestEncodeThumbnail_preservesAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 10, G: 20, B: 30, A: 0})
		}
	}

	data, err := encodeThumbnail(img, 85, "/tmp/icon.png")
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, a := decoded.At(1, 1).RGBA()
	if a == 0xffff {
		t.Fatal("expected transparent pixel in WebP output")
	}
}

func TestEncodeThumbnail_opaqueJPEGSource(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	data, err := encodeThumbnail(img, 85, "/tmp/photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty webp output")
	}
}

func TestGraphicsSource(t *testing.T) {
	for name, want := range map[string]bool{"/a/b.png": true, "/a/b.GIF": true, "/a/b.jpg": false, "/a/b.webp": false} {
		if graphicsSource(name) != want {
			t.Errorf("%s: %v", name, !want)
		}
	}
}

// TestEncodeThumbnail_lossyPhotoLosslessGraphics - a photo is encoded
// lossy (a VP8 frame) and its quality counts; a PNG source, or an image
// with transparency, lossless (VP8L).
func TestEncodeThumbnail_lossyPhotoLosslessGraphics(t *testing.T) {
	photo := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			photo.SetRGBA(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: uint8((x * y) % 256), A: 255})
		}
	}
	low, err := encodeThumbnail(photo, 40, "/a/photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	high, err := encodeThumbnail(photo, 95, "/a/photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(low[:40], []byte("VP8 ")) || len(low) >= len(high) {
		t.Errorf("photo: lossy %v, quality 40 %d B, 95 %d B", bytes.Contains(low[:40], []byte("VP8 ")), len(low), len(high))
	}
	graphic, err := encodeThumbnail(photo, 85, "/a/logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(graphic[:40], []byte("VP8L")) {
		t.Error("a PNG source isn't lossless")
	}
}

func TestEncodeThumbnail_roundTripPNGFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alpha.png")
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{A: 0})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	opened, err := png.Decode(bytes.NewReader(mustReadFile(t, path)))
	if err != nil {
		t.Fatal(err)
	}
	data, err := encodeThumbnail(opened, 85, path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, a := decoded.At(0, 0).RGBA()
	if a == 0xffff {
		t.Fatal("expected transparency preserved from PNG source")
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestEncodeJPEG_flattensAlphaOnWhite - a JPEG style's output is a real
// JPEG; transparent pixels become white.
func TestEncodeJPEG_flattensAlphaOnWhite(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetNRGBA(x, y, color.NRGBA{A: 0})
		}
	}
	data, err := encodeJPEG(img, 90)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not a JPEG: %v", err)
	}
	r, g, b, _ := decoded.At(4, 4).RGBA()
	if r>>8 < 250 || g>>8 < 250 || b>>8 < 250 {
		t.Fatalf("transparent pixel should be white, got %d %d %d", r>>8, g>>8, b>>8)
	}
}

// TestProcess_jpegStyle - Process encodes per the style's format.
func TestProcess_jpegStyle(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	var src bytes.Buffer
	if err := png.Encode(&src, img); err != nil {
		t.Fatal(err)
	}

	style, err := NewImageStyle("feed", "Feed", 20, 0, ResizeModeFit, imaging.Center, 85)
	if err != nil {
		t.Fatal(err)
	}
	style.SetFormat(FormatJPEG)
	out, err := NewImagingProcessor().Process(&src, "src.png", style, style.Anchor(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.ContentType != "image/jpeg" || out.Extension != ".jpg" {
		t.Fatalf("got %s %s", out.ContentType, out.Extension)
	}
	if _, err := jpeg.Decode(bytes.NewReader(out.Data)); err != nil {
		t.Fatalf("not a JPEG: %v", err)
	}
	if ParseFormat("bogus") != FormatWebP || ParseFormat("jpeg") != FormatJPEG {
		t.Fatal("ParseFormat")
	}
}
