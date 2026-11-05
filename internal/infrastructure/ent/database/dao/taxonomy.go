package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToTaxonomyTermsEntity converts taxonomy_index rows (with their term
// loaded) to terms. A row whose term no longer exists (an index row left
// behind by a deleted term) is skipped, not dereferenced.
func ToTaxonomyTermsEntity(is []*generated.TaxonomyIndex) entity.TaxonomyTerms {
	terms := make(entity.TaxonomyTerms, 0, len(is))
	for _, index := range is {
		if index == nil || index.Edges.TaxonomyTermData == nil {
			continue
		}
		terms = append(terms, toTaxonomyTermEntity(index))
	}

	return terms
}

func toTaxonomyTermEntity(i *generated.TaxonomyIndex) entity.TaxonomyTerm {
	return entity.TaxonomyTerm{
		ID:         i.Edges.TaxonomyTermData.ID,
		Vocabulary: ToTaxonomyVocabularyEntity(i.Edges.TaxonomyTermData.Edges.Vocabulary),
		Name:       i.Edges.TaxonomyTermData.Name,
		Slug:       i.Edges.TaxonomyTermData.Slug,
		CreatedAt:  i.CreatedAt,
	}
}

// ToTaxonomyVocabularyEntity converts a generated TaxonomyVocabulary to its domain entity.
func ToTaxonomyVocabularyEntity(v *generated.TaxonomyVocabulary) entity.TaxonomyVocabulary {
	if v == nil {
		return entity.TaxonomyVocabulary{}
	}
	return entity.TaxonomyVocabulary{
		ID:              v.ID,
		Name:            v.Name,
		Desc:            v.Description,
		Weight:          v.Weight,
		Slug:            v.Slug,
		TermPathPrefix:  v.TermPathPrefix,
		PublicTermPages: v.PublicTermPages,
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
	}
}

// ToTaxonomyTermEntity converts a generated TaxonomyTermData (nil - nil)
// to its domain entity, with the vocabulary when loaded.
func ToTaxonomyTermEntity(t *generated.TaxonomyTermData) *entity.TaxonomyTerm {
	if t == nil {
		return nil
	}

	parentID := 0
	if t.ParentID != nil {
		parentID = *t.ParentID
	}

	term := &entity.TaxonomyTerm{
		ID:        t.ID,
		Name:      t.Name,
		Desc:      t.Description,
		PageTitle: t.PageTitle,
		Weight:    t.Weight,
		Slug:      t.Slug,
		ParentID:  parentID,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.Edges.Vocabulary != nil {
		term.Vocabulary = ToTaxonomyVocabularyEntity(t.Edges.Vocabulary)
	}
	return term
}
