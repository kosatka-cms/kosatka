package interfaces

import (
	"context"
	"encoding/xml"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

const rssFeedItemLimit = 20

// RssController serves the RSS 2.0 feed of the front page feed View
// (SiteInformation.FrontFeedView).
type RssController struct {
	siteInformationRepo entity.SiteInformationRepository
	feed                frontFeedSource
	logger              *slog.Logger
}

// frontFeedSource - the front page feed View's items (implemented by
// DynamicContentController).
type frontFeedSource interface {
	FrontFeedItems(ctx context.Context, machineName string, limit int) ([]entity.TeaserItem, entity.FeedImage, error)
}

// NewRssController creates an RssController on the site information and
// the front page feed's items (feed).
func NewRssController(
	siteInformationRepo entity.SiteInformationRepository,
	baseLogger *slog.Logger,
	feed frontFeedSource,
) *RssController {
	return &RssController{
		siteInformationRepo: siteInformationRepo,
		feed:                feed,
		logger:              baseLogger.With("component", "rss"),
	}
}

// Feed - GET /rss.xml: the latest items of the front page feed View as
// RSS 2.0, the channel named after the site.
func (c *RssController) Feed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dbCtx, cancel := context.WithTimeout(ctx, dbQueryTimeout)
	defer cancel()

	site, err := c.siteInformationRepo.Get(dbCtx)
	if err != nil || site == nil {
		logFailure(c.logger, err, "rss feed: site information")
		httputil.InternalServerError(w)
		return
	}
	var items []entity.TeaserItem
	var image entity.FeedImage
	if c.feed != nil {
		items, image, err = c.feed.FrontFeedItems(dbCtx, site.FrontFeedView, rssFeedItemLimit)
		if err != nil {
			logFailure(c.logger, err, "rss feed: front feed items")
			httputil.InternalServerError(w)
			return
		}
	}

	base := publicBaseURL(r)
	channelTitle := strings.TrimSpace(site.SiteName)
	channelDesc := strings.TrimSpace(site.Slogan)
	if channelDesc == "" {
		channelDesc = channelTitle
	}

	doc := buildRssDoc(channelTitle, base+"/", channelDesc, items, image, base)
	writeRssXML(w, doc)
}

func buildRssDoc(channelTitle, channelLink, channelDesc string, teasers []entity.TeaserItem, image entity.FeedImage, base string) RSSDoc {
	items := make([]RSSItem, 0, len(teasers))
	for _, t := range teasers {
		items = append(items, buildRssItem(t, image, base))
	}
	return RSSDoc{
		Version: "2.0",
		Channel: RSSChannel{
			Title:         channelTitle,
			Link:          channelLink,
			Description:   channelDesc,
			Language:      i18n.Default(),
			LastBuildDate: time.Now().UTC().Format(time.RFC1123Z),
			Items:         items,
		},
	}
}

// buildRssItem - a feed item: description is the plain text of the
// body field, enclosure the feed's image field in its style.
func buildRssItem(t entity.TeaserItem, image entity.FeedImage, base string) RSSItem {
	link := RSSPostURL(base, t.Node)
	item := RSSItem{
		Title:       t.Node.Title,
		Link:        link,
		Description: service.GetPlainPreview(t.Body(), 600),
		PubDate:     t.Node.CreatedAt.UTC().Format(time.RFC1123Z),
		GUID: RSSGUID{
			IsPermaLink: "true",
			Value:       link,
		},
	}

	if file := t.OwnImage(image.Field); file.URI != "" {
		uri := imageurl.Original(file).URI
		if image.Style != "" {
			uri = imageurl.Styled(file, image.Style).URI
		}
		item.Enclosure = &RSSEnclosure{
			URL:    AbsoluteAssetURL(base, uri),
			Length: "0",
			Type:   ImageMIME(uri),
		}
	}

	return item
}

// RSSPostURL - n's absolute address under base (entity.Node.URL: a
// module type's path prefix included).
func RSSPostURL(base string, n entity.Node) string {
	return base + n.URL()
}

// AbsoluteAssetURL - uri absolute under base (an absolute http(s) URL as
// it is).
func AbsoluteAssetURL(base, uri string) string {
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return uri
	}
	if !strings.HasPrefix(uri, "/") {
		uri = "/" + uri
	}
	return base + uri
}

// ImageMIME - the media type of the image at uri by its extension (JPEG
// when none of WebP, PNG, GIF).
func ImageMIME(uri string) string {
	switch strings.ToLower(filepath.Ext(uri)) {
	case ".webp":
		return "image/webp"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

func writeRssXML(w http.ResponseWriter, doc RSSDoc) {
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		httputil.InternalServerError(w)
		return
	}
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}

// RSSDoc - the RSS 2.0 document (<rss>).
type RSSDoc struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel RSSChannel `xml:"channel"`
}

// RSSChannel - the feed's <channel>: the site and its items.
type RSSChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []RSSItem `xml:"item"`
}

// RSSItem - a feed's <item>: a content item.
type RSSItem struct {
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description string        `xml:"description"`
	PubDate     string        `xml:"pubDate"`
	GUID        RSSGUID       `xml:"guid"`
	Enclosure   *RSSEnclosure `xml:"enclosure,omitempty"`
}

// RSSGUID - an item's <guid>.
type RSSGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// RSSEnclosure - an item's <enclosure>: its image.
type RSSEnclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}
