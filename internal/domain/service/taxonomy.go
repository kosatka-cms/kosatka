package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// Guard errors returned by admin taxonomy CRUD when a delete would leave the
// dictionary in an inconsistent state (orphaned taxonomy_index rows or
// dangling parent_id references). Callers should use errors.Is to detect them.
var (
	ErrTaxonomyTermInUse          = errors.New("taxonomy term is tagged on existing content")
	ErrTaxonomyTermHasChildren    = errors.New("taxonomy term has child terms")
	ErrTaxonomyVocabularyHasTerms = errors.New("taxonomy vocabulary has terms")
)

// Errors returned when a term's new parent would break the tree: the
// parent is the term itself or one of its descendants (a loop), or not a
// term of the term's vocabulary. See DefaultTaxonomyService.checkTermParent.
var (
	ErrTaxonomyTermParentLoop    = errors.New("taxonomy term parent is the term itself or one of its descendants")
	ErrTaxonomyTermParentInvalid = errors.New("taxonomy term parent is not a term of the same vocabulary")
)

// TaxonomyService - the vocabularies and their terms, and the terms of the
// nodes (the tags typed in a form, the terms index).
type TaxonomyService interface {
	NodeTermsByID(ctx context.Context, nID int) (entity.TaxonomyTerms, error)
	// NodeTermsByIDs - NodeTermsByID batched over many node IDs, see
	// entity.TaxonomyRepository.NodeTermsByIDs' own doc comment.
	NodeTermsByIDs(ctx context.Context, nodeIDs []int) (map[int]entity.TaxonomyTerms, error)
	TermBySlug(ctx context.Context, slug string) (*entity.TaxonomyTerm, error)
	CountNodesByTermID(ctx context.Context, termID int) (int, error)
	// TagCloud - the tag cloud block's terms (tag_cloud.go).
	TagCloud(ctx context.Context, vocabularyID, count, levels int) ([]entity.TagCloudTerm, error)
	SearchTermsByName(ctx context.Context, vocabularyID int, query string, limit int) (entity.TaxonomyTerms, error)
	TermsByVocabularyID(ctx context.Context, vocabularyID int) (entity.TaxonomyTerms, error)
	CreateTerm(ctx context.Context, vocabularyID int, name, slug string, createdBy int) (*entity.TaxonomyTerm, error)
	UpdateNodeTerms(ctx context.Context, nodeID int, termIDs []int) error
	// FieldTermIDsForNode - see entity.TaxonomyRepository.FieldTermIDsForNode.
	FieldTermIDsForNode(ctx context.Context, nodeID int) ([]int, error)
	ProcessTagInputs(ctx context.Context, inputs []entity.TagSpecification, vocabularyID int, userID int) ([]int, error)

	// Admin CRUD for taxonomy vocabularies and terms.
	Vocabularies(ctx context.Context) (entity.TaxonomyVocabularies, error)
	VocabularyByID(ctx context.Context, id int) (*entity.TaxonomyVocabulary, error)
	// GenerateTermSlug - URL for a term of the vocabulary:
	// "<prefix>/<term>", unique across all terms (term pages resolve by
	// slug alone). termID - the term the URL is for when it already
	// exists (its own current URL isn't taken), 0 for a new term.
	GenerateTermSlug(ctx context.Context, vocabularyID int, name string, termID int) (string, error)
	CreateVocabulary(ctx context.Context, name, slug, termPathPrefix, desc string, publicTermPages bool, weight, createdBy int) (*entity.TaxonomyVocabulary, error)
	UpdateVocabulary(ctx context.Context, id int, name, slug, termPathPrefix, desc string, publicTermPages bool, weight int) (*entity.TaxonomyVocabulary, error)
	UpdateVocabularySortOrder(ctx context.Context, id int, weight int) error
	DeleteVocabulary(ctx context.Context, id int) error

	TermByID(ctx context.Context, id int) (*entity.TaxonomyTerm, error)
	TermsByIDs(ctx context.Context, ids []int) (map[int]entity.TaxonomyTerm, error)
	// CreateTermFull, UpdateTerm and UpdateTermSortOrder refuse a parent
	// from another vocabulary (ErrTaxonomyTermParentInvalid) and, for an
	// existing term, the term itself or its descendant
	// (ErrTaxonomyTermParentLoop).
	CreateTermFull(ctx context.Context, vocabularyID int, name, slug, desc string, weight, parentID, createdBy int) (*entity.TaxonomyTerm, error)
	UpdateTerm(ctx context.Context, id int, name, slug, desc string, weight, parentID int) (*entity.TaxonomyTerm, error)
	// UpdateTermPageTitle - entity.TaxonomyTerm.PageTitle.
	UpdateTermPageTitle(ctx context.Context, id int, pageTitle string) error
	UpdateTermSortOrder(ctx context.Context, id int, weight, parentID int) error
	DeleteTerm(ctx context.Context, id int) error
}

// DefaultTaxonomyService - the TaxonomyService on the taxonomy repository.
type DefaultTaxonomyService struct {
	repository entity.TaxonomyRepository
	logger     *slog.Logger
}

// NewTaxonomyService - the taxonomy service on r (the vocabularies, terms
// and the nodes' terms) and baseLogger.
func NewTaxonomyService(
	r entity.TaxonomyRepository,
	baseLogger *slog.Logger,
) TaxonomyService {
	return &DefaultTaxonomyService{
		repository: r,
		logger:     baseLogger.With("component", "taxonomy.service"),
	}
}

// NodeTermsByID - the terms of node nID (the terms index), with their
// vocabularies.
func (s *DefaultTaxonomyService) NodeTermsByID(ctx context.Context, nID int) (entity.TaxonomyTerms, error) {
	ct, err := s.repository.NodeTermsByID(ctx, nID)
	if err != nil {
		return nil, fmt.Errorf("finding terms for node (nID %v): %w", nID, err)
	}

	return ct, nil
}

// NodeTermsByIDs - see TaxonomyService.NodeTermsByIDs.
func (s *DefaultTaxonomyService) NodeTermsByIDs(ctx context.Context, nodeIDs []int) (map[int]entity.TaxonomyTerms, error) {
	ct, err := s.repository.NodeTermsByIDs(ctx, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("finding terms for nodes (nodeIDs %v): %w", nodeIDs, err)
	}

	return ct, nil
}

// TermBySlug - the term whose slug (URL) is slug.
func (s *DefaultTaxonomyService) TermBySlug(ctx context.Context, slug string) (*entity.TaxonomyTerm, error) {
	term, err := s.repository.TermBySlug(ctx, slug)
	return entity.Found(term, err, "finding term by slug (slug %v)", slug)
}

// CountNodesByTermID - how many nodes, of any status, the term is on.
func (s *DefaultTaxonomyService) CountNodesByTermID(ctx context.Context, termID int) (int, error) {
	count, err := s.repository.CountNodesByTermID(ctx, termID)
	if err != nil {
		return 0, fmt.Errorf("counting nodes by term ID (termID %v): %w", termID, err)
	}

	return count, nil
}

// SearchTermsByName - at most limit terms of the vocabulary whose name
// has query (any case), by weight and name.
func (s *DefaultTaxonomyService) SearchTermsByName(ctx context.Context, vocabularyID int, query string, limit int) (entity.TaxonomyTerms, error) {
	terms, err := s.repository.SearchTermsByName(ctx, vocabularyID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("searching terms by name (vocabularyID %v, query %v): %w", vocabularyID, query, err)
	}

	return terms, nil
}

// TermsByVocabularyID - every term of the vocabulary, by weight and name.
func (s *DefaultTaxonomyService) TermsByVocabularyID(ctx context.Context, vocabularyID int) (entity.TaxonomyTerms, error) {
	terms, err := s.repository.TermsByVocabularyID(ctx, vocabularyID)
	if err != nil {
		return nil, fmt.Errorf("finding terms by vocabulary ID (vocabularyID %v): %w", vocabularyID, err)
	}

	return terms, nil
}

// CreateTerm creates the term name (at slug) in the vocabulary, by user
// createdBy.
func (s *DefaultTaxonomyService) CreateTerm(ctx context.Context, vocabularyID int, name, slug string, createdBy int) (*entity.TaxonomyTerm, error) {
	term, err := s.repository.CreateTerm(ctx, vocabularyID, name, slug, createdBy)
	if err != nil {
		return nil, fmt.Errorf("creating term (vocabularyID %v, name %v, slug %v): %w", vocabularyID, name, slug, err)
	}

	return term, nil
}

// FieldTermIDsForNode - see TaxonomyService.FieldTermIDsForNode.
func (s *DefaultTaxonomyService) FieldTermIDsForNode(ctx context.Context, nodeID int) ([]int, error) {
	return s.repository.FieldTermIDsForNode(ctx, nodeID)
}

// UpdateNodeTerms sets the node's terms in the terms index to termIDs.
func (s *DefaultTaxonomyService) UpdateNodeTerms(ctx context.Context, nodeID int, termIDs []int) error {
	err := s.repository.UpdateNodeTerms(ctx, nodeID, termIDs)
	if err != nil {
		return fmt.Errorf("updating node terms (nodeID %v): %w", nodeID, err)
	}

	return nil
}

func (s *DefaultTaxonomyService) resolveExistingTagID(ctx context.Context, vocID int, name string) int {
	nameNorm := strings.TrimSpace(name)
	if nameNorm == "" {
		return 0
	}
	terms, err := s.SearchTermsByName(ctx, vocID, nameNorm, 80)
	LogFallback(s.logger, err, "searching terms by name", "vocabularyID", vocID)
	if err != nil || len(terms) == 0 {
		return 0
	}
	for _, t := range terms {
		if strings.EqualFold(strings.TrimSpace(t.Name), nameNorm) {
			return t.ID
		}
	}
	return 0
}

// ProcessTagInputs - the term IDs of a form's tags in the vocabulary
// vocID, each once: an existing term's ID as given, a new tag's - the
// term of that name (any case), else one created by user userID.
func (s *DefaultTaxonomyService) ProcessTagInputs(ctx context.Context, inputs []entity.TagSpecification, vocID int, userID int) ([]int, error) {
	termIDs := make([]int, 0, len(inputs))

	for _, input := range inputs {
		value := strings.TrimSpace(input.Value)
		if value == "" {
			continue
		}

		if input.IsNew {
			tagName := value
			if existingID := s.resolveExistingTagID(ctx, vocID, tagName); existingID > 0 {
				termIDs = append(termIDs, existingID)
				continue
			}
			// A failure fails the whole: the terms are made in the content's
			// save transaction, which a failed statement leaves unusable.
			slug, err := s.GenerateTermSlug(ctx, vocID, value, 0)
			if err != nil {
				return nil, fmt.Errorf("making the slug of a new tag (name %q): %w", tagName, err)
			}
			newTerm, err := s.CreateTerm(ctx, vocID, tagName, slug, userID)
			if err != nil {
				return nil, fmt.Errorf("creating a new tag (vocabularyID %v): %w", vocID, err)
			}

			termIDs = append(termIDs, newTerm.ID)
		} else {
			if tagID, parseErr := strconv.Atoi(value); parseErr == nil {
				termIDs = append(termIDs, tagID)
			}
		}
	}

	// A term named twice ("5,new:Foo" where Foo is 5, "new:Foo,new:foo")
	// is in the result once.
	return dedupeInts(termIDs), nil
}

// Vocabularies - every vocabulary.
func (s *DefaultTaxonomyService) Vocabularies(ctx context.Context) (entity.TaxonomyVocabularies, error) {
	vocabularies, err := s.repository.Vocabularies(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing vocabularies: %w", err)
	}

	return vocabularies, nil
}

// VocabularyByID - the vocabulary id.
func (s *DefaultTaxonomyService) VocabularyByID(ctx context.Context, id int) (*entity.TaxonomyVocabulary, error) {
	vocabulary, err := s.repository.VocabularyByID(ctx, id)
	return entity.Found(vocabulary, err, "finding vocabulary by id (id %v)", id)
}

// CreateVocabulary creates a vocabulary with these settings, by user
// createdBy.
func (s *DefaultTaxonomyService) CreateVocabulary(ctx context.Context, name, slug, termPathPrefix, desc string, publicTermPages bool, weight, createdBy int) (*entity.TaxonomyVocabulary, error) {
	vocabulary, err := s.repository.CreateVocabulary(ctx, name, slug, termPathPrefix, desc, publicTermPages, weight, createdBy)
	if err != nil {
		return nil, fmt.Errorf("creating vocabulary (name %v, slug %v): %w", name, slug, err)
	}

	return vocabulary, nil
}

// UpdateVocabulary saves the settings of vocabulary id.
func (s *DefaultTaxonomyService) UpdateVocabulary(ctx context.Context, id int, name, slug, termPathPrefix, desc string, publicTermPages bool, weight int) (*entity.TaxonomyVocabulary, error) {
	vocabulary, err := s.repository.UpdateVocabulary(ctx, id, name, slug, termPathPrefix, desc, publicTermPages, weight)
	if err != nil {
		return nil, fmt.Errorf("updating vocabulary (id %v): %w", id, err)
	}

	return vocabulary, nil
}

// UpdateVocabularySortOrder sets only the weight of vocabulary id (a
// drag-and-drop reorder).
func (s *DefaultTaxonomyService) UpdateVocabularySortOrder(ctx context.Context, id int, weight int) error {
	if err := s.repository.UpdateVocabularySortOrder(ctx, id, weight); err != nil {
		return fmt.Errorf("updating vocabulary sort order (id %v): %w", id, err)
	}
	return nil
}

// DeleteVocabulary deletes vocabulary id, refused while it has terms
// (ErrTaxonomyVocabularyHasTerms).
func (s *DefaultTaxonomyService) DeleteVocabulary(ctx context.Context, id int) error {
	terms, err := s.repository.TermsByVocabularyID(ctx, id)
	if err != nil {
		return fmt.Errorf("checking vocabulary terms before delete (id %v): %w", id, err)
	}
	if len(terms) > 0 {
		return fmt.Errorf("%w: %d terms in vocabulary %d", ErrTaxonomyVocabularyHasTerms, len(terms), id)
	}

	if err := s.repository.DeleteVocabulary(ctx, id); err != nil {
		return fmt.Errorf("deleting vocabulary (id %v): %w", id, err)
	}

	return nil
}

// TermByID - the term id, with its vocabulary.
func (s *DefaultTaxonomyService) TermByID(ctx context.Context, id int) (*entity.TaxonomyTerm, error) {
	term, err := s.repository.TermByID(ctx, id)
	return entity.Found(term, err, "finding term by id (id %v)", id)
}

// TermsByIDs - the terms ids by ID; unknown IDs are left out.
func (s *DefaultTaxonomyService) TermsByIDs(ctx context.Context, ids []int) (map[int]entity.TaxonomyTerm, error) {
	terms, err := s.repository.TermsByIDs(ctx, ids)
	return entity.Wrapped(terms, err, "finding terms by ids (ids %v)", ids)
}

// CreateTermFull - see TaxonomyService.CreateTermFull.
func (s *DefaultTaxonomyService) CreateTermFull(ctx context.Context, vocabularyID int, name, slug, desc string, weight, parentID, createdBy int) (*entity.TaxonomyTerm, error) {
	if err := s.checkTermParent(ctx, vocabularyID, 0, parentID); err != nil {
		return nil, fmt.Errorf("creating term (full) (vocabularyID %v, name %v): %w", vocabularyID, name, err)
	}
	term, err := s.repository.CreateTermFull(ctx, vocabularyID, name, slug, desc, weight, parentID, createdBy)
	if err != nil {
		return nil, fmt.Errorf("creating term (full) (vocabularyID %v, name %v, slug %v): %w", vocabularyID, name, slug, err)
	}

	return term, nil
}

// UpdateTermPageTitle - see TaxonomyService.UpdateTermPageTitle.
func (s *DefaultTaxonomyService) UpdateTermPageTitle(ctx context.Context, id int, pageTitle string) error {
	return s.repository.UpdateTermPageTitle(ctx, id, strings.TrimSpace(pageTitle))
}

// UpdateTerm saves term id; its parent must be a term of its vocabulary
// (ErrTaxonomyTermParentInvalid), not itself or below it
// (ErrTaxonomyTermParentLoop).
func (s *DefaultTaxonomyService) UpdateTerm(ctx context.Context, id int, name, slug, desc string, weight, parentID int) (*entity.TaxonomyTerm, error) {
	if err := s.checkExistingTermParent(ctx, id, parentID); err != nil {
		return nil, fmt.Errorf("updating term (id %v): %w", id, err)
	}
	term, err := s.repository.UpdateTerm(ctx, id, name, slug, desc, weight, parentID)
	if err != nil {
		return nil, fmt.Errorf("updating term (id %v): %w", id, err)
	}

	return term, nil
}

// UpdateTermSortOrder sets only the weight and the parent of term id (a
// drag-and-drop reorder), the parent checked as by UpdateTerm.
func (s *DefaultTaxonomyService) UpdateTermSortOrder(ctx context.Context, id int, weight, parentID int) error {
	if err := s.checkExistingTermParent(ctx, id, parentID); err != nil {
		return fmt.Errorf("updating term sort order (id %v): %w", id, err)
	}
	if err := s.repository.UpdateTermSortOrder(ctx, id, weight, parentID); err != nil {
		return fmt.Errorf("updating term sort order (id %v): %w", id, err)
	}
	return nil
}

// checkExistingTermParent - checkTermParent for the existing term id,
// in its own vocabulary. No parent (parentID 0) needs no query, so a
// reorder of a flat vocabulary costs nothing extra.
func (s *DefaultTaxonomyService) checkExistingTermParent(ctx context.Context, id, parentID int) error {
	if parentID <= 0 {
		return nil
	}
	term, err := s.repository.TermByID(ctx, id)
	if err != nil {
		return fmt.Errorf("loading term (id %v): %w", id, err)
	}
	return s.checkTermParent(ctx, term.Vocabulary.ID, id, parentID)
}

// checkTermParent reports whether parentID may be the parent of term id
// (0 - a new term) of vocabularyID: a term of the same vocabulary, and
// neither the term itself nor one of its descendants - that would make
// a loop in the tree, which the tree's walkers (SortByHierarchy and the
// like) never leave. parentID 0 - no parent. The walk goes up from the
// parent, one query per level; meeting the term means the parent is
// below it.
func (s *DefaultTaxonomyService) checkTermParent(ctx context.Context, vocabularyID, id, parentID int) error {
	if parentID <= 0 {
		return nil
	}
	if parentID == id {
		return fmt.Errorf("%w: term %d", ErrTaxonomyTermParentLoop, id)
	}
	parent, err := s.repository.TermByID(ctx, parentID)
	if errors.Is(err, entity.ErrEntityNotFound) {
		return fmt.Errorf("%w: no term %d", ErrTaxonomyTermParentInvalid, parentID)
	}
	if err != nil {
		return fmt.Errorf("loading parent term (id %v): %w", parentID, err)
	}
	if parent.Vocabulary.ID != vocabularyID {
		return fmt.Errorf("%w: term %d is in vocabulary %d, not %d", ErrTaxonomyTermParentInvalid, parentID, parent.Vocabulary.ID, vocabularyID)
	}
	if id <= 0 {
		return nil
	}
	seen := map[int]bool{parentID: true}
	for ancestorID := parent.ParentID; ancestorID > 0 && !seen[ancestorID]; {
		if ancestorID == id {
			return fmt.Errorf("%w: term %d is below term %d", ErrTaxonomyTermParentLoop, parentID, id)
		}
		seen[ancestorID] = true
		ancestor, err := s.repository.TermByID(ctx, ancestorID)
		if errors.Is(err, entity.ErrEntityNotFound) {
			break
		}
		if err != nil {
			return fmt.Errorf("loading ancestor term (id %v): %w", ancestorID, err)
		}
		ancestorID = ancestor.ParentID
	}
	return nil
}

// DeleteTerm deletes term id, refused while it is on content
// (ErrTaxonomyTermInUse) or has child terms (ErrTaxonomyTermHasChildren).
func (s *DefaultTaxonomyService) DeleteTerm(ctx context.Context, id int) error {
	count, err := s.repository.CountNodesByTermID(ctx, id)
	if err != nil {
		return fmt.Errorf("checking term usage before delete (id %v): %w", id, err)
	}
	if count > 0 {
		return fmt.Errorf("%w: %d materials tagged with term %d", ErrTaxonomyTermInUse, count, id)
	}

	term, err := s.repository.TermByID(ctx, id)
	if err != nil {
		return fmt.Errorf("loading term before delete (id %v): %w", id, err)
	}
	children, err := s.repository.TermsByVocabularyID(ctx, term.Vocabulary.ID)
	if err != nil {
		return fmt.Errorf("checking child terms before delete (id %v): %w", id, err)
	}
	for _, child := range children {
		if child.ParentID == id {
			return fmt.Errorf("%w: term %d has children", ErrTaxonomyTermHasChildren, id)
		}
	}

	if err := s.repository.DeleteTerm(ctx, id); err != nil {
		return fmt.Errorf("deleting term (id %v): %w", id, err)
	}

	return nil
}

// NormalizeTermPathPrefix cleans a vocabulary's term path prefix: each
// "/"-separated part through slug.Generate, empty parts dropped.
func NormalizeTermPathPrefix(raw string) string {
	var parts []string
	for _, p := range strings.Split(raw, "/") {
		if p = slug.Generate(strings.TrimSpace(p)); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "/")
}

// GenerateTermSlug - see TaxonomyService.GenerateTermSlug.
func (s *DefaultTaxonomyService) GenerateTermSlug(ctx context.Context, vocabularyID int, name string, termID int) (string, error) {
	vocabulary, err := s.repository.VocabularyByID(ctx, vocabularyID)
	if err != nil {
		return "", fmt.Errorf("loading vocabulary (id %v): %w", vocabularyID, err)
	}
	prefix := NormalizeTermPathPrefix(vocabulary.TermPathPrefix)
	if prefix == "" {
		prefix = NormalizeTermPathPrefix(vocabulary.Name)
	}
	base := slug.Generate(name)
	if base == "" {
		base = "term"
	}
	if prefix != "" {
		base = prefix + "/" + base
	}
	for i := 0; i < 1000; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		taken, err := s.repository.TermBySlug(ctx, candidate)
		if errors.Is(err, entity.ErrEntityNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("checking term slug (slug %v): %w", candidate, err)
		}
		if termID > 0 && taken.ID == termID {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not pick a unique slug for term %q", name)
}
