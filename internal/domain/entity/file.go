package entity

import (
	"context"
	"time"
)

// PublicScheme - the prefix of a public file's URI ("public://<path>");
// the URL path it is served at is the config's (templates:
// publicFileURL, imageurl.PublicURI). PrivateScheme - a private file's
// ("private://<path>", in the private files' store): the site serves it
// only to those who may see a content item it is attached to.
const (
	PublicScheme  = "public://"
	PrivateScheme = "private://"
)

// File - an uploaded file (a files row): its URI (PublicScheme,
// PrivateScheme), type, size, an image's dimensions, alt text and focal
// point.
type File struct {
	ID        int
	URI       string
	MimeType  string
	Width     int
	Height    int
	SizeBytes int
	Alt       string
	// OriginalFilename - the client-supplied file name. Set only for File
	// field uploads and used as the download-link text; other uploads leave
	// it empty (they store a generated name).
	OriginalFilename string
	// Focal - the image's focal point (nil - none): a style cropping at it
	// (ImageStyleAnchorFocal) keeps it in the picture.
	Focal     *FocalPoint
	CreatedBy *User
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FocalPoint - a point of an image, in percent of its width (X) and
// height (Y) from the top left, 0-100.
type FocalPoint struct {
	X, Y int
}

// Valid - whether both are 0-100.
func (p FocalPoint) Valid() bool {
	return p.X >= 0 && p.X <= 100 && p.Y >= 0 && p.Y <= 100
}

// FileRepository defines the interface for file operations
type FileRepository interface {
	Create(ctx context.Context, uri, mimeType string, width, height, sizeBytes int, createdBy int) (*File, error)
	// ByID returns a file by id; ErrEntityNotFound if missing.
	ByID(ctx context.Context, id int) (*File, error)
	// ByIDs batch-loads files keyed by id; missing ids are absent from the
	// result rather than an error.
	ByIDs(ctx context.Context, ids []int) (map[int]File, error)
	// UpdateAlt sets a file's alt text. Alt is stored per file, not per
	// usage - fine while every upload creates its own file row.
	UpdateAlt(ctx context.Context, id int, alt string) error
	// SetFocal sets a file's focal point (nil - none).
	SetFocal(ctx context.Context, id int, focal *FocalPoint) error
	// SetOriginalFilename sets OriginalFilename right after Create (File
	// field uploads only); kept separate so Create's signature stays shared.
	SetOriginalFilename(ctx context.Context, id int, name string) error
	// ByURI returns the file stored under uri; ErrEntityNotFound if none.
	ByURI(ctx context.Context, uri string) (*File, error)
}
