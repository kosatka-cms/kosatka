package imagestyle

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"math"

	"github.com/disintegration/imaging"
)

// The largest source the processor decodes: a decoded image takes 4 bytes
// a pixel, and a small file may claim any size in its header.
const (
	MaxSourceSide   = 20000
	MaxSourcePixels = 50_000_000
)

// renderSlots - how many sources are decoded at once: a derivative is
// rendered on the first request for it, by anyone.
const renderSlots = 2

// ImagingProcessor implements ImageProcessor with the imaging library.
type ImagingProcessor struct {
	slots chan struct{}
}

// NewImagingProcessor - an ImagingProcessor rendering renderSlots images
// at a time.
func NewImagingProcessor() *ImagingProcessor {
	return &ImagingProcessor{slots: make(chan struct{}, renderSlots)}
}

// Process renders the derivative; ErrNotRenderable when the source isn't
// an image it decodes or is larger than MaxSourceSide/MaxSourcePixels.
func (p *ImagingProcessor) Process(r io.Reader, sourceName string, style *ImageStyle, anchor imaging.Anchor, focal *FocalPoint) (ProcessedImage, error) {
	// The header is read first; the decoder then reads it again from head.
	var head bytes.Buffer
	cfg, _, err := image.DecodeConfig(io.TeeReader(r, &head))
	if err != nil {
		return ProcessedImage{}, fmt.Errorf("%w: %w", ErrNotRenderable, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > MaxSourceSide || cfg.Height > MaxSourceSide ||
		int64(cfg.Width)*int64(cfg.Height) > MaxSourcePixels {
		return ProcessedImage{}, fmt.Errorf("%w: %d×%d px", ErrNotRenderable, cfg.Width, cfg.Height)
	}
	p.slots <- struct{}{}
	defer func() { <-p.slots }()
	src, err := imaging.Decode(io.MultiReader(&head, r))
	if err != nil {
		return ProcessedImage{}, fmt.Errorf("%w: %w", ErrNotRenderable, err)
	}

	var result image.Image

	switch style.Mode() {
	case ResizeModeFill:
		switch {
		case style.Height() == 0:
			aspectRatio := float64(src.Bounds().Dy()) / float64(src.Bounds().Dx())
			height := int(float64(style.Width()) * aspectRatio)
			result = imaging.Fill(src, style.Width(), height, anchor, imaging.Lanczos)
		case focal != nil:
			result = fillAt(src, style.Width(), style.Height(), *focal)
		default:
			result = imaging.Fill(src, style.Width(), style.Height(), anchor, imaging.Lanczos)
		}
	case ResizeModeFit:
		if style.Height() == 0 {
			result = imaging.Resize(src, style.Width(), 0, imaging.Lanczos)
		} else {
			result = imaging.Fit(src, style.Width(), style.Height(), imaging.Lanczos)
		}
	case ResizeModeResize:
		if style.Height() == 0 {
			result = imaging.Resize(src, style.Width(), 0, imaging.Lanczos)
		} else {
			result = imaging.Resize(src, style.Width(), style.Height(), imaging.Lanczos)
		}
	default:
		result = imaging.Fill(src, style.Width(), style.Height(), imaging.Center, imaging.Lanczos)
	}

	if style.Format() == FormatJPEG {
		data, err := encodeJPEG(result, style.Quality())
		if err != nil {
			return ProcessedImage{}, fmt.Errorf("failed to encode jpeg: %w", err)
		}
		return newProcessedImage(data, FormatJPEG), nil
	}

	data, err := encodeThumbnail(result, style.Quality(), sourceName)
	if err != nil {
		return ProcessedImage{}, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	return NewProcessedImage(data), nil
}

// fillAt - src cropped to the proportions of width × height around
// focal (the crop centered on it, as far as the image allows), scaled to
// width × height.
func fillAt(src image.Image, width, height int, focal FocalPoint) image.Image {
	b := src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	scale := max(float64(width)/sw, float64(height)/sh)
	cw := min(b.Dx(), max(1, int(math.Round(float64(width)/scale))))
	ch := min(b.Dy(), max(1, int(math.Round(float64(height)/scale))))
	x := clampInt(int(math.Round(focal.X*sw-float64(cw)/2)), 0, b.Dx()-cw)
	y := clampInt(int(math.Round(focal.Y*sh-float64(ch)/2)), 0, b.Dy()-ch)
	crop := imaging.Crop(src, image.Rect(b.Min.X+x, b.Min.Y+y, b.Min.X+x+cw, b.Min.Y+y+ch))
	return imaging.Resize(crop, width, height, imaging.Lanczos)
}

func clampInt(v, lo, hi int) int {
	return max(lo, min(v, hi))
}
