package entity

import "context"

// SearchResult represents a single search result for live search autocomplete
type SearchResult struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`     // content type machine name, e.g. article
	URL       string `json:"url"`      // /articles/foo
	Title     string `json:"title"`    // the type's title field, else the node title
	Subtitle  string `json:"subtitle"` // the type's subtitle field (search settings)
	ImageURL  string `json:"image_url"`
	Badge     string `json:"badge"`      // terms of the badge vocabulary (search settings)
	TypeLabel string `json:"type_label"` // human label of the type
	// Snippet - the piece of the item's text with the query's words (the
	// results page's: SearchService.WithSnippets).
	Snippet SearchSnippet `json:"-"`
}

// SearchSnippet - a piece of a search result's text around the query's
// words: its parts, the words marked (Match); Before, After - the text
// goes on there.
type SearchSnippet struct {
	Parts         []SearchSnippetPart
	Before, After bool
}

// SearchSnippetPart - a piece of a snippet's text.
type SearchSnippetPart struct {
	Text  string
	Match bool
}

// SearchCategory represents a category link for empty search state
type SearchCategory struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// SearchTypeDef - how one node type takes part in the live search: which
// text fields are searched besides the title, how many results, and how a
// result is shown. A type with no row is searched with the defaults
// (DefaultSearchLimit, its title); one switched off (Enabled false) isn't.
type SearchTypeDef struct {
	NodeType string
	// Enabled - false: the type is left out of the search.
	Enabled bool
	Weight  int
	// Limit - results of this type from the field search.
	Limit int
	// Label - the result's type label.
	Label string
	// MatchFields - text field machine names searched besides the title.
	MatchFields []string
	// AllFields - all the type's text fields are searched besides the
	// title, new ones too (MatchFields is then only kept for the form).
	AllFields bool
	// TitleField - the result's title (empty value or field - the node
	// title); SubtitleField - its second line.
	TitleField    string
	SubtitleField string
	// ImageField - an image field for the picture (a node without a value
	// gets the field's default image; empty - none: the results get
	// SearchSettings.PlaceholderImage); ImageStyle - its image style.
	ImageField string
	ImageStyle string
	// BadgeVocabularyID - the vocabulary whose terms of the node form the
	// result's caption ("A / B / "); 0 - none.
	BadgeVocabularyID int
}

// DefaultSearchLimit - the results of a type from the field search, for a
// type searched with the defaults.
const DefaultSearchLimit = 5

// SearchTypeDefs - a list of the live search's types.
type SearchTypeDefs []SearchTypeDef

// SearchSettings - the live search's site-wide settings (single row).
type SearchSettings struct {
	// PlaceholderImage - the picture of a result without one.
	PlaceholderImage string
	// CategoriesMenu - the menu whose links the empty search shows.
	CategoriesMenu string
}

// SearchConfigRepository - storage port for the live search's settings:
// its types and the site-wide row.
type SearchConfigRepository interface {
	// SearchTypes - every type's row (switched on or off), by weight.
	SearchTypes(ctx context.Context) (SearchTypeDefs, error)
	SaveSearchType(ctx context.Context, t SearchTypeDef) error
	UpdateSearchTypeWeight(ctx context.Context, nodeType string, weight int) error
	SearchSettings(ctx context.Context) (*SearchSettings, error)
	SaveSearchSettings(ctx context.Context, s SearchSettings) error
}
