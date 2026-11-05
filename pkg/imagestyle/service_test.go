package imagestyle

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

type namedStyles map[string]*ImageStyle

func (n namedStyles) FindByName(name string) (*ImageStyle, error) {
	if s, ok := n[name]; ok {
		return s, nil
	}
	return nil, ErrStyleNotFound
}
func (namedStyles) FindAll() ([]*ImageStyle, error) { return nil, nil }
func (namedStyles) Save(*ImageStyle) error          { return nil }
func (namedStyles) Delete(string) error             { return nil }

// TestGetStyledOriginal - a derivative made from the original at its key
// exactly, cached under its whole name: a.png and a.jpg side by side get
// derivatives of their own; a missing original is ErrOriginalNotFound.
func TestGetStyledOriginal(t *testing.T) {
	ctx := context.Background()
	store := filestore.NewLocal(t.TempDir())
	put := func(key string, w, h int) {
		var buf bytes.Buffer
		if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Put(ctx, key, &buf, filestore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	put("img/a.png", 40, 20)
	put("img/a.jpg", 20, 40) // another file, another shape
	style, err := NewImageStyle("card", "Card", 10, 0, ResizeModeFit, imaging.Center, 80)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewStyleService(namedStyles{"card": style}, NewStorageRepository(store), NewImagingProcessor())
	f, info, err := svc.GetStyledOriginal(ctx, "card", false, "img/a.png")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if info.Key != "styles/card/img/a.png.webp" {
		t.Errorf("derivative key = %q", info.Key)
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width != 10 || cfg.Height != 5 {
		t.Errorf("derivative of a.png: %dx%d %v (made from another file?)", cfg.Width, cfg.Height, err)
	}
	if _, _, err := svc.GetStyledOriginal(ctx, "card", false, "img/missing.png"); !errors.Is(err, ErrOriginalNotFound) {
		t.Errorf("missing original: %v", err)
	}
	if _, _, err := svc.GetStyledOriginal(ctx, "nope", false, "img/a.png"); !errors.Is(err, ErrStyleNotFound) {
		t.Errorf("unknown style: %v", err)
	}
}

// TestStyledDouble - a HiDPI style's 2x derivative: under @2x/ of its
// folder, at double size; of a style without it - ErrStyleNotFound; a
// private original's too.
func TestStyledDouble(t *testing.T) {
	ctx := context.Background()
	store := filestore.NewLocal(t.TempDir())
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 400, 300))); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(ctx, "img/a.png", &buf, filestore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	card, err := NewImageStyle("card", "Card", 100, 50, ResizeModeFill, imaging.Center, 80)
	if err != nil {
		t.Fatal(err)
	}
	card.SetHiDPI(true)
	plain, _ := NewImageStyle("plain", "Plain", 100, 50, ResizeModeFill, imaging.Center, 80)
	svc := NewStyleService(namedStyles{"card": card, "plain": plain}, NewStorageRepository(store), NewImagingProcessor())
	size := func(f filestore.File) (int, int) {
		t.Helper()
		defer func() { _ = f.Close() }()
		cfg, _, err := image.DecodeConfig(f)
		if err != nil {
			t.Fatal(err)
		}
		return cfg.Width, cfg.Height
	}

	f, info, err := svc.GetStyledImage(ctx, "card", DoublePath("img/a.webp"))
	if err != nil {
		t.Fatal(err)
	}
	if w, h := size(f); info.Key != "styles/card/@2x/img/a.webp" || w != 200 || h != 100 {
		t.Errorf("the 2x derivative: %s %dx%d", info.Key, w, h)
	}
	if _, _, err := svc.GetStyledImage(ctx, "plain", DoublePath("img/a.webp")); !errors.Is(err, ErrStyleNotFound) {
		t.Errorf("a style without 2x: %v", err)
	}
	f, info, err = svc.GetStyledOriginal(ctx, "card", true, "img/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if w, h := size(f); info.Key != "styles/card/@2x/img/a.png.webp" || w != 200 || h != 100 {
		t.Errorf("the private 2x derivative: %s %dx%d", info.Key, w, h)
	}
}

// focalOf - FocalPoints with one point, for the key it's asked about.
type focalOf struct {
	point *FocalPoint
	asked *string
}

func (f focalOf) FocalPoint(_ context.Context, key string) (*FocalPoint, error) {
	*f.asked = key
	return f.point, nil
}

// TestStyledFocal - a style cropping at the focal point crops around the
// original's point: a white square at the right edge of a black image
// is in the middle of the crop; without a point - the center (black);
// a style without it never asks.
func TestStyledFocal(t *testing.T) {
	ctx := context.Background()
	store := filestore.NewLocal(t.TempDir())
	img := image.NewRGBA(image.Rect(0, 0, 400, 100))
	for x := 360; x < 400; x++ {
		for y := 30; y < 70; y++ {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(ctx, "img/a.png", &buf, filestore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	focal, _ := NewImageStyle("square", "Square", 100, 100, ResizeModeFill, imaging.Center, 90)
	focal.SetFocal(true)
	plain, _ := NewImageStyle("plain", "Plain", 100, 100, ResizeModeFill, imaging.Center, 90)
	var asked string
	point := &FocalPoint{X: 0.95, Y: 0.5}
	svc := NewStyleService(namedStyles{"square": focal, "plain": plain}, NewStorageRepository(store), NewImagingProcessor())
	svc.SetFocalPoints(focalOf{point: point, asked: &asked})
	middle := func(style string) uint32 {
		t.Helper()
		f, _, err := svc.GetStyledImage(ctx, style, "img/a.webp")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		out, _, err := image.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		r, _, _, _ := out.At(85, 50).RGBA()
		return r
	}
	if r := middle("square"); asked != "img/a.png" || r < 0x8000 {
		t.Errorf("cropped at the point: asked %q, red %x", asked, r)
	}
	asked = ""
	if r := middle("plain"); asked != "" || r > 0x8000 {
		t.Errorf("a style without the point: asked %q, red %x", asked, r)
	}
}

// countingProcessor fails every render, counting them.
type countingProcessor struct{ calls int }

func (p *countingProcessor) Process(io.Reader, string, *ImageStyle, imaging.Anchor, *FocalPoint) (ProcessedImage, error) {
	p.calls++
	return ProcessedImage{}, ErrNotRenderable
}

// TestFailedOriginalNotRenderedAgain - an original that can't be rendered
// isn't decoded on the next request, until it changes.
func TestFailedOriginalNotRenderedAgain(t *testing.T) {
	ctx := context.Background()
	store := filestore.NewLocal(t.TempDir())
	if _, err := store.Put(ctx, "img/a.png", bytes.NewReader([]byte("not an image")), filestore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	style, err := NewImageStyle("card", "Card", 10, 0, ResizeModeFit, imaging.Center, 80)
	if err != nil {
		t.Fatal(err)
	}
	proc := &countingProcessor{}
	svc := NewStyleService(namedStyles{"card": style}, NewStorageRepository(store), proc)
	for range 3 {
		if _, _, err := svc.GetStyledImage(ctx, "card", "img/a.webp"); !errors.Is(err, ErrNotRenderable) {
			t.Fatalf("got %v, want ErrNotRenderable", err)
		}
	}
	if proc.calls != 1 {
		t.Fatalf("rendered %d times, want 1", proc.calls)
	}
	if _, err := store.Put(ctx, "img/a.png", bytes.NewReader([]byte("still not an image")), filestore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	_, _, _ = svc.GetStyledImage(ctx, "card", "img/a.webp")
	if proc.calls != 2 {
		t.Fatalf("a changed original: rendered %d times, want 2", proc.calls)
	}
}

// TestFailedOriginalsBounded - the memory of failures stays within
// failedKeep entries.
func TestFailedOriginalsBounded(t *testing.T) {
	var f failedOriginals
	for i := range failedKeep + 10 {
		f.add(filestore.Info{Key: "k", Size: int64(i)})
	}
	if len(f.seen) > failedKeep {
		t.Fatalf("%d entries", len(f.seen))
	}
	if !f.has(filestore.Info{Key: "k", Size: failedKeep + 9}) {
		t.Fatal("the last failure is forgotten")
	}
}
