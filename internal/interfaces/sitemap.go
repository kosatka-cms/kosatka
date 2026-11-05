package interfaces

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

const (
	sitemapNS       = "http://www.sitemaps.org/schemas/sitemap/0.9"
	sitemapChunkMax = 45000
)

// SitemapController serves sitemap index and urlset fragments for search engines.
type SitemapController struct {
	site     entity.SiteInformationRepository
	nodeRepo entity.NodesRepository
	views    views.Service
	menus    service.MenuService
	logger   *slog.Logger
}

// NewSitemapController creates a SitemapController on the site
// information, the nodes, the Views and the menus.
func NewSitemapController(
	site entity.SiteInformationRepository,
	nodeRepo entity.NodesRepository,
	views views.Service,
	menus service.MenuService,
	baseLogger *slog.Logger,
) *SitemapController {
	return &SitemapController{
		site:     site,
		nodeRepo: nodeRepo,
		views:    views,
		menus:    menus,
		logger:   baseLogger.With("component", "sitemap"),
	}
}

// Robots serves GET/HEAD /robots.txt (service.RobotsTxt): the engine's and
// the modules' rules, the sitemap, the site's own rules from the admin.
func (c *SitemapController) Robots(w http.ResponseWriter, r *http.Request) {
	var extra string
	if site, err := c.site.Get(r.Context()); err != nil {
		logFailure(c.logger, err, "robots.txt: site information (served without the site's rules)")
	} else {
		extra = site.RobotsExtra
	}
	http.ServeContent(w, r, "robots.txt", time.Time{}, strings.NewReader(service.RobotsTxt(publicBaseURL(r), extra)))
}

// Index - GET /sitemap.xml: the sitemap index - the static pages' sitemap
// and one sitemap per sitemapChunkMax public content items.
func (c *SitemapController) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	base := publicBaseURL(r)

	count, err := c.nodeRepo.CountPublicSitemapNodes(ctx, service.DisabledNodeTypes())
	if err != nil {
		logFailure(c.logger, err, "sitemap index: count nodes")
		httputil.InternalServerError(w)
		return
	}

	numChunks := (count + sitemapChunkMax - 1) / sitemapChunkMax
	doc := sitemapIndexDoc{
		Xmlns: sitemapNS,
		Items: []sitemapRef{
			{Loc: base + "/sitemap/static.xml"},
		},
	}
	for i := 1; i <= numChunks; i++ {
		doc.Items = append(doc.Items, sitemapRef{
			Loc: fmt.Sprintf("%s/sitemap/nodes/%d", base, i),
		})
	}

	writeSitemapXML(w, &doc)
}

// Static - GET /sitemap/static.xml: the site's pages other than its
// content: the Views' public pages (without a "%" argument), the modules'
// pages (AddSitemapPaths) and the links of the menus marked for the
// sitemap.
func (c *SitemapController) Static(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	base := publicBaseURL(r)

	seen := make(map[string]struct{})
	var urls []urlEntry
	add := func(rel string) {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			return
		}
		if !strings.HasPrefix(rel, "/") {
			rel = "/" + rel
		}
		loc := base + rel
		if _, ok := seen[loc]; ok {
			return
		}
		seen[loc] = struct{}{}
		urls = append(urls, urlEntry{Loc: loc})
	}

	// Every Views page display with a public URL. PublicPageViews (unlike the
	// admin list's Views) returns all page displays regardless of grouping.
	// A page with a "%" argument has no address of its own (one per value):
	// it isn't listed.
	if c.views != nil {
		views, err := c.views.PublicPageViews(ctx)
		if err != nil {
			logFailure(c.logger, err, "sitemap static: views")
			httputil.InternalServerError(w)
			return
		}
		for _, v := range views {
			if v.Status == 1 && !strings.Contains(v.URLPath, entity.ViewURLArgument) {
				add(v.URLPath)
			}
		}
	}

	// The modules' own pages (AddSitemapPaths), e.g. the forum sections.
	for _, fn := range sitemapPathHooks {
		paths, err := fn(ctx)
		if err != nil {
			logFailure(c.logger, err, "sitemap static: module pages")
			httputil.InternalServerError(w)
			return
		}
		for _, p := range paths {
			add(p)
		}
	}

	// Every resolved, non-broken item, at any depth, of the menus marked
	// "Include in the sitemap". ResolvedTree already omits disabled items;
	// add() dedups overlaps with other sources.
	if c.menus != nil {
		menus, err := c.menus.Menus(ctx)
		if err != nil {
			logFailure(c.logger, err, "sitemap static: menus")
		}
		for _, m := range menus {
			if !m.InSitemap {
				continue
			}
			tree, err := c.menus.ResolvedTree(ctx, m.MachineName)
			if err != nil {
				logFailure(c.logger, err, "sitemap static: menu", "menu", m.MachineName)
				continue
			}
			addMenuTreeURLs(tree, add)
		}
	}

	doc := urlsetDoc{Xmlns: sitemapNS, URLs: urls}
	writeSitemapXML(w, &doc)
}

// NodesChunk - GET /sitemap/nodes/{n}: the n-th chunk (from 1) of the
// public content's addresses.
func (c *SitemapController) NodesChunk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	base := publicBaseURL(r)

	chunkStr := r.PathValue("n")
	chunkIdx, err := strconv.Atoi(chunkStr)
	if err != nil || chunkIdx < 1 {
		http.NotFound(w, r)
		return
	}

	count, err := c.nodeRepo.CountPublicSitemapNodes(ctx, service.DisabledNodeTypes())
	if err != nil {
		logFailure(c.logger, err, "sitemap nodes: count")
		httputil.InternalServerError(w)
		return
	}
	numChunks := (count + sitemapChunkMax - 1) / sitemapChunkMax
	if chunkIdx > numChunks || numChunks == 0 {
		http.NotFound(w, r)
		return
	}

	offset := (chunkIdx - 1) * sitemapChunkMax
	rows, err := c.nodeRepo.ListPublicSitemapNodes(ctx, service.DisabledNodeTypes(), sitemapChunkMax, offset)
	if err != nil {
		logFailure(c.logger, err, "sitemap nodes: list", "chunk", chunkIdx)
		httputil.InternalServerError(w)
		return
	}

	urls := make([]urlEntry, 0, len(rows))
	for _, row := range rows {
		// The site's 404/403 pages answer with an error status.
		if service.ErrorPageStatus(row.ID) != 0 {
			continue
		}
		rel := entity.NodeRelPath(entity.ContentType(row.Type), row.Slug)
		if rel == "" {
			continue
		}
		loc := base + entity.LocalPath(row.Langcode, "/"+rel)
		entry := urlEntry{Loc: loc}
		if !row.UpdatedAt.IsZero() {
			entry.Lastmod = row.UpdatedAt.UTC().Format(time.RFC3339)
		}
		urls = append(urls, entry)
	}

	doc := urlsetDoc{Xmlns: sitemapNS, URLs: urls}
	writeSitemapXML(w, &doc)
}

func writeSitemapXML(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	out, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		httputil.InternalServerError(w)
		return
	}
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}

type urlsetDoc struct {
	XMLName xml.Name   `xml:"urlset"`
	Xmlns   string     `xml:"xmlns,attr"`
	URLs    []urlEntry `xml:"url"`
}

type urlEntry struct {
	Loc     string `xml:"loc"`
	Lastmod string `xml:"lastmod,omitempty"`
}

type sitemapIndexDoc struct {
	XMLName xml.Name     `xml:"sitemapindex"`
	Xmlns   string       `xml:"xmlns,attr"`
	Items   []sitemapRef `xml:"sitemap"`
}

type sitemapRef struct {
	Loc string `xml:"loc"`
}

// addMenuTreeURLs walks a resolved menu tree (service.MenuTreeItem)
// recursively, feeding every non-broken URL to add, at full depth.
func addMenuTreeURLs(items []service.MenuTreeItem, add func(string)) {
	for _, item := range items {
		if !item.Broken {
			add(item.URL)
		}
		addMenuTreeURLs(item.Children, add)
	}
}

func escapePathSegments(rel string) string {
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return ""
	}
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, url.PathEscape(p))
	}
	return strings.Join(out, "/")
}
