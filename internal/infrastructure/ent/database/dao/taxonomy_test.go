package dao

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// TestToTaxonomyTermsEntitySkipsMissingTerm checks that an index row whose
// term was deleted (nil edge) is skipped instead of panicking.
func TestToTaxonomyTermsEntitySkipsMissingTerm(t *testing.T) {
	ok := &generated.TaxonomyIndex{Nid: 1, Tid: 5}
	ok.Edges.TaxonomyTermData = &generated.TaxonomyTermData{ID: 5, Name: "Tag", Slug: "tags/tag"}
	orphan := &generated.TaxonomyIndex{Nid: 1, Tid: 662}

	terms := ToTaxonomyTermsEntity([]*generated.TaxonomyIndex{orphan, ok, nil})
	if len(terms) != 1 || terms[0].ID != 5 || terms[0].Name != "Tag" {
		t.Fatalf("got %+v", terms)
	}
}
