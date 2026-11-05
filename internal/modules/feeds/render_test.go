package feeds

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

func TestBuildDzenFeed(t *testing.T) {
	testkit.UseImageStyleFormats(t, map[string]imagestyle.Format{"feed_jpeg": imagestyle.FormatJPEG})

	created := time.Date(2026, 10, 9, 12, 30, 0, 0, time.UTC)
	item := modulekit.TeaserItem{
		Node:      modulekit.Node{Title: "News headline", Slug: "news/headline", CreatedAt: created},
		BodyField: "body",
		Fields: map[string]any{"body": `<p>Teaser</p><!--break--><p>Text <a href="/articles/foo">link</a></p>` +
			`<p><img src="/uploads/news/images/pic.webp" alt="x"></p>` +
			`<p><img src="/uploads/styles/large/news/images/old.webp"></p>` +
			`<p><img src="https://cdn.example.org/a.png"></p>` +
			`<p><img src="../uploads/u1/news/rel.webp"> <a href="../articles/bar">x</a></p>`},
		Images: map[string]modulekit.File{"cover": {URI: "public://news/images/cover.png"}},
		Terms:  map[string]modulekit.TaxonomyTerms{"category": {{Name: "Movies"}, {Name: "TV series"}}, "tags": {{Name: "Tag"}}},
	}
	item.Node.Type = "news"
	ch := feedChannel{Title: "Example Site", Link: "https://example.com/", Description: "<b>Site</b>  slogan", Language: "ru"}
	maps := FeedFieldMaps{{ContentType: "news", ImageField: "cover", CategoryField: "category"}}
	doc := buildDzenFeed(ch, []modulekit.TeaserItem{item}, maps, "feed_jpeg", "https://example.com")

	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		`<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:media="http://search.yahoo.com/mrss/">`,
		`<description>Site slogan</description>`,
		`<language>ru</language>`,
		`<link>https://example.com/news/headline</link>`,
		`<guid>https://example.com/news/headline</guid>`,
		`<pubDate>Fri, 09 Oct 2026 12:30:00 +0000</pubDate>`,
		`<category>Movies</category>`,
		`<category>TV series</category>`,
		`<enclosure url="https://example.com/uploads/styles/feed_jpeg/news/images/cover.jpg" length="0" type="image/jpeg"></enclosure>`,
		`<content:encoded><![CDATA[`,
		`href="https://example.com/articles/foo"`,
		`src="https://example.com/uploads/styles/feed_jpeg/news/images/pic.jpg"`,
		`src="https://example.com/uploads/styles/feed_jpeg/news/images/old.jpg"`,
		`src="https://cdn.example.org/a.png"`,
		`src="https://example.com/uploads/styles/feed_jpeg/u1/news/rel.jpg"`,
		`href="https://example.com/articles/bar"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s\n%s", want, s)
		}
	}
	if strings.Contains(s, ".webp") {
		t.Error("no webp may remain in a feed with a JPEG style")
	}
	if strings.Contains(s, "<!--break-->") {
		t.Error("read-more marker must not reach the feed")
	}
	if strings.Contains(s, "<category>Tag</category>") {
		t.Error("only the mapped category field goes in")
	}
}

// TestBuildFeedExplicitMap - the field map decides the text, the cover
// and the category per content type; a type without a map gets the body
// field and no cover/category.
func TestBuildFeedExplicitMap(t *testing.T) {
	news := modulekit.TeaserItem{
		Node:      modulekit.Node{Type: "news", Title: "A", Slug: "news/a", CreatedAt: time.Now()},
		BodyField: "body",
		Fields:    map[string]any{"body": "<p>Main text</p>", "lead": "<p>Lead text</p>"},
		Images:    map[string]modulekit.File{"first": {URI: "public://a1.png"}, "second": {URI: "public://a2.png"}},
	}
	other := modulekit.TeaserItem{
		Node:      modulekit.Node{Type: "article", Title: "B", Slug: "articles/b", CreatedAt: time.Now()},
		BodyField: "text",
		Fields:    map[string]any{"text": "<p>Article text</p>"},
		Images:    map[string]modulekit.File{"pic": {URI: "public://b.png"}},
	}
	maps := FeedFieldMaps{{ContentType: "news", BodyField: "lead", ImageField: "second"}}
	out, err := xml.Marshal(buildDzenFeed(feedChannel{}, []modulekit.TeaserItem{news, other}, maps, "", "https://example.com"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"Lead text", "a2.png", "Article text"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, bad := range []string{"Main text", "a1.png", "b.png"} {
		if strings.Contains(s, bad) {
			t.Errorf("unexpected %q", bad)
		}
	}

	rss, err := xml.Marshal(buildFeedRSS(feedChannel{}, []modulekit.TeaserItem{news}, maps, "", "https://example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rss), "<description>Lead text</description>") || !strings.Contains(string(rss), "a2.png") {
		t.Errorf("rss: %s", rss)
	}
}

// TestFeedImageURLVersioned - a derivative's URL with its style's version
// goes back to the original's path and into the feed's style, with that
// style's version.
func TestFeedImageURLVersioned(t *testing.T) {
	testkit.UseImageStyleVersions(t, map[string]string{"article_card": "1a2b3c4d", "feed_jpeg": "99aa00ff"})
	src := modulekit.StyledImage(modulekit.File{URI: "public://images/a.png"}, "article_card").URI
	got := feedImageURL(src, "feed_jpeg", "https://example.com")
	if want := "https://example.com" + modulekit.PublicFilesURL() + "styles/feed_jpeg/images/a.webp?v=99aa00ff"; got != want {
		t.Errorf("feed image: %s, want %s", got, want)
	}
}
