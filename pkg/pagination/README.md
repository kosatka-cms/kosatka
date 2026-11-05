# Pagination

A lightweight and flexible Go pagination package for web applications.

## Installation

```bash
go get github.com/yourusername/pagination
```

## Quick Start

### Basic Usage

```go
package main

import (
    "net/http"
    "strconv"
    "github.com/kosatka-cms/kosatka/pkg/pagination"
)

func handler(w http.ResponseWriter, r *http.Request) {
    // Parse page from query string
    pageStr := r.URL.Query().Get("page")
    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        page = 1
    }
    
    totalCount := 150 // Total items from database
    itemsPerPage := 10
    
    pager := pagination.New(
        page,
        totalCount,
        itemsPerPage,
        r.URL.Path,
        r.URL.Query(),
    )
    
    // Use pager.Offset in your database query
    // items := db.Query("SELECT * FROM items LIMIT ? OFFSET ?", pager.ItemsPerPage, pager.Offset)
    
    // Get display range
    start, end := pager.GetRange()
    // Example: "Showing 11-20 of 150 items"
    
    // Pass to template
    // tmpl.Execute(w, map[string]interface{}{"pagination": pager})
}
```

### Custom Maximum Visible Pages

```go
// Show only 5 page numbers at a time instead of default 9
pager := pagination.NewWithOptions(
    page,
    totalCount,
    itemsPerPage,
    5, // maxVisible
    r.URL.Path,
    r.URL.Query(),
    "", // anchor
)
```

## Template Integration

### HTML Template Example

```html
{{define "pagination.html"}}
{{if gt .pagination.TotalPages 1}}
<nav aria-label="Pagination navigation">
    <ul class="pagination" role="list">
        <!-- First Page -->
        {{if .pagination.HasPrevious}}
        <li>
            <a href="{{.pagination.FirstURL}}" aria-label="Go to first page">
                <span aria-hidden="true">«</span> First
            </a>
        </li>
        {{end}}

        <!-- Previous Page -->
        {{if .pagination.HasPrevious}}
        <li>
            <a href="{{.pagination.PreviousURL}}" aria-label="Go to previous page">
                <span aria-hidden="true">‹</span> Previous
            </a>
        </li>
        {{end}}

        <!-- Page Numbers & Ellipsis -->
        {{range $page := .pagination.GetPages}}
        {{if $page.IsEllipsis}}
        <li class="ellipsis" aria-hidden="true">
            <span>…</span>
        </li>
        {{else if $page.IsCurrent}}
        <li>
            <span class="active" aria-current="page">{{$page.Number}}</span>
        </li>
        {{else}}
        <li>
            <a href="{{$page.URL}}" aria-label="Go to page {{$page.Number}}">
                {{$page.Number}}
            </a>
        </li>
        {{end}}
        {{end}}

        <!-- Next Page -->
        {{if .pagination.HasNext}}
        <li>
            <a href="{{.pagination.NextURL}}" aria-label="Go to next page">
                Next <span aria-hidden="true">›</span>
            </a>
        </li>
        {{end}}

        <!-- Last Page -->
        {{if .pagination.HasNext}}
        <li>
            <a href="{{.pagination.LastURL}}" aria-label="Go to last page">
                Last <span aria-hidden="true">»</span>
            </a>
        </li>
        {{end}}
    </ul>
</nav>

<!-- Display range info (place outside nav or add separate aria-label) -->
<div class="pagination-info" aria-live="polite">
    Showing {{(add .pagination.Offset 1)}}-{{min (add .pagination.Offset .pagination.ItemsPerPage) .pagination.TotalItems}} of {{.pagination.TotalItems}} items
</div>
{{end}}
{{end}}
```

### Using in Your Handler

```go
func listHandler(w http.ResponseWriter, r *http.Request) {
    // Parse page from query string
    pageStr := r.URL.Query().Get("page")
    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        page = 1
    }
    
    // Get total count from database
    totalCount := getTotalItemCount() // Your DB function
    
    // Create pagination
    pager := pagination.New(
        page,
        totalCount,
        20, // items per page
        r.URL.Path,
        r.URL.Query(),
    )
    
    // Fetch items with offset
    items := getItems(pager.ItemsPerPage, pager.Offset) // Your DB function
    
    // Render template
    data := map[string]interface{}{
        "items":      items,
        "pagination": pager,
    }
    
    tmpl.ExecuteTemplate(w, "list.html", data)
}
```

### CSS Styling Example

```css
/* Pagination */
.pagination {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 4px;
    padding: 15px 0;
    list-style: none;
}

.pagination li {
    display: inline-block;
}

.pagination li a,
.pagination li span {
    display: block;
    padding: 0.5em 0.75em;
    text-decoration: none;
    color: #333;
    background: #f0f0f0;
    border-radius: 4px;
    transition: background 0.2s;
}

.pagination li a:hover {
    background: #e0e0e0;
}

.pagination li.active span {
    font-weight: bold;
    background: #4a90e2;
    color: white;
}

.pagination li.ellipsis {
    background: transparent;
    padding: 0.5em;
}

.pagination-info {
    text-align: center;
    color: #666;
    margin-top: 10px;
    font-size: 0.9em;
}
```

## API Reference

### Constants

```go
const (
    DefaultItemsPerPage = 50  // Default items per page
    DefaultMaxVisible   = 9   // Default maximum visible pages
)
```

### Constructor Functions

#### `New(currentPage, totalItems, itemsPerPage int, baseURL string, queryParams url.Values) *Pagination`

Creates a new pagination instance with default settings (9 visible pages).

**Parameters:**
- `currentPage` - Current page number (1-indexed, e.g., 1, 2, 3...)
- `totalItems` - Total number of items
- `itemsPerPage` - Number of items per page
- `baseURL` - Base URL for pagination links
- `queryParams` - URL query parameters to preserve

**Returns:** `*Pagination`

#### `NewWithOptions(currentPage, totalItems, itemsPerPage, maxVisible int, baseURL string, queryParams url.Values, anchor string) *Pagination`

Creates a new pagination instance with every option.

**Parameters:**
- All parameters from `New()` plus:
- `maxVisible` - Maximum number of page links to display
- `anchor` - A fragment the page links end with ("" - none)

**Returns:** `*Pagination`

### Pagination Struct

```go
type Pagination struct {
    CurrentPage  int        // Current page (1-indexed)
    TotalPages   int        // Total number of pages
    TotalItems   int        // Total number of items
    ItemsPerPage int        // Items per page
    MaxVisible   int        // Maximum visible page numbers
    Offset       int        // SQL OFFSET value (0-indexed)
    BaseURL      string     // Base URL for links
    QueryParams  url.Values // Additional URL parameters
}
```

### Methods

#### Navigation Methods

- `HasPrevious() bool` - Returns true if there's a previous page
- `HasNext() bool` - Returns true if there's a next page
- `PreviousURL() string` - Returns URL for previous page (empty string if no previous page)
- `NextURL() string` - Returns URL for next page (empty string if no next page)
- `FirstURL() string` - Returns URL for first page
- `LastURL() string` - Returns URL for last page

#### Page Generation

- `GetPages() []PageInfo` - Returns slice of pages to display with ellipsis handling
- `GetRange() (start, end int)` - Returns the range of items on current page (e.g., 11-20)

### PageInfo Struct

```go
type PageInfo struct {
    Number     int    // Page number (1-indexed)
    IsCurrent  bool   // True if this is the current page
    IsEllipsis bool   // True if this is an ellipsis placeholder
    URL        string // URL for this page
}
```

## Examples

### Example 1: Basic Pagination with Database

```go
func productsHandler(w http.ResponseWriter, r *http.Request) {
    // Parse page number
    pageStr := r.URL.Query().Get("page")
    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        page = 1
    }
    
    // Get total count
    var totalCount int
    db.QueryRow("SELECT COUNT(*) FROM products").Scan(&totalCount)
    
    // Create paginator
    pager := pagination.New(page, totalCount, 25, r.URL.Path, r.URL.Query())
    
    // Fetch products
    var products []Product
    db.Query(`
        SELECT * FROM products 
        ORDER BY created_at DESC 
        LIMIT ? OFFSET ?
    `, pager.ItemsPerPage, pager.Offset).Scan(&products)
    
    // Get display range
    start, end := pager.GetRange()
    
    // Render
    render(w, "products.html", map[string]interface{}{
        "products":   products,
        "pagination": pager,
        "start":      start,
        "end":        end,
    })
}
```

### Example 2: Compact Pagination (5 visible pages)

```go
pager := pagination.NewWithOptions(
    page,
    totalCount,
    20,
    5, // Show only 5 page numbers
    r.URL.Path,
    r.URL.Query(),
    "",
)
```

### Example 3: Preserving Search Parameters

```go
// When you have search filters, they're automatically preserved
// URL: /products?category=electronics&sort=price&page=3
pager := pagination.New(
    page,
    totalCount,
    10,
    r.URL.Path,
    r.URL.Query(), // Preserves category and sort params
)
// Generated URLs will include: ?category=electronics&sort=price&page=X
```

### Example 4: Displaying Item Range

```go
pager := pagination.New(page, 150, 10, r.URL.Path, r.URL.Query())
start, end := pager.GetRange()

fmt.Printf("Showing %d-%d of %d items", start, end, pager.TotalItems)
// Output: "Showing 11-20 of 150 items" (for page 2)
```

## URL Generation

The package uses **1-based page numbering** in URLs for better user experience:

```
Page 1: /products              (no page parameter)
Page 2: /products?page=2
Page 3: /products?page=3
...
```

This is more intuitive than 0-based indexing and matches industry standards (GitHub, Google, etc.).

## Page Number Calculation

**Example with maxVisible=9:**

```
Page 1:   1 2 3 4 5 6 7 8 9 …
Page 5:   1 2 3 4 [5] 6 7 8 9 …
Page 15:  … 11 12 13 14 [15] 16 17 18 19 …
Page 50:  … 42 43 44 45 46 47 48 49 [50]
```

**Example with maxVisible=5:**

```
Page 1:   1 2 3 4 5 …
Page 3:   1 2 [3] 4 5 …
Page 10:  … 8 9 [10] 11 12 …
Page 20:  … 16 17 18 19 [20]
```

## Edge Cases Handled

- Invalid page numbers (< 1) default to page 1
- Page numbers exceeding total pages are capped to last page
- Invalid items per page defaults to 50
- Empty or nil query parameters are handled safely
- Zero total items creates a single empty page

## Contributing

Pull requests are welcome! For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License

[MIT](https://choosealicense.com/licenses/mit/)