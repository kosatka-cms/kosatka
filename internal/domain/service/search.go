package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// SearchRepository defines the storage for live search.
type SearchRepository interface {
	Search(ctx context.Context, query string, limit int, types entity.SearchTypeDefs, settings entity.SearchSettings) ([]entity.SearchResult, error)
	// Snippets - per result's ID its snippet: the piece of its match
	// fields' text with the query's words (none - absent).
	Snippets(ctx context.Context, query string, results []entity.SearchResult, types entity.SearchTypeDefs) (map[int]entity.SearchSnippet, error)
}

// SearchService provides the live search; which node types are searched
// and how is configured on /admin/config/search.
type SearchService struct {
	// uow - a reorder in one transaction (SetUnitOfWork; nil - none).
	uow       entity.UnitOfWork
	repo      SearchRepository
	config    entity.SearchConfigRepository
	menuTrees func() map[string][]MenuTreeItem
	logger    *slog.Logger
	// defaultTypes - the node types searched without a row of their own,
	// with their labels (SetDefaultTypes; nil - none).
	defaultTypes func(ctx context.Context) (map[string]string, error)

	// mu/cacheTypes/cacheSettings - the configuration, loaded once and
	// reset by every write below.
	mu            sync.Mutex
	cacheTypes    entity.SearchTypeDefs
	cacheSettings *entity.SearchSettings
}

// NewSearchService creates a new SearchService; menuTrees returns the
// resolved menu trees (CacheService's SidebarData.MenuTrees).
func NewSearchService(repo SearchRepository, config entity.SearchConfigRepository, menuTrees func() map[string][]MenuTreeItem, logger *slog.Logger) *SearchService {
	return &SearchService{
		repo:      repo,
		config:    config,
		menuTrees: menuTrees,
		logger:    logger.With("component", "search.service"),
	}
}

// SetDefaultTypes gives the node types searched while they have no row of
// their own (every content type with a page, the modules' types), by
// machine name with their labels: a new type is searched at once.
func (s *SearchService) SetDefaultTypes(types func(ctx context.Context) (map[string]string, error)) {
	s.defaultTypes = types
}

// rows - the types' rows, switched on or off, by weight (cached).
func (s *SearchService) rows(ctx context.Context) (entity.SearchTypeDefs, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheTypes != nil {
		return s.cacheTypes, nil
	}
	types, err := s.config.SearchTypes(ctx)
	if err != nil {
		return nil, err
	}
	s.cacheTypes = types
	return types, nil
}

// Types - the searched node types: the rows switched on, by weight, then
// the default types (SetDefaultTypes) with no row, by label, with the
// defaults - their title and all their text fields searched,
// DefaultSearchLimit results.
func (s *SearchService) Types(ctx context.Context) (entity.SearchTypeDefs, error) {
	rows, err := s.rows(ctx)
	if err != nil {
		return nil, err
	}
	out := make(entity.SearchTypeDefs, 0, len(rows))
	hasRow := make(map[string]bool, len(rows))
	for _, t := range rows {
		hasRow[t.NodeType] = true
		if t.Enabled {
			out = append(out, t)
		}
	}
	if s.defaultTypes == nil {
		return out, nil
	}
	labels, err := s.defaultTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("the types searched by default: %w", err)
	}
	var more entity.SearchTypeDefs
	for nodeType, label := range labels {
		if !hasRow[nodeType] {
			more = append(more, entity.SearchTypeDef{NodeType: nodeType, Enabled: true, Limit: entity.DefaultSearchLimit, AllFields: true, Label: label})
		}
	}
	sort.Slice(more, func(i, j int) bool {
		if more[i].Label != more[j].Label {
			return more[i].Label < more[j].Label
		}
		return more[i].NodeType < more[j].NodeType
	})
	weight := lastWeight(out, func(t entity.SearchTypeDef) int { return t.Weight })
	for i := range more {
		more[i].Weight = weight + 10*i
	}
	return append(out, more...), nil
}

// Settings - the site-wide search settings (cached).
func (s *SearchService) Settings(ctx context.Context) (entity.SearchSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheSettings != nil {
		return *s.cacheSettings, nil
	}
	settings, err := s.config.SearchSettings(ctx)
	if err != nil {
		return entity.SearchSettings{}, err
	}
	s.cacheSettings = settings
	return *settings, nil
}

func (s *SearchService) reset() {
	s.mu.Lock()
	s.cacheTypes, s.cacheSettings = nil, nil
	s.mu.Unlock()
}

// SaveType adds or updates a searched type (a new one goes last; one
// added back keeps its settings from before its removal).
func (s *SearchService) SaveType(ctx context.Context, t entity.SearchTypeDef, isNew bool) error {
	t.Enabled = true
	if isNew {
		rows, err := s.rows(ctx)
		if err != nil {
			return err
		}
		types, err := s.Types(ctx)
		if err != nil {
			return err
		}
		weight := lastWeight(types, func(x entity.SearchTypeDef) int { return x.Weight })
		// A type added back gets its settings from before its removal.
		for _, r := range rows {
			if r.NodeType == t.NodeType {
				t = r
				t.Enabled = true
			}
		}
		t.Weight = weight
	}
	defer s.reset()
	return s.config.SaveSearchType(ctx, t)
}

// ReorderTypes sets weights 0, 10, 20… in the order of nodeTypes; a type
// searched by default gets its row first.
func (s *SearchService) ReorderTypes(ctx context.Context, nodeTypes []string) error {
	defer s.reset()
	rows, err := s.rows(ctx)
	if err != nil {
		return err
	}
	types, err := s.Types(ctx)
	if err != nil {
		return err
	}
	hasRow := make(map[string]bool, len(rows))
	for _, t := range rows {
		hasRow[t.NodeType] = true
	}
	for _, t := range types {
		if !hasRow[t.NodeType] && slices.Contains(nodeTypes, t.NodeType) {
			if err := s.config.SaveSearchType(ctx, t); err != nil {
				return fmt.Errorf("saving the search type %q: %w", t.NodeType, err)
			}
		}
	}
	return reorder(ctx, s.uow, nodeTypes, s.config.UpdateSearchTypeWeight, "the searched types")
}

// SetUnitOfWork sets the transactions a reorder runs in.
func (s *SearchService) SetUnitOfWork(uow entity.UnitOfWork) { s.uow = uow }

// RemoveType leaves the node type out of the search: its row, switched
// off (a type with no row is searched by default), keeps its settings.
func (s *SearchService) RemoveType(ctx context.Context, nodeType string) error {
	defer s.reset()
	rows, err := s.rows(ctx)
	if err != nil {
		return err
	}
	t := entity.SearchTypeDef{NodeType: nodeType, Limit: entity.DefaultSearchLimit, AllFields: true}
	for _, r := range rows {
		if r.NodeType == nodeType {
			t = r
		}
	}
	t.Enabled = false
	return s.config.SaveSearchType(ctx, t)
}

// SaveSettings saves the site-wide search settings.
func (s *SearchService) SaveSettings(ctx context.Context, settings entity.SearchSettings) error {
	defer s.reset()
	return s.config.SaveSearchSettings(ctx, settings)
}

// Search performs the live search over the configured types.
func (s *SearchService) Search(ctx context.Context, query string, limit int) ([]entity.SearchResult, error) {
	if limit <= 0 {
		limit = 50
	}
	types, err := s.Types(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	types = slices.DeleteFunc(slices.Clone(types), func(t entity.SearchTypeDef) bool { return !NodeTypeEnabled(t.NodeType) })
	return s.repo.Search(ctx, query, limit, types, settings)
}

// SearchAll - the results page's search: up to max results, the types'
// own limits (made for the live search's short list) raised to limit; the
// page cuts it into pages.
func (s *SearchService) SearchAll(ctx context.Context, query string, limit int) ([]entity.SearchResult, error) {
	types, err := s.Types(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	types = slices.DeleteFunc(slices.Clone(types), func(t entity.SearchTypeDef) bool { return !NodeTypeEnabled(t.NodeType) })
	for i := range types {
		// A type off for field matches (limit 0) stays off.
		if types[i].Limit > 0 && types[i].Limit < limit {
			types[i].Limit = limit
		}
	}
	return s.repo.Search(ctx, query, limit, types, settings)
}

// WithSnippets - results with their snippets (SearchSnippet): for the
// results a page shows, not all it found.
func (s *SearchService) WithSnippets(ctx context.Context, query string, results []entity.SearchResult) ([]entity.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}
	types, err := s.Types(ctx)
	if err != nil {
		return nil, err
	}
	snippets, err := s.repo.Snippets(ctx, query, results, types)
	if err != nil {
		return nil, err
	}
	out := slices.Clone(results)
	for i := range out {
		out[i].Snippet = snippets[out[i].ID]
	}
	return out, nil
}

// GetCategories - the empty search's links: the top-level links of the
// configured menu (SearchSettings.CategoriesMenu).
func (s *SearchService) GetCategories(ctx context.Context) []entity.SearchCategory {
	out := []entity.SearchCategory{}
	settings, err := s.Settings(ctx)
	if err != nil {
		LogFallback(s.logger, err, "error loading search settings")
		return out
	}
	if settings.CategoriesMenu == "" || s.menuTrees == nil {
		return out
	}
	for _, it := range s.menuTrees()[settings.CategoriesMenu] {
		if it.Broken {
			continue
		}
		out = append(out, entity.SearchCategory{Label: it.Title, URL: it.URL})
	}
	return out
}
