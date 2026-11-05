package feeds

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// FeedFallback serves a feed of the feeds module (/admin/config/feeds)
// at its path: its materials as XML in its format. Returns false when
// the path isn't an enabled feed, so the next fallback runs.
func (c *FeedController) FeedFallback(w http.ResponseWriter, r *http.Request) bool {
	if c.feeds == nil {
		return false
	}
	path := strings.Trim(r.URL.Path, "/")
	if path == "" {
		return false
	}
	feed := c.feeds.FeedByPath(r.Context(), path)
	if feed == nil {
		return false
	}
	// Public by nature, kept by any cache: as for a guest, whoever asks.
	ctx, err := modulekit.AsGuest(r.Context())
	if err != nil {
		c.LogFailure(err, "feed: the content types closed to guests", "feedID", feed.ID)
		httputil.InternalServerError(w)
		return true
	}

	nodes, err := c.feeds.Nodes(ctx, *feed, time.Now())
	if err != nil {
		c.LogFailure(err, "feed: nodes", "feedID", feed.ID)
		httputil.InternalServerError(w)
		return true
	}
	var items []modulekit.TeaserItem
	if c.teasers != nil {
		if items, err = c.teasers.Build(ctx, nodes); err != nil {
			c.LogFailure(err, "feed: teasers", "feedID", feed.ID)
			httputil.InternalServerError(w)
			return true
		}
	}

	var site modulekit.SiteInformation
	if c.siteInformationRepo != nil {
		if s, err := c.siteInformationRepo.Get(ctx); err == nil && s != nil {
			site = *s
		}
	}
	base := modulekit.SiteDomain()
	if base == "" {
		base = modulekit.PublicBaseURL(r)
	}
	ch := feedChannel{
		Title:       firstNonEmpty(feed.Title, site.SiteName),
		Link:        base + "/",
		Description: firstNonEmpty(feed.Description, site.Slogan, site.SiteName),
		Language:    firstNonEmpty(feed.Language, i18n.Default()),
	}

	var doc any
	if feed.Format == FeedFormatDzen {
		doc = buildDzenFeed(ch, items, feed.FieldMaps, feed.ImageStyle, base)
	} else {
		doc = buildFeedRSS(ch, items, feed.FieldMaps, feed.ImageStyle, base)
	}
	writeFeedXML(w, doc)
	return true
}

// feedChannel - a feed's channel texts.
type feedChannel struct {
	Title, Link, Description, Language string
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// plainOneLine - s without HTML, whitespace collapsed (Dzen: the channel
// description is one sentence without markup).
func plainOneLine(s string) string {
	return strings.Join(strings.Fields(htmlsanitize.StripToText(s)), " ")
}

func writeFeedXML(w http.ResponseWriter, doc any) {
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		httputil.InternalServerError(w)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}

// --- Dzen (dzen.ru/help/ru/export-content/export.html) ---

type dzenDoc struct {
	XMLName      xml.Name    `xml:"rss"`
	Version      string      `xml:"version,attr"`
	XmlnsContent string      `xml:"xmlns:content,attr"`
	XmlnsMedia   string      `xml:"xmlns:media,attr"`
	Channel      dzenChannel `xml:"channel"`
}

type dzenChannel struct {
	Title       string     `xml:"title"`
	Link        string     `xml:"link"`
	Description string     `xml:"description"`
	Language    string     `xml:"language"`
	Items       []dzenItem `xml:"item"`
}

type dzenItem struct {
	Title      string                  `xml:"title"`
	Link       string                  `xml:"link"`
	GUID       string                  `xml:"guid"`
	PubDate    string                  `xml:"pubDate"`
	Categories []string                `xml:"category,omitempty"`
	Enclosure  *modulekit.RSSEnclosure `xml:"enclosure,omitempty"`
	Content    dzenCDATA               `xml:"content:encoded"`
}

type dzenCDATA struct {
	Value string `xml:",cdata"`
}

// buildDzenFeed - the Dzen export RSS: per item, by its content type's
// field map, the full text (absolute links, local images in imageStyle,
// read-more marker dropped) in content:encoded, the cover in enclosure,
// the category field's terms as category.
func buildDzenFeed(ch feedChannel, teasers []modulekit.TeaserItem, maps FeedFieldMaps, imageStyle, base string) dzenDoc {
	items := make([]dzenItem, 0, len(teasers))
	for _, t := range teasers {
		m := maps.For(string(t.Node.Type))
		link := modulekit.RSSPostURL(base, t.Node)
		item := dzenItem{
			Title:     t.Node.Title,
			Link:      link,
			GUID:      link,
			PubDate:   t.Node.CreatedAt.UTC().Format(time.RFC1123Z),
			Enclosure: feedEnclosure(t, m, imageStyle, base),
			Content:   dzenCDATA{Value: feedBodyHTML(feedBody(t, m), imageStyle, base, link)},
		}
		if m.CategoryField != "" {
			for _, term := range t.Terms[m.CategoryField] {
				item.Categories = append(item.Categories, term.Name)
			}
		}
		items = append(items, item)
	}
	return dzenDoc{
		Version:      "2.0",
		XmlnsContent: "http://purl.org/rss/1.0/modules/content/",
		XmlnsMedia:   "http://search.yahoo.com/mrss/",
		Channel: dzenChannel{
			Title:       ch.Title,
			Link:        ch.Link,
			Description: plainOneLine(ch.Description),
			Language:    ch.Language,
			Items:       items,
		},
	}
}

// buildFeedRSS - plain RSS 2.0: per item a summary of its text and its
// cover, by its content type's field map.
func buildFeedRSS(ch feedChannel, teasers []modulekit.TeaserItem, maps FeedFieldMaps, imageStyle, base string) modulekit.RSSDoc {
	items := make([]modulekit.RSSItem, 0, len(teasers))
	for _, t := range teasers {
		m := maps.For(string(t.Node.Type))
		link := modulekit.RSSPostURL(base, t.Node)
		items = append(items, modulekit.RSSItem{
			Title:       t.Node.Title,
			Link:        link,
			Description: modulekit.PlainPreview(feedBody(t, m), 600),
			PubDate:     t.Node.CreatedAt.UTC().Format(time.RFC1123Z),
			GUID:        modulekit.RSSGUID{IsPermaLink: "true", Value: link},
			Enclosure:   feedEnclosure(t, m, imageStyle, base),
		})
	}
	return modulekit.RSSDoc{
		Version: "2.0",
		Channel: modulekit.RSSChannel{
			Title:         ch.Title,
			Link:          ch.Link,
			Description:   plainOneLine(ch.Description),
			Language:      ch.Language,
			LastBuildDate: time.Now().UTC().Format(time.RFC1123Z),
			Items:         items,
		},
	}
}

// feedBody - an item's text: the map's body field, else the type's body
// field.
func feedBody(t modulekit.TeaserItem, m FeedFieldMap) string {
	if m.BodyField != "" {
		return t.Text(m.BodyField)
	}
	return t.Body()
}

// feedEnclosure - an item's cover from the map's image field, in
// imageStyle ("" - the original); nil without one.
func feedEnclosure(t modulekit.TeaserItem, m FeedFieldMap, imageStyle, base string) *modulekit.RSSEnclosure {
	if m.ImageField == "" {
		return nil
	}
	file := t.OwnImage(m.ImageField)
	if file.URI == "" {
		return nil
	}
	uri := modulekit.OriginalImage(file).URI
	if imageStyle != "" {
		uri = modulekit.StyledImage(file, imageStyle).URI
	}
	return &modulekit.RSSEnclosure{URL: modulekit.AbsoluteAssetURL(base, uri), Length: "0", Type: modulekit.ImageMIME(uri)}
}

// feedBodyHTML - body HTML for a feed: src/href resolved against the item's
// page URL (editors may save relative ones), local images re-pointed at
// imageStyle ("" keeps them as they are), the "read more" marker dropped.
func feedBodyHTML(body, imageStyle, base, pageURL string) string {
	body = strings.ReplaceAll(body, htmlsanitize.BreakMarker, "")
	if strings.TrimSpace(body) == "" {
		return ""
	}
	nodes, err := html.ParseFragment(strings.NewReader(body), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return body
	}
	page, _ := url.Parse(pageURL)
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for i, a := range n.Attr {
				if a.Key != "href" && a.Key != "src" {
					continue
				}
				abs := resolveFeedURL(page, a.Val)
				if n.Data == "img" && a.Key == "src" {
					abs = feedImageURL(abs, imageStyle, base)
				}
				n.Attr[i].Val = abs
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	var buf bytes.Buffer
	for _, n := range nodes {
		walk(n)
		_ = html.Render(&buf, n)
	}
	return buf.String()
}

// resolveFeedURL - ref resolved against the page URL; anything that
// doesn't parse (or a nil page) comes back as is.
func resolveFeedURL(page *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if page == nil || ref == "" || strings.HasPrefix(ref, "#") {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return page.ResolveReference(u).String()
}

// feedImageURL - an image src for a feed: a local image (original or an
// existing derivative) in imageStyle, absolute; anything else as is.
func feedImageURL(src, imageStyle, base string) string {
	path := src
	if strings.HasPrefix(path, base+"/") {
		path = strings.TrimPrefix(path, base)
	}
	// Local images (originals and style derivatives) live under the
	// public files URL.
	localFilesPrefix := modulekit.PublicFilesURL()
	if !strings.HasPrefix(path, localFilesPrefix) {
		if strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") {
			return base + path
		}
		return src
	}
	if imageStyle == "" {
		return base + path
	}
	rel := strings.TrimPrefix(path, localFilesPrefix)
	// A derivative's version (?v=…) isn't part of the file's path.
	rel, _, _ = strings.Cut(rel, "?")
	if strings.HasPrefix(rel, "styles/") {
		// styles/<style>/<rel> - back to the original's path.
		if parts := strings.SplitN(rel, "/", 3); len(parts) == 3 {
			rel = parts[2]
		}
	}
	return base + modulekit.StyledImage(modulekit.File{URI: "public://" + rel}, imageStyle).URI
}
