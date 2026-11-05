package imagestyle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// ImageStyle - a named image derivative preset (size, resize mode, anchor,
// quality, format).
type ImageStyle struct {
	name        string
	label       string
	width       int
	height      int
	mode        ResizeMode
	anchor      imaging.Anchor
	quality     int
	format      Format
	description string
	hiDPI       bool
	focal       bool
}

// Format - the encoding of a style's derivatives.
type Format string

const (
	// FormatWebP - the default.
	FormatWebP Format = "webp"
	// FormatJPEG - for consumers that don't accept WebP (e.g. some feed
	// readers accept only JPEG/GIF/PNG).
	FormatJPEG Format = "jpeg"
)

// ParseFormat - f if known, else FormatWebP.
func ParseFormat(f string) Format {
	if Format(f) == FormatJPEG {
		return FormatJPEG
	}
	return FormatWebP
}

// Extension - the derivative file extension (with the dot).
func (f Format) Extension() string {
	if f == FormatJPEG {
		return ".jpg"
	}
	return ThumbnailExtension
}

// ContentType - the derivative MIME type.
func (f Format) ContentType() string {
	if f == FormatJPEG {
		return "image/jpeg"
	}
	return ThumbnailContentType
}

// ResizeMode - how a style fits an image into its width and height (a
// height of 0 keeps the aspect ratio in every mode).
type ResizeMode string

// The resize modes.
const (
	// ResizeModeFill - the exact size, the rest cropped at the anchor or
	// the focal point.
	ResizeModeFill ResizeMode = "fill"
	// ResizeModeFit - within the size, the aspect ratio kept.
	ResizeModeFit ResizeMode = "fit"
	// ResizeModeResize - the exact size, the aspect ratio not kept.
	ResizeModeResize ResizeMode = "resize"
)

// NewImageStyle validates the parameters and creates a style; an invalid
// quality defaults to 85.
func NewImageStyle(name, label string, width, height int, mode ResizeMode, anchor imaging.Anchor, quality int) (*ImageStyle, error) {
	if name == "" {
		return nil, fmt.Errorf("style name cannot be empty")
	}

	// height = 0 keeps the aspect ratio.
	if width <= 0 {
		return nil, fmt.Errorf("width must be positive")
	}
	if height < 0 {
		return nil, fmt.Errorf("height cannot be negative")
	}

	if quality <= 0 || quality > 100 {
		quality = 85
	}

	return &ImageStyle{
		name:    name,
		label:   label,
		width:   width,
		height:  height,
		mode:    mode,
		anchor:  anchor,
		quality: quality,
		format:  FormatWebP,
	}, nil
}

// Name returns the style's machine name.
func (s *ImageStyle) Name() string { return s.name }

// Label returns the style's name shown in the admin.
func (s *ImageStyle) Label() string { return s.label }

// Width returns the derivative's width in pixels.
func (s *ImageStyle) Width() int { return s.width }

// Height returns the derivative's height in pixels; 0 - by the aspect
// ratio.
func (s *ImageStyle) Height() int { return s.height }

// Mode returns how the image is fitted into the size.
func (s *ImageStyle) Mode() ResizeMode { return s.mode }

// Anchor returns where ResizeModeFill crops without a focal point.
func (s *ImageStyle) Anchor() imaging.Anchor { return s.anchor }

// Quality returns the encoding's quality, 1-100.
func (s *ImageStyle) Quality() int { return s.quality }

// Description returns the admin's note on the style.
func (s *ImageStyle) Description() string { return s.description }

// Format - the derivative encoding (FormatWebP unless set).
func (s *ImageStyle) Format() Format {
	if s.format == "" {
		return FormatWebP
	}
	return s.format
}

// SetFormat sets the derivative encoding (ParseFormat: anything but JPEG
// is WebP).
func (s *ImageStyle) SetFormat(f Format) {
	s.format = ParseFormat(string(f))
}

// SetDescription sets the admin's note on the style.
func (s *ImageStyle) SetDescription(desc string) {
	s.description = desc
}

// HiDPI - whether its derivatives are made at double size too, for
// high-density screens (srcset 2x, DoublePath).
func (s *ImageStyle) HiDPI() bool { return s.hiDPI }

// SetHiDPI sets whether the derivatives are made at double size too.
func (s *ImageStyle) SetHiDPI(on bool) {
	s.hiDPI = on
}

// UsesFocal - whether it crops at the image's focal point (FocalPoint;
// at Anchor when the image has none).
func (s *ImageStyle) UsesFocal() bool { return s.focal }

// SetFocal sets whether the style crops at the focal point.
func (s *ImageStyle) SetFocal(on bool) {
	s.focal = on
}

// FocalPoint - a point of an image, as a fraction of its width (X) and
// height (Y) from the top left, 0-1.
type FocalPoint struct {
	X, Y float64
}

// Double - the style at double size (a HiDPI style's 2x derivatives);
// false for a style without them.
func (s *ImageStyle) Double() (*ImageStyle, bool) {
	if !s.hiDPI {
		return nil, false
	}
	v := *s
	v.width, v.height = 2*s.width, 2*s.height
	v.hiDPI = false
	return &v, true
}

// DoubleDir - the folder of a style's 2x derivatives under its own:
// styles/<style>/@2x/<path>. No file path starts with it (the paths of
// uploads are transliterated to letters, digits, - and _).
const DoubleDir = "@2x"

// DoublePath - relativePath of the 2x derivative under its style's
// folder.
func DoublePath(relativePath string) string {
	return DoubleDir + "/" + relativePath
}

// SplitDouble - whether relativePath is a DoublePath, and the path
// without it.
func SplitDouble(relativePath string) (bool, string) {
	rest, ok := strings.CutPrefix(relativePath, DoubleDir+"/")
	if !ok || rest == "" {
		return false, relativePath
	}
	return true, rest
}

// ErrStyleNotFound - no image style with the requested name.
var ErrStyleNotFound = errors.New("image style not found")

// ErrOriginalNotFound - the source image of a requested derivative isn't
// stored.
var ErrOriginalNotFound = errors.New("original image not found")

// ErrNotRenderable - the source isn't an image the processor decodes
// (another file, a broken one, one too large).
var ErrNotRenderable = errors.New("the source can't be rendered")

// ImageStyleRepository finds the image styles by name.
type ImageStyleRepository interface {
	FindByName(name string) (*ImageStyle, error)
}

// ImageStorageRepository finds originals and keeps the derivatives
// (StorageRepository: in the public files' store).
type ImageStorageRepository interface {
	OpenOriginal(ctx context.Context, relativePath string) (filestore.File, filestore.Info, error)
	// OpenExact opens the file at key itself, no derivative's extension
	// mapped to the original's.
	OpenExact(ctx context.Context, key string) (filestore.File, filestore.Info, error)
	OpenCached(ctx context.Context, styleName, relativePath string) (filestore.File, filestore.Info, error)
	SaveToCache(ctx context.Context, styleName, relativePath string, img ProcessedImage) error
	ClearStyleCache(ctx context.Context, styleName string) error
	ClearAllCache(ctx context.Context) error
	ClearResizeDir(ctx context.Context) error
	ClearAllGeneratedImages(ctx context.Context) error
}

// Thumbnail format for generated resize/ and styles/ derivatives.
const (
	ThumbnailContentType = "image/webp"
	ThumbnailExtension   = ".webp"
)

// ProcessedImage - an encoded derivative.
type ProcessedImage struct {
	Data        []byte
	ContentType string
	Extension   string
}

// NewProcessedImage wraps encoded WebP bytes.
func NewProcessedImage(data []byte) ProcessedImage {
	return newProcessedImage(data, FormatWebP)
}

func newProcessedImage(data []byte, f Format) ProcessedImage {
	return ProcessedImage{
		Data:        data,
		ContentType: f.ContentType(),
		Extension:   f.Extension(),
	}
}

// ImageProcessor renders a derivative of a source image (read from src;
// sourceName - its file name, for its format) for a style; a style
// cropping (fill) keeps focal (nil - none) in the picture, else crops at
// anchor.
type ImageProcessor interface {
	Process(src io.Reader, sourceName string, style *ImageStyle, anchor imaging.Anchor, focal *FocalPoint) (ProcessedImage, error)
}
