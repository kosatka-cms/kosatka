package entity

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSearchResultJSON - the live-search API keys app.js reads.
func TestSearchResultJSON(t *testing.T) {
	b, err := json.Marshal(SearchResult{ID: 1, Type: "article", URL: "/articles/foo", Title: "Heading", Subtitle: "Title", ImageURL: "/i.webp", Badge: "Section", TypeLabel: "Article"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"title":"Heading"`, `"subtitle":"Title"`, `"badge":"Section"`, `"type_label":"Article"`, `"image_url":"/i.webp"`, `"url":"/articles/foo"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s in %s", want, b)
		}
	}
}
