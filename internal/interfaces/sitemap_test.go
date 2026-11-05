package interfaces

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
)

type sitemapMenus struct{ service.MenuService }

func (sitemapMenus) Menus(context.Context) (entity.MenuDefs, error) {
	return entity.MenuDefs{{MachineName: "primary", InSitemap: true}, {MachineName: "footer"}}, nil
}

func (sitemapMenus) ResolvedTree(_ context.Context, machineName string) ([]service.MenuTreeItem, error) {
	return []service.MenuTreeItem{{URL: "/from-" + machineName, Children: []service.MenuTreeItem{{URL: "/broken-" + machineName, Broken: true}}}}, nil
}

// TestSitemapStaticMenus - the static sitemap lists the links of the menus
// marked "Include in the sitemap", whatever their machine names; broken
// links are skipped.
func TestSitemapStaticMenus(t *testing.T) {
	withSiteDomain(t, "https://example.com")
	c := NewSitemapController(nil, nil, nil, sitemapMenus{}, testLogger())
	rec := httptest.NewRecorder()
	c.Static(rec, httptest.NewRequest(http.MethodGet, "/sitemap/static.xml", nil))
	out := rec.Body.String()
	if !strings.Contains(out, "https://example.com/from-primary") {
		t.Errorf("flagged menu missing: %s", out)
	}
	if strings.Contains(out, "from-footer") || strings.Contains(out, "broken-") {
		t.Errorf("unflagged menu or broken link listed: %s", out)
	}
}

type sitemapViews struct{ views.Service }

func (sitemapViews) PublicPageViews(context.Context) (entity.ViewDisplayDefs, error) {
	return entity.ViewDisplayDefs{
		{URLPath: "articles", Status: 1},
		{URLPath: "users/%/posts", Status: 1},
		{URLPath: "drafts", Status: 0},
	}, nil
}

// TestSitemapStaticViews - the static sitemap lists the enabled Views
// pages; a page with a "%" argument (one address per value) isn't listed.
func TestSitemapStaticViews(t *testing.T) {
	withSiteDomain(t, "https://example.com")
	c := NewSitemapController(nil, nil, sitemapViews{}, nil, testLogger())
	rec := httptest.NewRecorder()
	c.Static(rec, httptest.NewRequest(http.MethodGet, "/sitemap/static.xml", nil))
	out := rec.Body.String()
	if !strings.Contains(out, "https://example.com/articles<") {
		t.Errorf("the Views page missing: %s", out)
	}
	if strings.Contains(out, "%") || strings.Contains(out, "drafts") {
		t.Errorf("a page with an argument or a disabled one listed: %s", out)
	}
}

// TestSitemapRobots - robots.txt carries the sitemap at the site URL and
// the site's own rules.
func TestSitemapRobots(t *testing.T) {
	withSiteDomain(t, "https://example.com")
	site := &stubSiteInformation{site: entity.SiteInformation{RobotsExtra: "User-agent: SomeBot\nDisallow: /"}}
	c := NewSitemapController(site, nil, nil, nil, testLogger())
	rec := httptest.NewRecorder()
	c.Robots(rec, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	out := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("status %d, type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{"Disallow: /admin/", "Sitemap: https://example.com/sitemap.xml", "User-agent: SomeBot"} {
		if !strings.Contains(out, want) {
			t.Errorf("robots.txt lacks %q:\n%s", want, out)
		}
	}
}
