package pagination

import (
	"net/url"
	"testing"
)

func TestNewClampsCurrentPage(t *testing.T) {
	p := New(0, 100, 10, "/list", nil)
	if p.CurrentPage != 1 {
		t.Fatalf("expected page < 1 to clamp to 1, got %d", p.CurrentPage)
	}

	p2 := New(999, 100, 10, "/list", nil)
	if p2.CurrentPage != p2.TotalPages {
		t.Fatalf("expected page beyond total to clamp to %d, got %d", p2.TotalPages, p2.CurrentPage)
	}
}

func TestNewDefaults(t *testing.T) {
	p := New(1, 5, 0, "/list", nil)
	if p.ItemsPerPage != DefaultItemsPerPage {
		t.Fatalf("expected default items per page %d, got %d", DefaultItemsPerPage, p.ItemsPerPage)
	}
	if p.MaxVisible != DefaultMaxVisible {
		t.Fatalf("expected default max visible %d, got %d", DefaultMaxVisible, p.MaxVisible)
	}
	if p.TotalPages != 1 {
		t.Fatalf("expected at least 1 total page, got %d", p.TotalPages)
	}
}

func TestOffsetAndTotalPages(t *testing.T) {
	p := New(3, 95, 10, "/list", nil)
	if p.TotalPages != 10 {
		t.Fatalf("expected 10 total pages, got %d", p.TotalPages)
	}
	if p.Offset != 20 {
		t.Fatalf("expected offset 20, got %d", p.Offset)
	}
}

func TestHasPreviousNext(t *testing.T) {
	p := New(1, 30, 10, "/list", nil)
	if p.HasPrevious() {
		t.Fatal("first page should have no previous")
	}
	if !p.HasNext() {
		t.Fatal("first page of 3 should have next")
	}

	last := New(3, 30, 10, "/list", nil)
	if !last.HasPrevious() {
		t.Fatal("last page should have previous")
	}
	if last.HasNext() {
		t.Fatal("last page should have no next")
	}
}

func TestURLs(t *testing.T) {
	q := url.Values{"filter": {"active"}, "page": {"5"}}
	p := New(2, 50, 10, "/list", q)

	if got := p.FirstURL(); got != "/list?filter=active" {
		t.Fatalf("FirstURL = %q", got)
	}
	if got := p.NextURL(); got != "/list?filter=active&page=3" {
		t.Fatalf("NextURL = %q", got)
	}
	if got := p.PreviousURL(); got != "/list?filter=active" {
		t.Fatalf("PreviousURL = %q", got)
	}
	if got := p.LastURL(); got != "/list?filter=active&page=5" {
		t.Fatalf("LastURL = %q", got)
	}
}

func TestURLsWithAnchor(t *testing.T) {
	p := NewWithAnchor(2, 30, 10, "/post/1", nil, "comments")
	if got := p.NextURL(); got != "/post/1?page=3#comments" {
		t.Fatalf("NextURL with anchor = %q", got)
	}
}

func TestPreviousNextURLEmptyAtBounds(t *testing.T) {
	p := New(1, 30, 10, "/list", nil)
	if got := p.PreviousURL(); got != "" {
		t.Fatalf("expected empty PreviousURL on first page, got %q", got)
	}

	last := New(3, 30, 10, "/list", nil)
	if got := last.NextURL(); got != "" {
		t.Fatalf("expected empty NextURL on last page, got %q", got)
	}
}

func TestGetRange(t *testing.T) {
	p := New(2, 25, 10, "/list", nil)
	start, end := p.GetRange()
	if start != 11 || end != 20 {
		t.Fatalf("GetRange() = %d,%d want 11,20", start, end)
	}

	last := New(3, 25, 10, "/list", nil)
	start, end = last.GetRange()
	if start != 21 || end != 25 {
		t.Fatalf("GetRange() = %d,%d want 21,25", start, end)
	}

	empty := New(1, 0, 10, "/list", nil)
	start, end = empty.GetRange()
	if start != 0 || end != 0 {
		t.Fatalf("GetRange() on empty = %d,%d want 0,0", start, end)
	}
}

func TestGetPagesNoEllipsisWhenFewPages(t *testing.T) {
	p := New(1, 20, 10, "/list", nil)
	pages := p.GetPages()
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}
	for _, pg := range pages {
		if pg.IsEllipsis {
			t.Fatal("did not expect ellipsis with only 2 pages")
		}
	}
	if !pages[0].IsCurrent {
		t.Fatal("expected page 1 to be current")
	}
}

func TestGetPagesEmptyWhenSinglePage(t *testing.T) {
	p := New(1, 5, 10, "/list", nil)
	if pages := p.GetPages(); len(pages) != 0 {
		t.Fatalf("expected no pages to display for a single page, got %d", len(pages))
	}
}
