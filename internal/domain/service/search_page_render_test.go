package service

import (
	"bytes"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestSearchResultsTemplate - the results page lists results (link,
// subtitle, snippet with the words marked, type and badge, image only
// when there is one), says nothing was found, or asks for a longer query.
func TestSearchResultsTemplate(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := NewTemplateService(false)
	render := func(data map[string]any) string {
		t.Helper()
		data["searchMinChars"] = 3
		var buf bytes.Buffer
		if err := svc.RenderNamed(&buf, "content", data, "search/results.html", "pagination.html"); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}

	out := render(map[string]any{"query": "bat", "searched": true, "total": 2, "results": []entity.SearchResult{
		{Type: "event", URL: "/events/bat-night", Title: "Bat night", Subtitle: "A walk in the park", ImageURL: "/files/b.webp", TypeLabel: "Events", Badge: "Outdoor",
			Snippet: entity.SearchSnippet{Before: true, Parts: []entity.SearchSnippetPart{{Text: "see the "}, {Text: "bats", Match: true}, {Text: " <fly>"}}}},
		{Type: "news", URL: "/news/bat", Title: "Bat news", TypeLabel: "News"},
	}})
	for _, want := range []string{`value="bat"`, `2 results`, `<a href="/events/bat-night" class="search-result-title">Bat night</a>`, `A walk in the park`, `Events · Outdoor`, `src="/files/b.webp"`, `search-result-news`,
		`<p class="search-result-snippet">...see the <mark>bats</mark> &lt;fly&gt;</p>`} {
		if !strings.Contains(out, want) {
			t.Errorf("results: no %q in %s", want, out)
		}
	}
	if strings.Contains(out, "pager") {
		t.Error("one page needs no pager")
	}
	paged := render(map[string]any{"query": "bat", "searched": true, "total": 45, "results": []entity.SearchResult{{URL: "/a", Title: "A"}},
		"pagination": pagination.New(2, 45, 20, "/search", url.Values{"q": {"bat & co"}, "page": {"2"}})})
	for _, want := range []string{"45 results", `class="pager-current">2<`, `href="/search?page=3&amp;q=bat&#43;%26&#43;co" data-page="3"`, `data-page="1">`} {
		if !strings.Contains(paged, want) {
			t.Errorf("paged: no %q in %s", want, paged)
		}
	}
	if strings.Count(out, "search-result-snippet") != 1 {
		t.Errorf("a result without a snippet shows one: %s", out)
	}
	if strings.Count(out, "<img") != 1 {
		t.Errorf("one image expected: %s", out)
	}
	if out := render(map[string]any{"query": "zzz", "searched": true}); !strings.Contains(out, "Nothing found.") {
		t.Errorf("empty: %s", out)
	}
	if out := render(map[string]any{"query": "ab", "queryTooShort": true}); !strings.Contains(out, "Enter at least 3 characters.") || strings.Contains(out, "Nothing found.") {
		t.Errorf("too short: %s", out)
	}
}
