package entity

import "context"

// MediaKind - which of the files the media library lists: all, the
// images, the others.
type MediaKind string

// The media library's kinds (MediaKind).
const (
	MediaAll       MediaKind = ""
	MediaImages    MediaKind = "images"
	MediaDocuments MediaKind = "documents"
)

// MediaFilter - the media library's search: text in the file's path,
// name or alt text; its kind; who uploaded it (0 - anyone); only the
// files nothing uses; the page. A field's picker narrows it further: the
// store (Scheme, public:// or private://; "" - any), the extensions
// (none - any), the least width and height, the largest size (0 - any),
// the files themselves (IDs; none - any).
type MediaFilter struct {
	Search     string
	Kind       MediaKind
	CreatedBy  int
	Unused     bool
	Scheme     string
	Extensions []string
	MinWidth   int
	MinHeight  int
	MaxBytes   int
	IDs        []int
	Offset     int
	Limit      int
}

// MediaUse - one place a file is used: a content item's field
// (NodeID, its Title, Type and Slug, InTrash; Field - the field's label), a
// content type's default image (Field, TypeLabel), or a module's table
// (Module - its name; NodeID when the module attaches it to a node).
type MediaUse struct {
	NodeID    int
	Title     string
	Type      string
	Slug      string
	InTrash   bool
	Field     string
	TypeLabel string
	Module    string
}

// URL - the content item's address (Node.URL).
func (u MediaUse) URL() string {
	return Node{Type: ContentType(u.Type), Slug: u.Slug}.URL()
}

// MediaTotals - how many files there are and their size.
type MediaTotals struct {
	Count int
	Bytes int
}

// MediaRepository - the media library's queries over the files.
type MediaRepository interface {
	// MediaFiles - the files the filter finds, the newest first, and
	// how many it finds in all.
	MediaFiles(ctx context.Context, f MediaFilter) ([]File, int, error)
	// MediaUseCounts - in how many places each of the files is used.
	MediaUseCounts(ctx context.Context, fileIDs []int) (map[int]int, error)
	// MediaUses - the places the file is used.
	MediaUses(ctx context.Context, fileID int) ([]MediaUse, error)
	// MediaUploaders - the users who uploaded files, by name.
	MediaUploaders(ctx context.Context) ([]User, error)
	MediaTotals(ctx context.Context) (MediaTotals, error)
}

// MediaThumbnailStyle - the image style of the media library's tiles
// (the engine's own, like the avatars'); gone - the originals.
const MediaThumbnailStyle = "media_library"

// LightboxStyle - the image style the lightbox opens an image in (the
// engine's own: a large WebP copy instead of the original, which may be
// a camera's megabytes); gone - the originals.
const LightboxStyle = "lightbox"
