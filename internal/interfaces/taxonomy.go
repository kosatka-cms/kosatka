package interfaces

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TaxonomyController serves the term pages and the content forms' tag
// autocomplete.
type TaxonomyController struct {
	*BaseController
	taxonomyService service.TaxonomyService
	viewService     views.Service
	teasers         *service.TeaserBuilder
	// seo - term page SEO templates (TaxonomyDeps).
	seo *service.SEOService
}

// TaxonomyDeps - what NewTaxonomyController needs besides the BaseDeps.
type TaxonomyDeps struct {
	TaxonomyService service.TaxonomyService
	ViewService     views.Service
	Teasers         *service.TeaserBuilder
	SEO             *service.SEOService
}

// NewTaxonomyController creates a TaxonomyController on deps and d.
func NewTaxonomyController(deps BaseDeps, d TaxonomyDeps) *TaxonomyController {
	base := NewBaseController(deps)
	base.logger = deps.Logger.With("component", "taxonomy.controller")
	return &TaxonomyController{
		BaseController:  base,
		taxonomyService: d.TaxonomyService,
		viewService:     d.ViewService,
		teasers:         d.Teasers,
		seo:             d.SEO,
	}
}

// termPage renders the page of term, found by TermPageFallback.
func (c *TaxonomyController) termPage(w http.ResponseWriter, r *http.Request, term *entity.TaxonomyTerm) {
	ctx := r.Context()
	slug := term.Slug

	data, err := c.termPageData(ctx, r, term)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error getting term page data", "slug", slug)
		return
	}

	c.RenderOrFail(w, r, "taxonomy/term.html", data)
}

// TermPageFallback serves a term page from the site-wide catch-all
// route (after nodes and View pages found nothing): the URL path is the
// term's slug, and only vocabularies with public term pages are served
// (TaxonomyVocabulary.PublicTermPages). Returns false - nothing written -
// when there is no such public term, so the caller renders its 404.
func (c *TaxonomyController) TermPageFallback(w http.ResponseWriter, r *http.Request) bool {
	slug := strings.TrimPrefix(r.URL.Path, "/")
	if slug == "" || c.shouldSkipPath(slug) {
		return false
	}
	term, err := c.taxonomyService.TermBySlug(r.Context(), slug)
	if err != nil || term == nil || !term.Vocabulary.PublicTermPages {
		c.LogFailure(err, "loading the term", "slug", slug)
		return false
	}
	c.termPage(w, r, term)
	return true
}

// shouldSkipPath reports whether the path must not be treated as a taxonomy term.
func (c *TaxonomyController) shouldSkipPath(path string) bool {
	// Skip exact matches for common files
	exactMatches := []string{
		"favicon.ico",
		"robots.txt",
		"sitemap.xml",
	}

	for _, exact := range exactMatches {
		if path == exact {
			return true
		}
	}

	// Skip static file paths: the assets, the public files (their
	// configured address), the well-known files, the API.
	staticPrefixes := []string{
		"static/",
		".well-known/",
		"api/",
	}
	if files := paths.PublicFilesRel(); files != "" && !strings.Contains(files, "://") {
		staticPrefixes = append(staticPrefixes, files)
	}

	for _, prefix := range staticPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// Skip paths with file extensions (likely static files)
	if strings.Contains(path, ".") {
		parts := strings.Split(path, "/")
		lastPart := parts[len(parts)-1]
		if strings.Contains(lastPart, ".") && !strings.HasSuffix(lastPart, ".") {
			// Has extension and it's not ending with a dot (which could be part of slug)
			ext := lastPart[strings.LastIndex(lastPart, ".")+1:]
			// Common file extensions
			fileExtensions := []string{"css", "js", "json", "png", "jpg", "jpeg", "gif", "svg", "ico", "woff", "woff2", "ttf", "eot", "map", "xml", "txt", "html", "htm"}
			for _, fileExt := range fileExtensions {
				if ext == fileExt {
					return true
				}
			}
		}
	}

	return false
}

func (c *TaxonomyController) termPageData(ctx context.Context, r *http.Request, term *entity.TaxonomyTerm) (map[string]any, error) {
	// The term in the page's language: its heading, description and SEO.
	localized := entity.LocalizeTerm(i18n.FromContext(ctx), *term)
	term = &localized
	page := c.GetPageNumber(r)
	items, total, pageSize, err := c.termItems(ctx, term, page)
	if err != nil {
		return nil, err
	}
	pager := pagination.New(page, total, pageSize, r.URL.Path, r.URL.Query())

	pagePath := "/" + strings.TrimPrefix(r.URL.Path, "/")
	// The term's page title (else its name); title/description come from
	// the term page SEO templates (/admin/config/seo).
	displayTitle := strings.TrimSpace(term.PageTitle)
	if displayTitle == "" {
		displayTitle = term.Name
	}
	pageSEO := PageSEOForList(displayTitle, pagePath)
	if c.seo != nil {
		pageSEO = c.seo.ApplyTerm(ctx, *term, pageSEO)
	}

	ApplyPaginationToPageSEO(&pageSEO, r.URL.Path, page, r.URL.Query())

	data := c.GetCommonData(r)
	newData := map[string]any{
		"title":      displayTitle,
		"term":       term,
		"items":      items,
		"pagination": pager,
		"pageSEO":    pageSEO,
		"moduleTerm": ModuleTermData(ctx, *term),
	}

	maps.Copy(data, newData)
	c.setPathTrail(data, r.URL.Path, displayTitle)

	return data, nil
}

// termItems - the term's items as teaser cards via the term display View
// for its vocabulary (lowest weight wins; as visitors see it -
// PublishedConfig), which supplies filters, sort and page size. Without
// such a View the list is empty.
func (c *TaxonomyController) termItems(ctx context.Context, term *entity.TaxonomyTerm, page int) ([]entity.TeaserItem, int, int, error) {
	displays, err := c.viewService.TermDisplays(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	for _, d := range displays {
		if !d.MatchesVocabulary(term.Vocabulary.ID) {
			continue
		}
		cfg, err := c.viewService.PublishedConfig(ctx, d)
		if err != nil {
			return nil, 0, 0, err
		}
		nodes, total, err := c.viewService.RunConfig(ctx, cfg, views.RunContext{Page: page, TermID: term.ID})
		if err != nil {
			return nil, 0, 0, err
		}
		items, err := c.teasers.Build(ctx, nodes)
		if err != nil {
			return nil, 0, 0, err
		}
		return items, total, cfg.View.PageSize, nil
	}
	return nil, 0, 0, nil
}

// Autocomplete handles AJAX autocomplete requests for taxonomy terms.
func (c *TaxonomyController) Autocomplete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vocabularyIDStr := r.URL.Query().Get("vid")
	query := r.URL.Query().Get("q")

	if vocabularyIDStr == "" || query == "" {
		httputil.JSONError(w, i18n.T(r.Context(), "Choose a vocabulary and type a search."), http.StatusBadRequest)
		return
	}

	vocabularyID := 0
	if _, err := fmt.Sscanf(vocabularyIDStr, "%d", &vocabularyID); err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The vocabulary was not found."), http.StatusBadRequest)
		return
	}

	terms, err := c.taxonomyService.SearchTermsByName(ctx, vocabularyID, query, 10)
	if err != nil {
		c.LogFailure(err, "error searching terms",
			"vocabularyID", vocabularyID,
			"query", query,
		)
		httputil.JSONError(w, i18n.T(r.Context(), "The terms could not be searched. Try again."), http.StatusInternalServerError)
		return
	}

	type Suggestion struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	suggestions := make([]Suggestion, len(terms))
	for i, term := range terms {
		suggestions[i] = Suggestion{
			ID:   term.ID,
			Name: term.Name,
		}
	}

	response := map[string]any{
		"suggestions": suggestions,
	}

	httputil.JSON(w, response, http.StatusOK)
}
