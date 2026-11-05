package interfaces

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

type fakeFallbackTaxonomy struct {
	service.TaxonomyService
	terms map[string]*entity.TaxonomyTerm
}

func (f fakeFallbackTaxonomy) TermBySlug(_ context.Context, slug string) (*entity.TaxonomyTerm, error) {
	if t, ok := f.terms[slug]; ok {
		return t, nil
	}
	return nil, entity.ErrEntityNotFound
}

// TestTermPageFallbackDeclines - no term, a term of a vocabulary
// without public term pages, or a static-looking path: the fallback
// writes nothing and returns false, so the catch-all renders its 404.
func TestTermPageFallbackDeclines(t *testing.T) {
	c := &TaxonomyController{taxonomyService: fakeFallbackTaxonomy{terms: map[string]*entity.TaxonomyTerm{
		"otsenki/5": {ID: 1, Slug: "otsenki/5", Vocabulary: entity.TaxonomyVocabulary{ID: 42}},
	}}}
	for _, path := range []string{"/no/such-term", "/otsenki/5", "/static/app.js", "/"} {
		rec := httptest.NewRecorder()
		if c.TermPageFallback(rec, httptest.NewRequest(http.MethodGet, path, nil)) {
			t.Errorf("%s: fallback served a page", path)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("%s: fallback wrote a response", path)
		}
	}
}
