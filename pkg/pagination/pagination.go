package pagination

import (
	"math"
	"net/url"
	"strconv"
)

// The defaults: the items on a page and the most page numbers shown.
const (
	DefaultItemsPerPage = 50
	DefaultMaxVisible   = 9
)

// Pagination holds all information for pagination
type Pagination struct {
	CurrentPage  int        // Current page (starts from 1)
	TotalPages   int        // Total pages
	TotalItems   int        // Total items
	ItemsPerPage int        // Items per page
	MaxVisible   int        // Maximum visible page numbers
	Offset       int        // Offset for SQL query
	BaseURL      string     // Base URL for links
	QueryParams  url.Values // Additional URL parameters
	Anchor       string     // Optional anchor (e.g., "comments")
}

// PageInfo contains information about a single page for display
type PageInfo struct {
	Number     int    // Page number
	IsCurrent  bool   // Current page
	IsEllipsis bool   // Ellipsis
	URL        string // Page URL
}

// New creates a new pagination structure
func New(currentPage, totalItems, itemsPerPage int, baseURL string, queryParams url.Values) *Pagination {
	return NewWithOptions(currentPage, totalItems, itemsPerPage, DefaultMaxVisible, baseURL, queryParams, "")
}

// NewWithAnchor creates pagination with anchor
func NewWithAnchor(currentPage, totalItems, itemsPerPage int, baseURL string, queryParams url.Values, anchor string) *Pagination {
	return NewWithOptions(currentPage, totalItems, itemsPerPage, DefaultMaxVisible, baseURL, queryParams, anchor)
}

// NewWithOptions creates a new pagination structure with all options
func NewWithOptions(currentPage, totalItems, itemsPerPage, maxVisible int, baseURL string, queryParams url.Values, anchor string) *Pagination {
	if currentPage < 1 {
		currentPage = 1
	}

	if itemsPerPage < 1 {
		itemsPerPage = DefaultItemsPerPage
	}

	if maxVisible < 1 {
		maxVisible = DefaultMaxVisible
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(itemsPerPage)))
	if totalPages < 1 {
		totalPages = 1
	}

	if currentPage > totalPages {
		currentPage = totalPages
	}

	offset := (currentPage - 1) * itemsPerPage

	return &Pagination{
		CurrentPage:  currentPage,
		TotalPages:   totalPages,
		TotalItems:   totalItems,
		ItemsPerPage: itemsPerPage,
		MaxVisible:   maxVisible,
		Offset:       offset,
		BaseURL:      baseURL,
		QueryParams:  queryParams,
		Anchor:       anchor,
	}
}

// GetPages returns an array of pages for display
func (p *Pagination) GetPages() []PageInfo {
	var pages []PageInfo

	if p.TotalPages <= 1 {
		return pages
	}

	startPage := 1
	endPage := p.TotalPages

	if p.TotalPages > p.MaxVisible {
		halfVisible := p.MaxVisible / 2

		if p.CurrentPage <= halfVisible+1 {
			endPage = p.MaxVisible
		} else if p.CurrentPage >= p.TotalPages-halfVisible {
			startPage = p.TotalPages - p.MaxVisible + 1
		} else {
			startPage = p.CurrentPage - halfVisible
			endPage = p.CurrentPage + halfVisible
		}
	}

	if startPage > 1 {
		pages = append(pages, PageInfo{
			Number:     0,
			IsCurrent:  false,
			IsEllipsis: true,
			URL:        "",
		})
	}

	for i := startPage; i <= endPage; i++ {
		pages = append(pages, PageInfo{
			Number:     i,
			IsCurrent:  i == p.CurrentPage,
			IsEllipsis: false,
			URL:        p.buildURL(i),
		})
	}

	if endPage < p.TotalPages {
		pages = append(pages, PageInfo{
			Number:     0,
			IsCurrent:  false,
			IsEllipsis: true,
			URL:        "",
		})
	}

	return pages
}

// HasPrevious checks if there is a previous page
func (p *Pagination) HasPrevious() bool {
	return p.CurrentPage > 1
}

// HasNext checks if there is a next page
func (p *Pagination) HasNext() bool {
	return p.CurrentPage < p.TotalPages
}

// PreviousURL returns the URL of the previous page
func (p *Pagination) PreviousURL() string {
	if !p.HasPrevious() {
		return ""
	}
	return p.buildURL(p.CurrentPage - 1)
}

// NextURL returns the URL of the next page
func (p *Pagination) NextURL() string {
	if !p.HasNext() {
		return ""
	}
	return p.buildURL(p.CurrentPage + 1)
}

// FirstURL returns the URL of the first page
func (p *Pagination) FirstURL() string {
	return p.buildURL(1)
}

// LastURL returns the URL of the last page
func (p *Pagination) LastURL() string {
	return p.buildURL(p.TotalPages)
}

// GetRange returns the range of items being displayed (e.g., "Showing 11-20 of 150")
func (p *Pagination) GetRange() (start, end int) {
	if p.TotalItems == 0 {
		return 0, 0
	}

	start = p.Offset + 1
	end = p.Offset + p.ItemsPerPage
	if end > p.TotalItems {
		end = p.TotalItems
	}
	return start, end
}

// buildURL builds the URL for the specified page (1-based indexing)
func (p *Pagination) buildURL(pageNum int) string {
	params := url.Values{}

	// Copy query parameters except 'page'
	if p.QueryParams != nil {
		for key, values := range p.QueryParams {
			if key != "page" {
				params[key] = values
			}
		}
	}

	// Add page parameter only if not the first page (1-based)
	if pageNum > 1 {
		params.Set("page", strconv.Itoa(pageNum))
	}

	urlStr := p.BaseURL
	if len(params) > 0 {
		urlStr += "?" + params.Encode()
	}

	if p.Anchor != "" {
		urlStr += "#" + p.Anchor
	}

	return urlStr
}
