package feeds

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Feed - an XML feed of the site's materials for an aggregator (feeds
// module, /admin/config/feeds): which materials (content types, optional
// terms, count, age), the format served at Path and, per content type, which
// fields give the text, cover and category (FieldMaps).
type Feed struct {
	ID     int
	Label  string
	Path   string // URL path without the leading "/", e.g. "feeds/dzen.xml"
	Format FeedFormat
	Status int // 1 - served, 0 - off
	// ContentTypes - machine names of the types whose materials go in.
	ContentTypes []string
	// TermIDs - only materials tagged with any of these terms (none -
	// every material of ContentTypes).
	TermIDs []int
	// ItemLimit - at most this many items (1…FeedMaxItems).
	ItemLimit int
	// Days - items created within this many days (0 - the format's
	// own window, FeedFormat.DefaultDays).
	Days int
	// Title/Description/Language - the channel's texts (empty - the
	// site's name, slogan and language).
	Title       string
	Description string
	Language    string
	// ImageStyle - the image style for the cover and the images inside
	// the text ("" - the originals). Dzen takes only JPEG/GIF/PNG.
	ImageStyle string
	FieldMaps  FeedFieldMaps
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// FeedMaxItems - the most items a feed may emit (Dzen takes up to 500
// per upload).
const FeedMaxItems = 500

// FeedFormat - the XML a feed produces.
type FeedFormat string

const (
	// FeedFormatRSS - plain RSS 2.0 (title, link, summary, cover).
	FeedFormatRSS FeedFormat = "rss"
	// FeedFormatDzen - RSS 2.0 per Dzen's export spec: full text in
	// content:encoded, cover in enclosure, category.
	FeedFormatDzen FeedFormat = "dzen"
)

// FeedFormats - every feed format, for the admin form.
var FeedFormats = []FeedFormat{FeedFormatDzen, FeedFormatRSS}

// ParseFeedFormat - f if known, else FeedFormatRSS.
func ParseFeedFormat(f string) FeedFormat {
	for _, k := range FeedFormats {
		if string(k) == f {
			return k
		}
	}
	return FeedFormatRSS
}

// DefaultDays - how many days back a feed of this format goes when it
// sets none: Dzen takes items up to 7 days old; plain RSS has no window.
func (f FeedFormat) DefaultDays() int {
	if f == FeedFormatDzen {
		return 7
	}
	return 0
}

// Label - the format's name in the admin (untranslated: shown through
// {{t .Label}}).
func (f FeedFormat) Label() string {
	if f == FeedFormatDzen {
		return i18n.N("Dzen (full text)")
	}
	return "RSS 2.0"
}

// FeedFieldMap - for one content type of a feed: the fields giving an
// item's text, cover and category, by machine name ("" - BodyField: the
// type's body field; ImageField/CategoryField: none).
type FeedFieldMap struct {
	ContentType   string
	BodyField     string
	ImageField    string
	CategoryField string
}

// FeedFieldMaps - a feed's field maps, one per content type
// (Feed.FieldMaps).
type FeedFieldMaps []FeedFieldMap

// For - the map of contentType (zero value - none set).
func (ms FeedFieldMaps) For(contentType string) FeedFieldMap {
	for _, m := range ms {
		if m.ContentType == contentType {
			return m
		}
	}
	return FeedFieldMap{ContentType: contentType}
}

// HasContentType - whether the feed includes contentType.
func (f Feed) HasContentType(contentType string) bool {
	return slices.Contains(f.ContentTypes, contentType)
}

// TermIDsText - TermIDs comma-separated (for storage and the form).
func (f Feed) TermIDsText() string {
	parts := make([]string, len(f.TermIDs))
	for i, id := range f.TermIDs {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}

// ParseFeedTermIDs - positive integers from a comma/space separated list.
func ParseFeedTermIDs(s string) []int {
	var out []int
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		if id, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
}

// FeedRepository - the feeds' storage: the list, one by ID or by path,
// creating, updating and deleting one with its field maps.
type FeedRepository interface {
	Feeds(ctx context.Context) ([]Feed, error)
	FeedByID(ctx context.Context, id int) (*Feed, error)
	FeedByPath(ctx context.Context, path string) (*Feed, error)
	CreateFeed(ctx context.Context, f Feed) (int, error)
	// UpdateFeed saves f and replaces its FieldMaps.
	UpdateFeed(ctx context.Context, f Feed) error
	DeleteFeed(ctx context.Context, id int) error
}
