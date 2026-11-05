package imagestyle

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"path/filepath"
	"strings"

	"github.com/gen2brain/webp"
)

// encodeThumbnail encodes img as WebP: graphics (a PNG or GIF source, or
// an image with transparency) lossless - sharp edges and flat colours stay
// as they are, and come out smaller that way; anything else (photos)
// lossy at quality, several times smaller than lossless.
func encodeThumbnail(img image.Image, quality int, sourceName string) ([]byte, error) {
	opts := webp.Options{Quality: quality, Method: 4}
	if graphicsSource(sourceName) || imageHasAlpha(img) {
		opts.Lossless = true
	}
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, opts); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeJPEG encodes img as JPEG at quality; transparent areas become
// white (JPEG has no alpha).
func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	b := img.Bounds()
	flat := image.NewRGBA(b)
	draw.Draw(flat, b, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(flat, b, img, b.Min, draw.Over)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, flat, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// graphicsSource - the source is a PNG or a GIF: drawn, not photographed.
func graphicsSource(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".gif":
		return true
	default:
		return false
	}
}

func imageHasAlpha(img image.Image) bool {
	b := img.Bounds()
	if b.Empty() {
		return false
	}

	dx := b.Dx()
	dy := b.Dy()
	if dx*dy <= 512*512 {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if pixelHasTransparency(img.At(x, y)) {
					return true
				}
			}
		}
		return false
	}

	steps := 12
	stepX := max(dx/steps, 1)
	stepY := max(dy/steps, 1)
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			if pixelHasTransparency(img.At(x, y)) {
				return true
			}
		}
	}
	return false
}

func pixelHasTransparency(c color.Color) bool {
	_, _, _, a := c.RGBA()
	return a < 0xffff
}
