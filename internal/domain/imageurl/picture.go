package imageurl

import (
	"fmt"
	"math"
	"path"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

// StyleSize - what the <img> of a style's derivative is built from
// (StyledPicture): the style's size and mode, whether it has 2x
// derivatives and crops at the image's focal point (entity.ImageStyleDef).
type StyleSize struct {
	Width, Height int
	Mode          imagestyle.ResizeMode
	HiDPI         bool
	Focal         bool
}

// withFocal - u, the address of image's derivative in styleName, with
// the image's focal point when the style crops at it (f=<x>-<y>): a new
// point gives new addresses, so browsers and CDNs, which keep a
// derivative for a year, fetch the new crop.
func withFocal(u string, image entity.File, styleName string) string {
	sz, ok := styleSize(styleName)
	if !ok || !sz.Focal || image.Focal == nil {
		return u
	}
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%sf=%d-%d", u, sep, image.Focal.X, image.Focal.Y)
}

// styleSizes - image style name → its StyleSize. Kept in sync by the
// image style service like styleFormats; a style not listed gets an
// <img> without its size and other widths.
var (
	styleSizesMu sync.RWMutex
	styleSizes   = map[string]StyleSize{}
)

// SetStyleSizes replaces the whole style → size map.
func SetStyleSizes(sizes map[string]StyleSize) {
	styleSizesMu.Lock()
	defer styleSizesMu.Unlock()
	styleSizes = make(map[string]StyleSize, len(sizes))
	for name, s := range sizes {
		styleSizes[name] = s
	}
}

// SetStyleSize records one style's size (a zero StyleSize removes the
// entry).
func SetStyleSize(name string, s StyleSize) {
	styleSizesMu.Lock()
	defer styleSizesMu.Unlock()
	if s.Width == 0 {
		delete(styleSizes, name)
		return
	}
	styleSizes[name] = s
}

// HasStyle - whether an image style of that name exists (SetStyleSizes).
func HasStyle(name string) bool {
	_, ok := styleSize(name)
	return ok
}

func styleSize(name string) (StyleSize, bool) {
	styleSizesMu.RLock()
	defer styleSizesMu.RUnlock()
	s, ok := styleSizes[name]
	return s, ok
}

// Picture - what an image's <img> is built from (the template function
// img): its address, the 2x one beside it (Srcset), its size in pixels
// (0 - unknown).
type Picture struct {
	Src    string
	Srcset string
	Width  int
	Height int
}

// StyledPicture - image in styleName: its derivative's address (Styled)
// and size, and for a HiDPI style the 2x derivative (srcset) - unless the
// original is no larger than the derivative. No image - an empty
// Picture.
func StyledPicture(image entity.File, styleName string) Picture {
	if image.URI == "" {
		return Picture{}
	}
	p := Picture{Src: withFocal(styledURL(image.URI, styleName, false), image, styleName)}
	sz, ok := styleSize(styleName)
	if !ok {
		return p
	}
	p.Width, p.Height = derivativeSize(sz.Mode, sz.Width, sz.Height, image.Width, image.Height)
	if sz.HiDPI && (image.Width <= 0 || p.Width <= 0 || image.Width > p.Width) {
		p.Srcset = p.Src + " 1x, " + withFocal(styledURL(image.URI, styleName, true), image, styleName) + " 2x"
	}
	return p
}

// OriginalPicture - the image itself (Original) and its size. No image -
// an empty Picture.
func OriginalPicture(image entity.File) Picture {
	if image.URI == "" {
		return Picture{}
	}
	return Picture{Src: Original(image).URI, Width: image.Width, Height: image.Height}
}

// LightboxPicture - what the lightbox opens of image: its copy in
// entity.LightboxStyle (a large WebP) and that copy's size; the original
// itself where the site has no such style, the original's size isn't
// known (nor then the copy's), or a copy would lose what the original
// has (a GIF's animation, an SVG's vector). No image - an empty Picture.
func LightboxPicture(image entity.File) Picture {
	if image.URI == "" {
		return Picture{}
	}
	ext := strings.ToLower(path.Ext(strings.SplitN(image.URI, "?", 2)[0]))
	if image.Width > 0 && image.Height > 0 && ext != ".gif" && ext != ".svg" && HasStyle(entity.LightboxStyle) {
		if p := StyledPicture(image, entity.LightboxStyle); p.Width > 0 && p.Height > 0 {
			p.Srcset = ""
			return p
		}
	}
	return OriginalPicture(image)
}

// derivativeSize - the size of the derivative a style of mode and size
// width × height makes of an original ow × oh (0 - unknown), as
// imagestyle.ImagingProcessor makes it; 0 where it can't be told.
func derivativeSize(mode imagestyle.ResizeMode, width, height, ow, oh int) (int, int) {
	if height == 0 {
		if ow <= 0 || oh <= 0 {
			return width, 0
		}
		return width, max(1, int(math.Round(float64(width)*float64(oh)/float64(ow))))
	}
	if mode != imagestyle.ResizeModeFit {
		return width, height
	}
	if ow <= 0 || oh <= 0 {
		return 0, 0
	}
	if ow <= width && oh <= height {
		return ow, oh
	}
	scale := min(float64(width)/float64(ow), float64(height)/float64(oh))
	return max(1, int(math.Round(float64(ow)*scale))), max(1, int(math.Round(float64(oh)*scale)))
}
