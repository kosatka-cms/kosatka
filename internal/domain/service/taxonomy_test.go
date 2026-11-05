package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeTermSlugRepo struct {
	entity.TaxonomyRepository
	vocabularies map[int]entity.TaxonomyVocabulary
	slugs        map[string]int // slug → term ID
	created      []string
}

func (f *fakeTermSlugRepo) VocabularyByID(_ context.Context, id int) (*entity.TaxonomyVocabulary, error) {
	v, ok := f.vocabularies[id]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	return &v, nil
}

func (f *fakeTermSlugRepo) TermBySlug(_ context.Context, slug string) (*entity.TaxonomyTerm, error) {
	if id, ok := f.slugs[slug]; ok {
		return &entity.TaxonomyTerm{ID: id, Slug: slug}, nil
	}
	return nil, entity.ErrEntityNotFound
}

func (f *fakeTermSlugRepo) SearchTermsByName(context.Context, int, string, int) (entity.TaxonomyTerms, error) {
	return nil, nil
}

func (f *fakeTermSlugRepo) CreateTerm(_ context.Context, vid int, name, slug string, _ int) (*entity.TaxonomyTerm, error) {
	f.created = append(f.created, slug)
	return &entity.TaxonomyTerm{ID: len(f.created), Name: name, Slug: slug}, nil
}

// Cyrillic names on purpose: the tests check their transliteration into slugs.
func newTermSlugService() (*DefaultTaxonomyService, *fakeTermSlugRepo) {
	repo := &fakeTermSlugRepo{
		vocabularies: map[int]entity.TaxonomyVocabulary{
			3:  {ID: 3, Name: "Tags", TermPathPrefix: "tags"},
			11: {ID: 11, Name: "Платформа игр", TermPathPrefix: "platforma-igr"},
			44: {ID: 44, Name: "Режим игры"},
		},
		slugs: map[string]int{"tags/foto": 1, "tags/foto-1": 2},
	}
	return NewTaxonomyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil))).(*DefaultTaxonomyService), repo
}

func TestGenerateTermSlug(t *testing.T) {
	s, _ := newTermSlugService()
	for _, tc := range []struct {
		vid    int
		name   string
		termID int
		want   string
	}{
		{11, "PlayStation 5", 0, "platforma-igr/playstation-5"},
		{44, "Кооператив", 0, "rezhim-igry/kooperativ"}, // empty prefix → vocabulary name
		{3, "Фото", 0, "tags/foto-2"},                   // taken: tags/foto, tags/foto-1
		// An existing term's own address isn't taken: emptying the
		// slug of term 2 (tags/foto-1) gives it back, not tags/foto-2.
		{3, "Фото", 2, "tags/foto-1"},
		{3, "Фото", 1, "tags/foto"},
		{3, "Фото", 9, "tags/foto-2"},
	} {
		got, err := s.GenerateTermSlug(context.Background(), tc.vid, tc.name, tc.termID)
		if err != nil || got != tc.want {
			t.Errorf("GenerateTermSlug(%d, %q, %d) = %q, %v; want %q", tc.vid, tc.name, tc.termID, got, err, tc.want)
		}
	}
	if got := NormalizeTermPathPrefix(" /Платформа игр/ PC/ "); got != "platforma-igr/pc" {
		t.Errorf("NormalizeTermPathPrefix = %q", got)
	}
}

// TestProcessTagInputsUsesVocabularyPrefix - a new term typed into a
// taxonomy field gets its own vocabulary's URL prefix, not "tags/".
func TestProcessTagInputsUsesVocabularyPrefix(t *testing.T) {
	s, repo := newTermSlugService()
	if _, err := s.ProcessTagInputs(context.Background(), []entity.TagSpecification{{Value: "Steam Deck", IsNew: true}}, 11, 1); err != nil {
		t.Fatal(err)
	}
	if len(repo.created) != 1 || repo.created[0] != "platforma-igr/steam-deck" {
		t.Fatalf("created slugs = %v, want [platforma-igr/steam-deck]", repo.created)
	}
}

// TestProcessTagInputsOnce - a term given twice is in the result once,
// in the order first given.
func TestProcessTagInputsOnce(t *testing.T) {
	s, _ := newTermSlugService()
	ids, err := s.ProcessTagInputs(context.Background(), []entity.TagSpecification{{Value: "7"}, {Value: "5"}, {Value: "7"}}, 11, 1)
	if err != nil || len(ids) != 2 || ids[0] != 7 || ids[1] != 5 {
		t.Fatalf("got %v %v, want [7 5]", ids, err)
	}
}

// fakeTermTreeRepo - terms by ID for the parent checks; records the
// writes that got through.
type fakeTermTreeRepo struct {
	entity.TaxonomyRepository
	terms  map[int]entity.TaxonomyTerm
	writes []int
}

func (f *fakeTermTreeRepo) TermByID(_ context.Context, id int) (*entity.TaxonomyTerm, error) {
	t, ok := f.terms[id]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	return &t, nil
}

func (f *fakeTermTreeRepo) UpdateTerm(_ context.Context, id int, _, _, _ string, _, _ int) (*entity.TaxonomyTerm, error) {
	f.writes = append(f.writes, id)
	t := f.terms[id]
	return &t, nil
}

func (f *fakeTermTreeRepo) UpdateTermSortOrder(_ context.Context, id int, _, _ int) error {
	f.writes = append(f.writes, id)
	return nil
}

func (f *fakeTermTreeRepo) CreateTermFull(_ context.Context, vid int, name, slug, _ string, _, parentID, _ int) (*entity.TaxonomyTerm, error) {
	f.writes = append(f.writes, 0)
	return &entity.TaxonomyTerm{Vocabulary: entity.TaxonomyVocabulary{ID: vid}, Name: name, Slug: slug, ParentID: parentID}, nil
}

// TestTermParentChecks - a term can't be put under itself or its own
// descendant (a loop in the tree), nor under a term of another
// vocabulary, by any of the writes that set a parent.
func TestTermParentChecks(t *testing.T) {
	voc := func(id int) entity.TaxonomyVocabulary { return entity.TaxonomyVocabulary{ID: id} }
	// Vocabulary 1: 1 → 2 → 3, and 4 at the top; vocabulary 2: 5.
	repo := &fakeTermTreeRepo{terms: map[int]entity.TaxonomyTerm{
		1: {ID: 1, Vocabulary: voc(1)},
		2: {ID: 2, Vocabulary: voc(1), ParentID: 1},
		3: {ID: 3, Vocabulary: voc(1), ParentID: 2},
		4: {ID: 4, Vocabulary: voc(1)},
		5: {ID: 5, Vocabulary: voc(2)},
	}}
	s := NewTaxonomyService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	for _, tc := range []struct {
		id, parentID int
		want         error
	}{
		{1, 0, nil},
		{3, 1, nil}, // up the tree
		{1, 4, nil},
		{2, 4, nil},
		{1, 1, ErrTaxonomyTermParentLoop},
		{1, 2, ErrTaxonomyTermParentLoop}, // a child
		{1, 3, ErrTaxonomyTermParentLoop}, // a grandchild
		{2, 3, ErrTaxonomyTermParentLoop},
		{1, 5, ErrTaxonomyTermParentInvalid}, // another vocabulary
		{1, 99, ErrTaxonomyTermParentInvalid},
	} {
		repo.writes = nil
		_, err := s.UpdateTerm(ctx, tc.id, "Name", "slug", "", 0, tc.parentID)
		if !errors.Is(err, tc.want) || (tc.want != nil) != (err != nil) {
			t.Errorf("UpdateTerm(%d, parent %d) = %v, want %v", tc.id, tc.parentID, err, tc.want)
		}
		if (tc.want == nil) != (len(repo.writes) == 1) {
			t.Errorf("UpdateTerm(%d, parent %d): writes %v", tc.id, tc.parentID, repo.writes)
		}
		repo.writes = nil
		err = s.UpdateTermSortOrder(ctx, tc.id, 0, tc.parentID)
		if !errors.Is(err, tc.want) || (tc.want != nil) != (err != nil) {
			t.Errorf("UpdateTermSortOrder(%d, parent %d) = %v, want %v", tc.id, tc.parentID, err, tc.want)
		}
		if (tc.want == nil) != (len(repo.writes) == 1) {
			t.Errorf("UpdateTermSortOrder(%d, parent %d): writes %v", tc.id, tc.parentID, repo.writes)
		}
	}

	if _, err := s.CreateTermFull(ctx, 1, "New", "new", "", 0, 3, 1); err != nil {
		t.Errorf("CreateTermFull under a term of its vocabulary: %v", err)
	}
	if _, err := s.CreateTermFull(ctx, 1, "New", "new", "", 0, 5, 1); !errors.Is(err, ErrTaxonomyTermParentInvalid) {
		t.Errorf("CreateTermFull under a term of another vocabulary = %v, want ErrTaxonomyTermParentInvalid", err)
	}
}
