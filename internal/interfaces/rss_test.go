package interfaces

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// A feed item's link is the node's address, a module type's path prefix
// included (entity.NodeRelPath).
func TestRssPostURL(t *testing.T) {
	saved := entity.Modules
	t.Cleanup(func() { entity.Modules = saved })
	entity.Modules = append(slices.Clone(saved), entity.Module{Name: "boards", NodeTypes: []entity.ModuleNodeType{{Name: "board_topic", PathPrefix: "boards/"}}})
	for slug, want := range map[string]string{"news/test-slug": "https://www.example.com/news/test-slug"} {
		if got := RSSPostURL("https://www.example.com", entity.Node{Type: "news", Slug: slug}); got != want {
			t.Errorf("RSSPostURL(%q) = %q, want %q", slug, got, want)
		}
	}
	if got := RSSPostURL("https://www.example.com", entity.Node{Type: "board_topic", Slug: "who-wins"}); got != "https://www.example.com/boards/who-wins" {
		t.Errorf("a module's node type: %q", got)
	}
}

func TestBuildRssItem(t *testing.T) {
	created := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	item := buildRssItem(entity.TeaserItem{
		Node: entity.Node{
			Title:     "Test News",
			Slug:      "news/test-news",
			CreatedAt: created,
		},
		Fields:    map[string]any{"body": "<p>Hello <b>world</b></p>"},
		Images:    map[string]entity.File{"cover": {URI: "public://test.jpg"}},
		BodyField: "body",
	}, entity.FeedImage{Field: "cover", Style: "news_list"}, "https://www.example.com")

	if item.Title != "Test News" {
		t.Fatalf("title = %q", item.Title)
	}
	if item.Link != "https://www.example.com/news/test-news" {
		t.Fatalf("link = %q", item.Link)
	}
	if item.GUID.Value != item.Link {
		t.Fatalf("guid = %q", item.GUID.Value)
	}
	if !strings.Contains(item.Description, "Hello world") {
		t.Fatalf("description = %q", item.Description)
	}
	if item.Enclosure == nil || item.Enclosure.URL != "https://www.example.com/uploads/styles/news_list/test.webp" {
		t.Fatalf("enclosure = %+v", item.Enclosure)
	}
}

func TestBuildRssItem_NoImage(t *testing.T) {
	item := buildRssItem(entity.TeaserItem{Node: entity.Node{Title: "x", Slug: "x"}}, entity.FeedImage{Field: "cover", Style: "news_list"}, "https://www.example.com")
	if item.Enclosure != nil {
		t.Fatalf("enclosure = %+v, want none", item.Enclosure)
	}
}

func TestRssController_Feed(t *testing.T) {
	feed := &stubFrontFeed{items: []entity.TeaserItem{{
		Node: entity.Node{
			Title:     "First",
			Slug:      "news/first",
			CreatedAt: time.Now().UTC(),
		},
		Fields:    map[string]any{"body": "Body one"},
		BodyField: "body",
	}}}
	withSiteDomain(t, "https://www.example.com")
	site := &stubSiteInformation{site: entity.SiteInformation{SiteName: "Site", FrontFeedView: "front"}}
	c := NewRssController(site, testLogger(), feed)

	req := httptest.NewRequest(http.MethodGet, "/rss.xml", nil)
	rec := httptest.NewRecorder()
	c.Feed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/rss+xml") {
		t.Fatalf("Content-Type = %q", ct)
	}
	if feed.machineName != "front" || feed.limit != rssFeedItemLimit {
		t.Fatalf("feed asked for %q/%d", feed.machineName, feed.limit)
	}
	body := rec.Body.String()
	for _, want := range []string{"<rss", "<channel>", "<title>Site</title>", "<item>", "First", "Body one", "https://www.example.com/news/first"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

type stubFrontFeed struct {
	items       []entity.TeaserItem
	machineName string
	limit       int
}

func (s *stubFrontFeed) FrontFeedItems(ctx context.Context, machineName string, limit int) ([]entity.TeaserItem, entity.FeedImage, error) {
	s.machineName, s.limit = machineName, limit
	return s.items, entity.FeedImage{}, nil
}

type stubSiteInformation struct {
	site entity.SiteInformation
}

func (s *stubSiteInformation) Get(ctx context.Context) (*entity.SiteInformation, error) {
	site := s.site
	return &site, nil
}

func (s *stubSiteInformation) Update(ctx context.Context, site *entity.SiteInformation) error {
	return nil
}

func (s *stubSiteInformation) SetTheme(_ context.Context, theme string) error {
	s.site.Theme = &theme
	return nil
}

func (s *stubSiteInformation) SetRobotsExtra(_ context.Context, rules string) error {
	s.site.RobotsExtra = rules
	return nil
}

func (s *stubSiteInformation) SetLanguages(context.Context, []entity.SiteLanguage) error { return nil }

func (s *stubSiteInformation) SetSchemaOrg(context.Context, entity.SiteSchemaOrg) error { return nil }

// withSiteDomain sets the site URL for the test, restoring it after.
func withSiteDomain(t *testing.T, u string) {
	t.Helper()
	prev := entity.NewSystemSettings().Domain
	entity.SetSiteDomain(u)
	t.Cleanup(func() { entity.SetSiteDomain(prev) })
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
