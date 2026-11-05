package interfaces

import (
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// searchMinChars - the shortest query searched; shorter ones get the
// categories (live search) or a hint (the results page).
const searchMinChars = 3

// SearchMaxChars - the longest query searched; a longer one is cut (a
// query is matched with LIKE, as one substring, against every searched
// field).
const SearchMaxChars = 100

// searchLimit - the most results of a search (the types' own limits,
// /admin/config/search, apply first).
const searchLimit = 50

// searchPageMax / searchPerPage - the results page: the most results of a
// query and a page of them.
const (
	searchPageMax = 200
	searchPerPage = 20
)

// SearchController handles the live search API and the search results
// page.
type SearchController struct {
	*BaseController
	searchService *service.SearchService
}

// NewSearchController creates a SearchController.
func NewSearchController(base *BaseController, searchService *service.SearchService) *SearchController {
	base.logger = base.logger.With("component", "search.controller")
	return &SearchController{BaseController: base, searchService: searchService}
}

// searchQuery - the q parameter, trimmed and cut to SearchMaxChars, and
// whether it is long enough to search.
func searchQuery(r *http.Request) (string, bool) {
	query := cutQuery(r.URL.Query().Get("q"))
	return query, len([]rune(query)) >= searchMinChars
}

// cutQuery - q trimmed, at most SearchMaxChars characters.
func cutQuery(q string) string {
	q = strings.TrimSpace(q)
	if r := []rune(q); len(r) > SearchMaxChars {
		q = strings.TrimSpace(string(r[:SearchMaxChars]))
	}
	return q
}

// LiveSearch handles GET /api/search?q=. With q shorter than 3 characters
// it returns categories, otherwise search results.
func (c *SearchController) LiveSearch(w http.ResponseWriter, r *http.Request) {
	query, ok := searchQuery(r)
	if !ok {
		categories := c.searchService.GetCategories(r.Context())
		httputil.SendJSON(w, http.StatusOK, map[string]any{
			"categories": categories,
		})
		return
	}

	ctx := r.Context()
	results, err := c.searchService.Search(ctx, query, searchLimit)
	if err != nil {
		c.LogFailure(err, "error searching",
			"query", query,
		)
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to search",
		})
		return
	}

	httputil.SendJSON(w, http.StatusOK, map[string]any{
		"results": results,
	})
}

// SearchPage handles GET /search?q=: the search results page - the same
// results as the live search, for the search form without JavaScript or
// on Enter. Never indexed (one page per query).
func (c *SearchController) SearchPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query, ok := searchQuery(r)
	title := i18n.T(ctx, "Search")
	var results []entity.SearchResult
	page := c.GetPageNumber(r)
	data := c.GetCommonData(r)
	if ok {
		all, err := c.searchService.SearchAll(ctx, query, searchPageMax)
		if err != nil {
			c.LogFailure(err, "error searching", "query", query)
			httputil.InternalServerError(w)
			return
		}
		if from := (page - 1) * searchPerPage; from < len(all) {
			results = all[from:min(from+searchPerPage, len(all))]
		} else if page > 1 {
			c.RenderNotFound(w, r)
			return
		}
		// The page's results with their snippets; without them, should
		// they fail.
		if withSnippets, err := c.searchService.WithSnippets(ctx, query, results); err != nil {
			c.LogFailure(err, "error making search snippets", "query", query)
		} else {
			results = withSnippets
		}
		data["total"] = len(all)
		data["pagination"] = pagination.New(page, len(all), searchPerPage, r.URL.Path, r.URL.Query())
		title = i18n.T(ctx, "Search results: @query", "@query", query)
	}

	seo := PageSEOForList(title, r.URL.Path)
	seo.NoIndex = true
	data["title"] = title
	data["pageSEO"] = seo
	data["query"] = query
	data["queryTooShort"] = query != "" && !ok
	data["searched"] = ok
	data["results"] = results
	data["searchMinChars"] = searchMinChars
	c.setPathTrail(data, r.URL.Path, i18n.T(ctx, "Search"))

	c.RenderOrFail(w, r, "search/results.html", data)
}
