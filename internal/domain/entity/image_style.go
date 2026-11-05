package entity

import (
	"context"
	"time"
)

// ImageStyleDef - an admin-editable image resize preset. The domain-layer
// counterpart of pkg/imagestyle.ImageStyle, used for admin CRUD; the resize
// pipeline (pkg/imagestyle.StyleService) resolves styles by Name through
// internal/infrastructure/image_style_adapter.go, which wraps
// ImageStyleRepository.
type ImageStyleDef struct {
	ID          int
	Name        string
	Label       string
	Description string
	Width       int
	Height      int // 0 = keep aspect ratio
	Mode        string
	Anchor      string
	Quality     int
	// Format - encoding of the derivatives: "webp" (default) or "jpeg"
	// (pkg/imagestyle.Format), for consumers that don't take WebP.
	Format string
	// HiDPI - its derivatives are made at double size too, for
	// high-density screens (srcset 2x, imagestyle.ImageStyle.HiDPI).
	HiDPI     bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ImageStyleDefs - a list of image styles.
type ImageStyleDefs []ImageStyleDef

// ImageStyleResizeModes - allowed Mode values (the imagestyle.ResizeMode*
// constants).
var ImageStyleResizeModes = []string{"fill", "fit", "resize"}

// ImageStyleFormats - allowed Format values.
var ImageStyleFormats = []string{"webp", "jpeg"}

// NormalizeImageStyleFormat - f if it's one of ImageStyleFormats, else
// "webp".
func NormalizeImageStyleFormat(f string) string {
	if f == "jpeg" {
		return "jpeg"
	}
	return "webp"
}

// ImageStyleAnchorFocal - the anchor cropping at the image's focal point
// (File.Focal), at its center when it has none.
const ImageStyleAnchorFocal = "focal"

// ImageStyleAnchors - ImageStyleAnchorFocal and the 9 imaging.Anchor
// values, in disintegration/imaging order.
var ImageStyleAnchors = []string{
	ImageStyleAnchorFocal, "center", "top-left", "top", "top-right",
	"left", "right", "bottom-left", "bottom", "bottom-right",
}

// ImageStyleRepository - CRUD for image styles.
type ImageStyleRepository interface {
	ImageStyles(ctx context.Context) (ImageStyleDefs, error)
	ImageStyleByID(ctx context.Context, id int) (*ImageStyleDef, error)
	ImageStyleByName(ctx context.Context, name string) (*ImageStyleDef, error)
	CreateImageStyle(ctx context.Context, s ImageStyleDef) (*ImageStyleDef, error)
	UpdateImageStyle(ctx context.Context, s ImageStyleDef) (*ImageStyleDef, error)
	DeleteImageStyle(ctx context.Context, id int) error
}
