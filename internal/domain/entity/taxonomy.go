package entity

import (
	"context"
	"time"
)

// TaxonomyVocabularies - a list of vocabularies.
type TaxonomyVocabularies []TaxonomyVocabulary

// TaxonomyVocabulary - a vocabulary: a named set of terms (tags, rubrics)
// content is classified by.
type TaxonomyVocabulary struct {
	ID     int
	Name   string
	Desc   string
	Weight int
	Slug   string
	// TermPathPrefix - first part of the URL of this vocabulary's new
	// terms ("tags" → /tags/<term>), the terms' path pattern;
	// empty = derived from the vocabulary name. See
	// TaxonomyService.GenerateTermSlug.
	TermPathPrefix string
	// PublicTermPages - this vocabulary's terms have public pages at
	// their URL (/<term slug>, TaxonomyController.TermPageFallback);
	// off - the URL is 404 (a vocabulary used only for classification).
	PublicTermPages bool
	CreatedBy       User
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TaxonomyTerms - a list of terms.
type TaxonomyTerms []TaxonomyTerm

// TaxonomyTerm - a term of a vocabulary, under another (ParentID; 0 - a
// root term).
type TaxonomyTerm struct {
	ID         int
	Vocabulary TaxonomyVocabulary
	Name       string
	Desc       string
	// PageTitle - the term page's heading and [term:title] SEO token
	// (per-term meta title); empty - Name.
	PageTitle string
	Weight    int
	Slug      string
	ParentID  int
	CreatedBy User
	CreatedAt time.Time
	UpdatedAt time.Time
}

// HasPublicPage - the term has a public page at "/<Slug>": its
// vocabulary has PublicTermPages on. Templates link a term only then
// (else the link is a 404). Needs Vocabulary loaded with the term, as
// the repository's term queries do.
func (t TaxonomyTerm) HasPublicPage() bool {
	return t.Vocabulary.PublicTermPages
}

// TagCloudTerm - a term in the tag cloud (PluginBlockTagCloud): Count -
// the published materials it is on, Level - its size step, 1 (the
// fewest) to the block's levels (the most), Linked - its vocabulary has
// public term pages (PublicTermPages), so it links to its page.
type TagCloudTerm struct {
	Term   TaxonomyTerm
	Count  int
	Level  int
	Linked bool
}

// The tag cloud's settings (PluginConfig keys): the vocabulary whose
// terms it shows ("0" - every vocabulary), how many and in how many size
// levels.
const (
	TagCloudVocabularyKey = "vocabulary"
	TagCloudCountKey      = "count"
	TagCloudLevelsKey     = "levels"
)

// TagSpecification - a tag of a tags input: an existing term's ID, or a
// new tag's name (TaxonomyService.ProcessTagInputs finds or creates its
// term).
type TagSpecification struct {
	Value string // ID or name
	IsNew bool   // true = create a new term, false = use an existing one
}

// FilterByVID - the terms of vocabulary vID.
func (ts TaxonomyTerms) FilterByVID(vID int) TaxonomyTerms {
	var filteredTerms TaxonomyTerms
	for _, term := range ts {
		if term.Vocabulary.ID == vID {
			filteredTerms = append(filteredTerms, term)
		}
	}
	return filteredTerms
}

// IDs - the terms' IDs, in order.
func (ts TaxonomyTerms) IDs() []int {
	ids := make([]int, 0, len(ts))
	for _, t := range ts {
		ids = append(ids, t.ID)
	}
	return ids
}

// BuildHierarchy creates a map of terms with their display names including parent hierarchy
func (ts TaxonomyTerms) BuildHierarchy() map[int]string {
	// Map of ID to term for quick lookup.
	termMap := make(map[int]TaxonomyTerm)
	for _, t := range ts {
		termMap[t.ID] = t
	}

	result := make(map[int]string)
	for _, t := range ts {
		depth := ts.calculateDepth(t, termMap)
		prefix := ""
		for i := 0; i < depth; i++ {
			prefix += "-- "
		}
		result[t.ID] = prefix + t.Name
	}
	return result
}

// BuildDepths returns each term's nesting depth (0 = root), e.g. for
// drag-and-drop indentation in the terms admin.
func (ts TaxonomyTerms) BuildDepths() map[int]int {
	termMap := make(map[int]TaxonomyTerm)
	for _, t := range ts {
		termMap[t.ID] = t
	}
	result := make(map[int]int)
	for _, t := range ts {
		result[t.ID] = ts.calculateDepth(t, termMap)
	}
	return result
}

// calculateDepth calculates how deep a term is in the hierarchy
func (ts TaxonomyTerms) calculateDepth(term TaxonomyTerm, termMap map[int]TaxonomyTerm) int {
	if term.ParentID == 0 {
		return 0
	}

	parent, exists := termMap[term.ParentID]
	if !exists {
		return 0
	}

	return 1 + ts.calculateDepth(parent, termMap)
}

// SortByHierarchy sorts terms by parent-child relationship
func (ts TaxonomyTerms) SortByHierarchy() TaxonomyTerms {
	// Maps for quick lookup.
	termMap := make(map[int]TaxonomyTerm)
	childrenMap := make(map[int][]TaxonomyTerm)

	for _, t := range ts {
		termMap[t.ID] = t
		if t.ParentID == 0 {
			childrenMap[0] = append(childrenMap[0], t)
		} else {
			childrenMap[t.ParentID] = append(childrenMap[t.ParentID], t)
		}
	}

	var result TaxonomyTerms
	var addTermWithChildren func(parentID int)
	addTermWithChildren = func(parentID int) {
		children := childrenMap[parentID]
		for _, child := range children {
			result = append(result, child)
			addTermWithChildren(child.ID)
		}
	}

	// Start with root terms (ParentID == 0)
	addTermWithChildren(0)

	return result
}

// TaxonomyRepository - storage port for the vocabularies, their terms and
// the nodes' terms.
type TaxonomyRepository interface {
	NodeTermsByID(ctx context.Context, nID int) (TaxonomyTerms, error)
	// NodeTermsByIDs is NodeTermsByID batched over many node IDs in one query.
	NodeTermsByIDs(ctx context.Context, nodeIDs []int) (map[int]TaxonomyTerms, error)
	TermBySlug(ctx context.Context, slug string) (*TaxonomyTerm, error)
	// CountNodesByTermID - the nodes the term is on, of any status: in the
	// index or in a taxonomy field's values (which keep the term from
	// being deleted).
	CountNodesByTermID(ctx context.Context, termID int) (int, error)
	// TermNodeCounts - how many published materials (not on moderation)
	// each term of the vocabulary is on (0 - every vocabulary), by term
	// ID; terms on none are left out.
	TermNodeCounts(ctx context.Context, vocabularyID int) (map[int]int, error)
	SearchTermsByName(ctx context.Context, vocabularyID int, query string, limit int) (TaxonomyTerms, error)
	TermsByVocabularyID(ctx context.Context, vocabularyID int) (TaxonomyTerms, error)
	CreateTerm(ctx context.Context, vocabularyID int, name, slug string, createdBy int) (*TaxonomyTerm, error)
	UpdateNodeTerms(ctx context.Context, nodeID int, termIDs []int) error
	// FieldTermIDsForNode - every term the node references through any
	// taxonomy field (field_value_taxonomy ∪ field_value_taxonomy_multi),
	// whatever the field or vocabulary - what taxonomy_index must cover.
	FieldTermIDsForNode(ctx context.Context, nodeID int) ([]int, error)

	// Admin CRUD for taxonomy vocabularies and terms (dictionary management screen).
	Vocabularies(ctx context.Context) (TaxonomyVocabularies, error)
	VocabularyByID(ctx context.Context, id int) (*TaxonomyVocabulary, error)
	CreateVocabulary(ctx context.Context, name, slug, termPathPrefix, desc string, publicTermPages bool, weight, createdBy int) (*TaxonomyVocabulary, error)
	UpdateVocabulary(ctx context.Context, id int, name, slug, termPathPrefix, desc string, publicTermPages bool, weight int) (*TaxonomyVocabulary, error)
	// UpdateVocabularySortOrder updates only Weight (drag-and-drop reorder).
	UpdateVocabularySortOrder(ctx context.Context, id int, weight int) error
	DeleteVocabulary(ctx context.Context, id int) error

	TermByID(ctx context.Context, id int) (*TaxonomyTerm, error)
	// TermsByIDs is TermByID batched over many term IDs in one query. Unknown
	// IDs are absent from the map, not an error.
	TermsByIDs(ctx context.Context, ids []int) (map[int]TaxonomyTerm, error)
	CreateTermFull(ctx context.Context, vocabularyID int, name, slug, desc string, weight, parentID, createdBy int) (*TaxonomyTerm, error)
	UpdateTerm(ctx context.Context, id int, name, slug, desc string, weight, parentID int) (*TaxonomyTerm, error)
	UpdateTermPageTitle(ctx context.Context, id int, pageTitle string) error
	// UpdateTermSortOrder updates only Weight/ParentID (drag-and-drop reorder).
	UpdateTermSortOrder(ctx context.Context, id int, weight, parentID int) error
	DeleteTerm(ctx context.Context, id int) error
}
