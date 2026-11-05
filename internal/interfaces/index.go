package interfaces

import (
	"bytes"
	"context"
	"html/template"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// IndexController serves the home page and the pages of blocks loaded
// after the page: lazy block placements and View blocks' pages.
type IndexController struct {
	*BaseController
	// frontFeed - renders the front page feed View (IndexDeps).
	frontFeed frontFeedRenderer
}

// IndexDeps - what NewIndexController needs besides the BaseDeps.
type IndexDeps struct {
	// FrontFeed renders the front page feed View.
	FrontFeed frontFeedRenderer
}

// NewIndexController creates an IndexController on deps and d.
func NewIndexController(deps BaseDeps, d IndexDeps) *IndexController {
	base := NewBaseController(deps)
	base.logger = deps.Logger.With("component", "index.handler")

	return &IndexController{
		BaseController: base,
		frontFeed:      d.FrontFeed,
	}
}

// Index - GET /: the home page, with the front page feed View chosen in
// site information (SiteInformation.FrontFeedView) at its ?page=N.
func (c *IndexController) Index(w http.ResponseWriter, r *http.Request) {
	dbCtx, dbCancel := c.DBContext(r)
	defer dbCancel()

	page := c.GetPageNumber(r)

	var site *entity.SiteInformation
	if c.siteInformationRepo != nil {
		if si, err := c.siteInformationRepo.Get(dbCtx); err == nil {
			site = si
		} else if c.logger != nil {
			c.logger.Warn("failed to load site information for home", "error", err)
		}
	}

	// The feed: the View chosen in site information (SiteInformation.FrontFeedView),
	// rendered with its own fields, theme template and pager.
	var feedHTML template.HTML
	if site != nil && c.frontFeed != nil {
		html, ok, err := c.frontFeed.RenderFrontFeed(dbCtx, site.FrontFeedView, page, r.URL.Path, r.URL.Query())
		if err != nil {
			c.LogFailure(err, "error rendering the front page feed")
		} else if ok {
			feedHTML = html
		}
	}

	data := c.GetCommonData(
		r,
	)
	newData := map[string]any{
		"feedHTML": feedHTML,
	}

	maps.Copy(data, newData)

	// Canonical/og:url: the root or ?page=N (no other query parameters).
	data["pageSEO"] = PageSEOForHome(site, page)

	settings := entity.NewSystemSettings()
	data["rssFeedURL"] = strings.TrimSuffix(settings.Domain, "/") + "/rss.xml"
	if site != nil {
		data["rssFeedTitle"] = strings.TrimSpace(site.SiteName)
	}

	c.RenderOrFail(w, r, "home/index.html", data)
}

// BlockPlacementHTML - GET /api/block-placement/{id}: the content of a
// lazy block placement (entity.BlockPlacementDef.Lazy), which the page
// rendered as a placeholder (partials/region_blocks.html) and app.js
// loads when it scrolls near the viewport. Only lazy placements are
// served. The page decided the scope (its node and path); the roles
// condition is checked again, since this endpoint is public. ?path= is the
// page's own path for blocks that look at it.
func (c *IndexController) BlockPlacementHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || c.cacheService == nil {
		http.NotFound(w, r)
		return
	}
	sidebarData := c.cacheService.SidebarDataFor(r.Context())
	item, ok := service.LazyPlacement(sidebarData.BlockPlacements, id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	// The roles condition is checked here too: the endpoint is public.
	user, _ := c.GetCurrentUser(r)
	roles := entity.UserRoleNames(user)
	if !item.Placement.VisibleToRoles(roles) || (item.Placement.PluginID != nil && !service.PluginBlockEnabled(*item.Placement.PluginID)) {
		http.NotFound(w, r)
		return
	}
	currentPath := r.URL.Query().Get("path")
	ctx := map[string]any{
		"blockPlacements":     map[entity.Region][]entity.BlockWithPlacement{item.Placement.Region: {item}},
		"viewBlockContent":    c.cacheService.ViewBlockContentFor(r.Context(), sitetime.FromContext(r.Context())),
		"isHomepage":          currentPath == "/" || currentPath == "",
		"currentPath":         currentPath,
		"canEditNodeSettings": false,
		"currentRoles":        roles,
		"item":                nil,
	}
	data := withRequestContext(r.Context(), map[string]any{"Region": item.Placement.Region, "Ctx": ctx})
	var buf bytes.Buffer
	if err := c.templateService.RenderNamed(&buf, "region_blocks", data, service.RegionBlocksStandaloneFiles...); err != nil {
		c.LogFailure(err, "lazy block placement render", "id", id)
		httputil.InternalServerError(w)
		return
	}
	// What the block's text embeds, as the visitor sees it (a fragment,
	// not a page TemplateService.Render fills in).
	_, _ = w.Write(service.ResolveEmbeds(r.Context(), buf.Bytes()))
}

// ViewBlockPage - GET /api/view-block/{id}?page=N: a page of a paged View
// block (placement id) as JSON {"html": …}, the list and its pager that
// app.js swaps into the block (the block itself holds the first page,
// from the cache). The page decided where the block shows; the roles
// condition is checked again, since this endpoint is public.
func (c *IndexController) ViewBlockPage(w http.ResponseWriter, r *http.Request) {
	// Opened as a page (no script: a new tab, a crawler): back to the page
	// the block is on, not its JSON.
	if r.Header.Get("X-Requested-With") != "XMLHttpRequest" && !strings.Contains(r.Header.Get("Accept"), "application/json") {
		back := "/"
		if ref, err := url.Parse(r.Referer()); err == nil && ref.Host == r.Host {
			if p := service.SafeRedirectPath(ref.RequestURI()); p != "" {
				back = p
			}
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || c.cacheService == nil {
		http.NotFound(w, r)
		return
	}
	item, ok := service.ViewPlacement(c.cacheService.SidebarDataFor(r.Context()).BlockPlacements, id)
	user, _ := c.GetCurrentUser(r)
	if !ok || !item.Placement.VisibleToRoles(entity.UserRoleNames(user)) {
		http.NotFound(w, r)
		return
	}
	html, ok, err := c.cacheService.ViewBlockPage(r.Context(), *item.ViewDisplay, id, c.GetPageNumber(r), sitetime.FromContext(r.Context()))
	if err != nil {
		c.LogFailure(err, "error rendering a View block's page", "id", id)
		httputil.InternalServerError(w)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	// Its rows' texts may embed something: as the visitor sees it.
	html = template.HTML(service.ResolveEmbeds(r.Context(), []byte(html)))
	httputil.JSON(w, map[string]any{"html": html}, http.StatusOK)
}

// frontFeedRenderer - renders the front page feed View (implemented by
// DynamicContentController).
type frontFeedRenderer interface {
	RenderFrontFeed(ctx context.Context, machineName string, page int, basePath string, query url.Values) (template.HTML, bool, error)
}
