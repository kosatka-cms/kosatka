package service

import (
	"context"
	"encoding/json"
	"html/template"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// homeCrumb - the first link of every trail.
func homeCrumb(ctx context.Context) entity.BreadcrumbItem {
	return entity.BreadcrumbItem{URL: "/", Label: i18n.T(ctx, "Home")}
}

// breadcrumbTermsLoader - terms of a vocabulary with their parents
// (implemented by TaxonomyService).
type breadcrumbTermsLoader interface {
	TermsByVocabularyID(ctx context.Context, vocabularyID int) (entity.TaxonomyTerms, error)
}

// breadcrumbFieldLoader - a node's taxonomy field values outside a
// fully loaded node page (implemented by ContentTypeService).
type breadcrumbFieldLoader interface {
	FieldStorageByID(ctx context.Context, id int) (*entity.FieldStorageDef, error)
	FieldTaxonomyValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]int, error)
	FieldTaxonomyMultiValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int][]int, error)
}

// BreadcrumbService builds breadcrumb trails from the site's menus: a page
// whose URL is a menu link gets that link's trail; a node page gets the
// trail of the menu link its breadcrumb rule points to.
type BreadcrumbService struct {
	// uow - a reorder in one transaction (SetUnitOfWork; nil - none).
	uow entity.UnitOfWork
	// baseURL - the site origin for absolute URLs in JSON-LD (SiteURL);
	// nil or empty - no JSON-LD.
	baseURL   func() string
	rules     entity.BreadcrumbRuleRepository
	menuTrees func() map[string][]MenuTreeItem
	terms     breadcrumbTermsLoader
	fields    breadcrumbFieldLoader
	logger    *slog.Logger

	// rulesMu/rulesCache - the rules, loaded once and reset by every
	// write below (rules are read on every node page).
	rulesMu    sync.Mutex
	rulesCache entity.BreadcrumbRuleDefs
}

// NewBreadcrumbService creates a BreadcrumbService; menuTrees returns
// the resolved menu trees (CacheService's SidebarData.MenuTrees).
func NewBreadcrumbService(baseURL func() string, rules entity.BreadcrumbRuleRepository, menuTrees func() map[string][]MenuTreeItem, terms breadcrumbTermsLoader, fields breadcrumbFieldLoader, logger *slog.Logger) *BreadcrumbService {
	return &BreadcrumbService{baseURL: baseURL, rules: rules, menuTrees: menuTrees, terms: terms, fields: fields, logger: logger.With("component", "breadcrumb.service")}
}

// JSONLD - the trail as schema.org BreadcrumbList JSON (for a
// <script type="application/ld+json">); empty without a base URL or a
// trail. The current page (last item, no URL) gets no "item".
func (s *BreadcrumbService) JSONLD(trail []entity.BreadcrumbItem) template.JS {
	var base string
	if s.baseURL != nil {
		base = NormalizeBaseURL(s.baseURL())
	}
	if base == "" || len(trail) == 0 {
		return ""
	}
	type listItem struct {
		Type     string `json:"@type"`
		Position int    `json:"position"`
		Name     string `json:"name"`
		Item     string `json:"item,omitempty"`
	}
	items := make([]listItem, 0, len(trail))
	for i, b := range trail {
		li := listItem{Type: "ListItem", Position: i + 1, Name: b.Label}
		if b.URL != "" {
			li.Item = base + b.URL
		}
		items = append(items, li)
	}
	out, err := json.Marshal(map[string]any{
		"@context":        "https://schema.org",
		"@type":           "BreadcrumbList",
		"itemListElement": items,
	})
	if err != nil {
		return ""
	}
	return template.JS(out)
}

// TrailForPath - the trail of the menu link whose URL is path (its last
// item is the current page, without a link); nil when no menu has it.
func (s *BreadcrumbService) TrailForPath(ctx context.Context, path string) []entity.BreadcrumbItem {
	chain := s.findChain(func(it MenuTreeItem) bool { return !it.Broken && it.URL == path })
	if chain == nil {
		return nil
	}
	trail := trailFromChain(ctx, chain)
	trail[len(trail)-1].URL = ""
	return trail
}

// TrailOrTitle - TrailForPath, or "Home › title" when path is in no menu.
func (s *BreadcrumbService) TrailOrTitle(ctx context.Context, path, title string) []entity.BreadcrumbItem {
	if trail := s.TrailForPath(ctx, path); trail != nil {
		return trail
	}
	return []entity.BreadcrumbItem{homeCrumb(ctx), {Label: title}}
}

// TrailForNode - the trail of a node page: the first breadcrumb rule (by
// weight) matching the node → its menu link's trail (none - home), then
// the node's title (current page). The rule fields' values come from dc
// (already loaded by the page).
func (s *BreadcrumbService) TrailForNode(ctx context.Context, dc entity.DynamicContent, title string) []entity.BreadcrumbItem {
	fieldTerms := func(storageID int) (int, []int) { return dcFieldTerms(dc, storageID) }
	return s.nodeTrail(ctx, string(dc.Node.Type), fieldTerms, title)
}

// NodeTrail - TrailForNode for a node page that has no fields loaded
// (e.g. a poll page): the rule fields' values are loaded on demand.
func (s *BreadcrumbService) NodeTrail(ctx context.Context, node entity.Node) []entity.BreadcrumbItem {
	fieldTerms := func(storageID int) (int, []int) {
		return s.loadNodeFieldTerms(ctx, node.ID, storageID)
	}
	return s.nodeTrail(ctx, string(node.Type), fieldTerms, node.Title)
}

// NodeSubpageTrail - the trail of a page under a node (e.g. a module's list,
// edit form, poll votes…): the node's own trail with the node linked,
// then extra, then title.
func (s *BreadcrumbService) NodeSubpageTrail(ctx context.Context, node entity.Node, title string, extra ...entity.BreadcrumbItem) []entity.BreadcrumbItem {
	trail := s.NodeTrail(ctx, node)
	trail[len(trail)-1].URL = node.URL()
	trail = append(trail, extra...)
	return append(trail, entity.BreadcrumbItem{Label: title})
}

// ChildTrail - the trail of a page with natural parents (a forum topic
// under its sections, a quote author under the quotes list…): the
// deepest parent whose URL is a menu link brings its menu chain, the
// parents after it follow, then title; with no parent in a menu - home,
// the parents, title. Same-URL neighbours collapse.
func (s *BreadcrumbService) ChildTrail(ctx context.Context, parents []entity.BreadcrumbItem, title string) []entity.BreadcrumbItem {
	trail := []entity.BreadcrumbItem{homeCrumb(ctx)}
	rest := parents
	for i := len(parents) - 1; i >= 0; i-- {
		url := parents[i].URL
		if url == "" {
			continue
		}
		if chain := s.findChain(func(it MenuTreeItem) bool { return !it.Broken && it.URL == url }); chain != nil {
			trail = trailFromChain(ctx, chain)
			rest = parents[i+1:]
			break
		}
	}
	for _, p := range rest {
		if p.URL != "" && p.URL == trail[len(trail)-1].URL {
			continue
		}
		trail = append(trail, p)
	}
	return append(trail, entity.BreadcrumbItem{Label: title})
}

// nodeTrail - the rule's menu chain + title. fieldTerms returns a rule
// field's vocabulary and the node's terms in it.
func (s *BreadcrumbService) nodeTrail(ctx context.Context, nodeType string, fieldTerms func(storageID int) (int, []int), title string) []entity.BreadcrumbItem {
	var chain []MenuTreeItem
	if linkID := s.matchingRuleLink(ctx, nodeType, fieldTerms); linkID > 0 {
		chain = s.findChain(func(it MenuTreeItem) bool { return it.ID == linkID })
	}
	trail := trailFromChain(ctx, chain)
	return append(trail, entity.BreadcrumbItem{Label: title})
}

// matchingRuleLink - the menu link ID of the first rule matching the
// node, 0 if none.
func (s *BreadcrumbService) matchingRuleLink(ctx context.Context, nodeType string, fieldTerms func(storageID int) (int, []int)) int {
	if s.rules == nil {
		return 0
	}
	rules, err := s.Rules(ctx)
	if err != nil {
		LogFallback(s.logger, err, "error loading breadcrumb rules")
		return 0
	}
	of := func(r entity.BreadcrumbRuleDef) (string, *int, func(int) bool) {
		return r.ContentType, r.FieldStorageID, r.HasTerm
	}
	if i := matchRule(ctx, rules, of, nodeType, fieldTerms, s.terms, s.logger); i >= 0 {
		return rules[i].MenuLinkID
	}
	return 0
}

// loadNodeFieldTerms - a taxonomy field's vocabulary and nodeID's terms
// in it (single- and multi-value storage both read).
func (s *BreadcrumbService) loadNodeFieldTerms(ctx context.Context, nodeID, storageID int) (int, []int) {
	if s.fields == nil {
		return 0, nil
	}
	storage, err := s.fields.FieldStorageByID(ctx, storageID)
	if err != nil || storage == nil {
		LogFallback(s.logger, err, "breadcrumbs: field storage", "fieldStorageID", storageID)
		return 0, nil
	}
	var terms []int
	single, err := s.fields.FieldTaxonomyValuesForNodes(ctx, storageID, []int{nodeID})
	LogFallback(s.logger, err, "breadcrumbs: field terms", "fieldStorageID", storageID, "nodeID", nodeID)
	if id, ok := single[nodeID]; ok {
		terms = append(terms, id)
	}
	multi, err := s.fields.FieldTaxonomyMultiValuesForNodes(ctx, storageID, []int{nodeID})
	LogFallback(s.logger, err, "breadcrumbs: multi-value field terms", "fieldStorageID", storageID, "nodeID", nodeID)
	terms = append(terms, multi[nodeID]...)
	return storage.VocabularyID(), terms
}

// findChain - the path from a menu root to the first item matching; when
// several menus have one, the deepest chain (the most specific
// hierarchy) wins, then the bigger menu (the site's main navigation
// rather than a footer), then the menu name.
func (s *BreadcrumbService) findChain(match func(MenuTreeItem) bool) []MenuTreeItem {
	if s.menuTrees == nil {
		return nil
	}
	trees := s.menuTrees()
	var best []MenuTreeItem
	bestSize := 0
	for _, name := range sortedMenuNames(trees) {
		chain := findInTree(trees[name], match, nil)
		if chain == nil {
			continue
		}
		size := countMenuItems(trees[name])
		if len(chain) > len(best) || (len(chain) == len(best) && size > bestSize) {
			best, bestSize = chain, size
		}
	}
	return best
}

func countMenuItems(items []MenuTreeItem) int {
	n := len(items)
	for _, it := range items {
		n += countMenuItems(it.Children)
	}
	return n
}

// sortedMenuNames - the menus' machine names, sorted.
func sortedMenuNames(trees map[string][]MenuTreeItem) []string {
	names := make([]string, 0, len(trees))
	for name := range trees {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func findInTree(items []MenuTreeItem, match func(MenuTreeItem) bool, path []MenuTreeItem) []MenuTreeItem {
	for _, it := range items {
		cur := append(append([]MenuTreeItem{}, path...), it)
		if match(it) {
			return cur
		}
		if found := findInTree(it.Children, match, cur); found != nil {
			return found
		}
	}
	return nil
}

// trailFromChain - home + the chain's links; a link with the same
// URL as the previous one (a section item and its own first child) is
// dropped.
func trailFromChain(ctx context.Context, chain []MenuTreeItem) []entity.BreadcrumbItem {
	trail := []entity.BreadcrumbItem{homeCrumb(ctx)}
	for _, it := range chain {
		if it.URL == trail[len(trail)-1].URL {
			continue
		}
		trail = append(trail, entity.BreadcrumbItem{URL: it.URL, Label: it.Title})
	}
	return trail
}

// Rules - all breadcrumb rules by weight (cached).
func (s *BreadcrumbService) Rules(ctx context.Context) (entity.BreadcrumbRuleDefs, error) {
	s.rulesMu.Lock()
	defer s.rulesMu.Unlock()
	if s.rulesCache != nil {
		return s.rulesCache, nil
	}
	rules, err := s.rules.BreadcrumbRules(ctx)
	if err != nil {
		return nil, err
	}
	if rules == nil {
		rules = entity.BreadcrumbRuleDefs{}
	}
	s.rulesCache = rules
	return rules, nil
}

func (s *BreadcrumbService) resetRules() {
	s.rulesMu.Lock()
	s.rulesCache = nil
	s.rulesMu.Unlock()
}

// RuleByID - the breadcrumb rule id.
func (s *BreadcrumbService) RuleByID(ctx context.Context, id int) (*entity.BreadcrumbRuleDef, error) {
	return s.rules.BreadcrumbRuleByID(ctx, id)
}

// CreateRule adds r at the end of the list (weight = max + 10).
func (s *BreadcrumbService) CreateRule(ctx context.Context, r entity.BreadcrumbRuleDef) (*entity.BreadcrumbRuleDef, error) {
	rules, err := s.Rules(ctx)
	if err != nil {
		return nil, err
	}
	r.Weight = lastWeight(rules, func(x entity.BreadcrumbRuleDef) int { return x.Weight })
	defer s.resetRules()
	return s.rules.CreateBreadcrumbRule(ctx, r)
}

// UpdateRule saves rule r (and drops the cached rules).
func (s *BreadcrumbService) UpdateRule(ctx context.Context, r entity.BreadcrumbRuleDef) (*entity.BreadcrumbRuleDef, error) {
	defer s.resetRules()
	return s.rules.UpdateBreadcrumbRule(ctx, r)
}

// ReorderRules sets weights 0, 10, 20… in the order of ids.
func (s *BreadcrumbService) ReorderRules(ctx context.Context, ids []int) error {
	defer s.resetRules()
	return reorder(ctx, s.uow, ids, s.rules.UpdateBreadcrumbRuleWeight, "the breadcrumb rules")
}

// SetUnitOfWork sets the transactions a reorder runs in.
func (s *BreadcrumbService) SetUnitOfWork(uow entity.UnitOfWork) { s.uow = uow }

// DeleteRule deletes rule id (and drops the cached rules).
func (s *BreadcrumbService) DeleteRule(ctx context.Context, id int) error {
	defer s.resetRules()
	return s.rules.DeleteBreadcrumbRule(ctx, id)
}

// BreadcrumbMenuLinkOption - a menu link for the rule form's select.
type BreadcrumbMenuLinkOption struct {
	ID    int
	Menu  string
	Title string
	URL   string
	Depth int
}

// Indent - the option label's nesting prefix.
func (o BreadcrumbMenuLinkOption) Indent() string { return strings.Repeat("- ", o.Depth) }

// MenuLinkOptions - every enabled link of every menu (menus by name),
// depth-first.
func (s *BreadcrumbService) MenuLinkOptions() []BreadcrumbMenuLinkOption {
	var out []BreadcrumbMenuLinkOption
	if s.menuTrees == nil {
		return out
	}
	trees := s.menuTrees()
	for _, name := range sortedMenuNames(trees) {
		var walk func(items []MenuTreeItem, depth int)
		walk = func(items []MenuTreeItem, depth int) {
			for _, it := range items {
				out = append(out, BreadcrumbMenuLinkOption{ID: it.ID, Menu: name, Title: it.Title, URL: it.URL, Depth: depth})
				walk(it.Children, depth+1)
			}
		}
		walk(trees[name], 0)
	}
	return out
}

// LinkTitlePath - the titles from the menu root down to link id, joined
// with " › "; empty when no enabled link has that id.
func (s *BreadcrumbService) LinkTitlePath(id int) string {
	chain := s.findChain(func(it MenuTreeItem) bool { return it.ID == id })
	titles := make([]string, 0, len(chain))
	for _, it := range chain {
		titles = append(titles, it.Title)
	}
	return strings.Join(titles, " › ")
}
