package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// ErrMenuDuplicate - the machine_name is taken by another menu.
var ErrMenuDuplicate = errors.New("menu machine_name already taken")

// MenuService - the menus and their links, and their trees resolved for
// the pages (the links' URLs and titles).
type MenuService interface {
	Menus(ctx context.Context) (entity.MenuDefs, error)
	MenuByID(ctx context.Context, id int) (*entity.MenuDef, error)
	MenuByMachineName(ctx context.Context, machineName string) (*entity.MenuDef, error)
	CreateMenu(ctx context.Context, m entity.MenuDef) (*entity.MenuDef, error)
	UpdateMenu(ctx context.Context, m entity.MenuDef) (*entity.MenuDef, error)
	DeleteMenu(ctx context.Context, id int) error

	MenuLinksByMenuID(ctx context.Context, menuID int) (entity.MenuLinkDefs, error)
	MenuLinkByID(ctx context.Context, id int) (*entity.MenuLinkDef, error)
	CreateMenuLink(ctx context.Context, l entity.MenuLinkDef) (*entity.MenuLinkDef, error)
	UpdateMenuLink(ctx context.Context, l entity.MenuLinkDef) (*entity.MenuLinkDef, error)
	DeleteMenuLink(ctx context.Context, id int) error
	ReorderMenuLinks(ctx context.Context, updates []entity.MenuLinkOrderUpdate) error

	// ResolveMenuLinkURL - see DefaultMenuService.ResolveMenuLinkURL.
	ResolveMenuLinkURL(ctx context.Context, link entity.MenuLinkDef) (url string, ok bool)

	// ResolvedTree - see DefaultMenuService.ResolvedTree.
	ResolvedTree(ctx context.Context, machineName string) ([]MenuTreeItem, error)
}

// MenuTreeItem - one node of a resolved menu tree for public rendering
// ("menu_tree_items" in region_blocks.html). Title/URL are resolved via
// ResolveMenuLinkURL; First/Last/HasChildren come from the position.
// These fields don't depend on the request, so the whole tree is cached
// in CacheService. Active/ActiveTrail are per-request: they are set by
// AnnotateActiveMenuTree on a copy, never on the shared cached tree.
type MenuTreeItem struct {
	// ID - the menu link's own ID (entity.MenuLinkDef.ID), e.g. a
	// breadcrumb rule's target.
	ID    int
	Title string
	URL   string
	// NodeID - the content item a link to content leads to (0 - none):
	// on a page in another language, to its translation
	// (LocalizeMenuTree).
	NodeID      int
	Broken      bool
	First       bool
	Last        bool
	HasChildren bool
	Active      bool
	ActiveTrail bool
	Children    []MenuTreeItem
}

// AnnotateActiveMenuTree returns a COPY of tree with Active/ActiveTrail
// set for currentPath. The input usually comes from the shared cache read
// concurrently by all requests - editing it in place would be a data race
// and leak one visitor's current page into everyone's markup.
func AnnotateActiveMenuTree(tree []MenuTreeItem, currentPath string) []MenuTreeItem {
	if len(tree) == 0 {
		return tree
	}
	out := make([]MenuTreeItem, len(tree))
	for i, item := range tree {
		item.Children = AnnotateActiveMenuTree(item.Children, currentPath)
		item.Active = !item.Broken && item.URL == currentPath
		item.ActiveTrail = item.Active
		for _, child := range item.Children {
			if child.Active || child.ActiveTrail {
				item.ActiveTrail = true
			}
		}
		out[i] = item
	}
	return out
}

// LocalizeMenuTree returns a COPY of tree for a page in lang on a site of
// several languages (as is on a site of one): the links shown in lang
// (entity.VisibleInLanguage), their titles in it (TranslatedText), a
// link to content leading to its version in lang where it has a public
// one (NodeURLsInLanguage), another of the site's addresses under lang's
// prefix (entity.LocalizePath).
func LocalizeMenuTree(ctx context.Context, tree []MenuTreeItem, lang string) []MenuTreeItem {
	if _, others := entity.SiteLanguages(); len(others) == 0 || len(tree) == 0 {
		return tree
	}
	var ids []int
	collectMenuNodeIDs(tree, &ids)
	return localizeMenuItems(tree, lang, entity.NodeURLsInLanguage(ctx, ids, lang))
}

func collectMenuNodeIDs(tree []MenuTreeItem, ids *[]int) {
	for _, it := range tree {
		if it.NodeID > 0 {
			*ids = append(*ids, it.NodeID)
		}
		collectMenuNodeIDs(it.Children, ids)
	}
}

func localizeMenuItems(tree []MenuTreeItem, lang string, nodeURLs map[int]string) []MenuTreeItem {
	out := make([]MenuTreeItem, 0, len(tree))
	for _, it := range tree {
		ref := strconv.Itoa(it.ID)
		if !entity.VisibleInLanguage(entity.VisibilityMenuLink, ref, lang) {
			continue
		}
		it.Title = entity.TranslatedText(lang, entity.TextMenuLinkTitle, ref, it.Title)
		switch {
		case it.NodeID > 0:
			if u, ok := nodeURLs[it.NodeID]; ok {
				it.URL = u
			}
		case !it.Broken:
			it.URL = entity.LocalizePath(lang, it.URL)
		}
		it.Children = localizeMenuItems(it.Children, lang, nodeURLs)
		it.HasChildren = len(it.Children) > 0
		out = append(out, it)
	}
	for i := range out {
		out[i].First = i == 0
		out[i].Last = i == len(out)-1
	}
	return out
}

// ResolveMenuBlockRoot - the "initial visibility level" of a menu block:
// which level of an already annotated tree (AnnotateActiveMenuTree must
// run first) becomes the render root. startLevel 1 returns tree as is;
// each further level descends into the single ActiveTrail item. If the
// active trail doesn't reach that deep, it returns nil (an empty block),
// not an error. The depth cap (MenuLevelsToShow) is applied by the
// "menu_tree_items" template in region_blocks.html, not here.
func ResolveMenuBlockRoot(tree []MenuTreeItem, startLevel int) []MenuTreeItem {
	root := tree
	for step := 1; step < startLevel; step++ {
		var next []MenuTreeItem
		found := false
		for _, item := range root {
			if item.ActiveTrail {
				next = item.Children
				found = true
				break
			}
		}
		if !found {
			return nil
		}
		root = next
	}
	return root
}

// DefaultMenuService - the MenuService on the menus' repository; a link's
// node, term or View it resolves by their repositories.
type DefaultMenuService struct {
	repository   entity.MenuRepository
	nodeRepo     entity.NodesRepository
	taxonomyRepo entity.TaxonomyRepository
	viewRepo     entity.ViewRepository
	uow          entity.UnitOfWork
	logger       *slog.Logger
}

// NewMenuService - the menu service on r (the menus and links), nodeRepo,
// taxonomyRepo and viewRepo (what the links lead to), uow (the
// transactions) and baseLogger.
func NewMenuService(
	r entity.MenuRepository,
	nodeRepo entity.NodesRepository,
	taxonomyRepo entity.TaxonomyRepository,
	viewRepo entity.ViewRepository,
	uow entity.UnitOfWork,
	baseLogger *slog.Logger,
) MenuService {
	return &DefaultMenuService{
		repository:   r,
		nodeRepo:     nodeRepo,
		taxonomyRepo: taxonomyRepo,
		viewRepo:     viewRepo,
		uow:          uow,
		logger:       baseLogger.With("component", "menu.service"),
	}
}

// Menus - every menu.
func (s *DefaultMenuService) Menus(ctx context.Context) (entity.MenuDefs, error) {
	menus, err := s.repository.Menus(ctx)
	return entity.Wrapped(menus, err, "listing menus")
}

// MenuByID - the menu id.
func (s *DefaultMenuService) MenuByID(ctx context.Context, id int) (*entity.MenuDef, error) {
	m, err := s.repository.MenuByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// MenuByMachineName - the menu named machineName.
func (s *DefaultMenuService) MenuByMachineName(ctx context.Context, machineName string) (*entity.MenuDef, error) {
	m, err := s.repository.MenuByMachineName(ctx, machineName)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// CreateMenu saves the new menu m; a taken machine name -
// ErrMenuDuplicate.
func (s *DefaultMenuService) CreateMenu(ctx context.Context, m entity.MenuDef) (*entity.MenuDef, error) {
	created, err := s.repository.CreateMenu(ctx, m)
	if err != nil {
		if isDuplicateKeyErr(err) {
			return nil, fmt.Errorf("%w", ErrMenuDuplicate)
		}
		return nil, fmt.Errorf("creating menu (machineName %v): %w", m.MachineName, err)
	}
	return created, nil
}

// UpdateMenu saves menu m.
func (s *DefaultMenuService) UpdateMenu(ctx context.Context, m entity.MenuDef) (*entity.MenuDef, error) {
	updated, err := s.repository.UpdateMenu(ctx, m)
	return entity.Wrapped(updated, err, "updating menu (id %v)", m.ID)
}

// DeleteMenu deletes the menu's links, then the menu row, in one
// transaction.
func (s *DefaultMenuService) DeleteMenu(ctx context.Context, id int) error {
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		return s.repository.DeleteMenu(ctx, id)
	})
	if err != nil {
		return fmt.Errorf("deleting menu (id %v): %w", id, err)
	}
	return nil
}

// MenuLinksByMenuID - the links of the menu.
func (s *DefaultMenuService) MenuLinksByMenuID(ctx context.Context, menuID int) (entity.MenuLinkDefs, error) {
	links, err := s.repository.MenuLinksByMenuID(ctx, menuID)
	return entity.Wrapped(links, err, "listing menu links (menuID %v)", menuID)
}

// MenuLinkByID - the menu link id.
func (s *DefaultMenuService) MenuLinkByID(ctx context.Context, id int) (*entity.MenuLinkDef, error) {
	l, err := s.repository.MenuLinkByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// CreateMenuLink saves the new menu link l.
func (s *DefaultMenuService) CreateMenuLink(ctx context.Context, l entity.MenuLinkDef) (*entity.MenuLinkDef, error) {
	created, err := s.repository.CreateMenuLink(ctx, l)
	return entity.Wrapped(created, err, "creating menu link (menuID %v)", l.MenuID)
}

// UpdateMenuLink saves menu link l.
func (s *DefaultMenuService) UpdateMenuLink(ctx context.Context, l entity.MenuLinkDef) (*entity.MenuLinkDef, error) {
	updated, err := s.repository.UpdateMenuLink(ctx, l)
	return entity.Wrapped(updated, err, "updating menu link (id %v)", l.ID)
}

// DeleteMenuLink reparents the link's children, then removes it, in one
// transaction.
func (s *DefaultMenuService) DeleteMenuLink(ctx context.Context, id int) error {
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		return s.repository.DeleteMenuLink(ctx, id)
	})
	if err != nil {
		return fmt.Errorf("deleting menu link (id %v): %w", id, err)
	}
	return nil
}

// ReorderMenuLinks applies the admin drag-and-drop order in one
// transaction.
func (s *DefaultMenuService) ReorderMenuLinks(ctx context.Context, updates []entity.MenuLinkOrderUpdate) error {
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		return s.repository.ReorderMenuLinks(ctx, updates)
	})
	if err != nil {
		return fmt.Errorf("reordering menu links: %w", err)
	}
	return nil
}

// ResolveMenuLinkURL resolves a link to its CURRENT public URL (never a
// stored path), so slug, term or View path changes are picked up.
// ok=false means a broken link (empty URL, or the target node/term/view
// is gone) - the caller decides how to show it; it is not an error.
func (s *DefaultMenuService) ResolveMenuLinkURL(ctx context.Context, link entity.MenuLinkDef) (string, bool) {
	switch link.LinkType {
	case entity.MenuLinkURL:
		if link.LinkURL == "" {
			return "", false
		}
		return link.LinkURL, true

	case entity.MenuLinkNode:
		if link.LinkNodeID == nil {
			return "", false
		}
		node, err := s.nodeRepo.ByID(ctx, *link.LinkNodeID)
		if err != nil || node == nil || node.Slug == "" {
			return "", false
		}
		return node.URL(), true

	case entity.MenuLinkTerm:
		if link.LinkTermID == nil {
			return "", false
		}
		term, err := s.taxonomyRepo.TermByID(ctx, *link.LinkTermID)
		if err != nil || term == nil || term.Slug == "" {
			return "", false
		}
		return "/" + term.Slug, true

	case entity.MenuLinkView:
		if link.LinkViewID == nil {
			return "", false
		}
		view, err := s.viewRepo.ViewByID(ctx, *link.LinkViewID)
		if err != nil || view == nil || view.URLPath == "" {
			return "", false
		}
		return "/" + view.URLPath, true

	default:
		return "", false
	}
}

// ResolvedTree builds machineName's nested MenuTreeItem tree
// (weight-sorted per level, URLs via ResolveMenuLinkURL). A disabled link
// is dropped with its whole subtree. A missing menu is
// ErrEntityNotFound.
func (s *DefaultMenuService) ResolvedTree(ctx context.Context, machineName string) ([]MenuTreeItem, error) {
	menu, err := s.repository.MenuByMachineName(ctx, machineName)
	if err != nil {
		return nil, err
	}
	links, err := s.repository.MenuLinksByMenuID(ctx, menu.ID)
	if err != nil {
		return nil, err
	}
	return s.buildMenuTree(links, s.resolveURLs(ctx, links), nil), nil
}

// resolveURLs - each enabled link's current URL (ResolveMenuLinkURL),
// the nodes' and the terms' in one query each; a link left out is
// broken. A failed batch leaves its links broken and is logged. The
// administrator's links keep their addresses whoever views the menu
// (entity.WithAllNodeTypes): the page itself checks its access.
func (s *DefaultMenuService) resolveURLs(ctx context.Context, links entity.MenuLinkDefs) map[int]string {
	ctx = entity.WithAllNodeTypes(ctx)
	var nodeIDs, termIDs []int
	for _, l := range links {
		switch {
		case !l.Enabled:
		case l.LinkType == entity.MenuLinkNode && l.LinkNodeID != nil:
			nodeIDs = append(nodeIDs, *l.LinkNodeID)
		case l.LinkType == entity.MenuLinkTerm && l.LinkTermID != nil:
			termIDs = append(termIDs, *l.LinkTermID)
		}
	}
	nodeURLs := map[int]string{}
	if len(nodeIDs) > 0 {
		var err error
		if nodeURLs, err = s.nodeRepo.URLsByIDs(ctx, nodeIDs); err != nil {
			LogFallback(s.logger, err, "loading the menu's nodes (their links shown broken)")
		}
	}
	terms := map[int]entity.TaxonomyTerm{}
	if len(termIDs) > 0 {
		var err error
		if terms, err = s.taxonomyRepo.TermsByIDs(ctx, termIDs); err != nil {
			LogFallback(s.logger, err, "loading the menu's terms (their links shown broken)")
		}
	}
	out := make(map[int]string, len(links))
	for _, l := range links {
		if !l.Enabled {
			continue
		}
		switch l.LinkType {
		case entity.MenuLinkNode:
			if l.LinkNodeID != nil && nodeURLs[*l.LinkNodeID] != "" {
				out[l.ID] = nodeURLs[*l.LinkNodeID]
			}
		case entity.MenuLinkTerm:
			if l.LinkTermID != nil && terms[*l.LinkTermID].Slug != "" {
				out[l.ID] = "/" + terms[*l.LinkTermID].Slug
			}
		default:
			if url, ok := s.ResolveMenuLinkURL(ctx, l); ok {
				out[l.ID] = url
			}
		}
	}
	return out
}

// buildMenuTree recursively builds parentID's weight-sorted children from
// one menu's flat links; urls - their resolved URLs (resolveURLs), a link
// without one broken.
func (s *DefaultMenuService) buildMenuTree(links entity.MenuLinkDefs, urls map[int]string, parentID *int) []MenuTreeItem {
	var siblings entity.MenuLinkDefs
	for _, l := range links {
		if samePtrInt(l.ParentID, parentID) {
			siblings = append(siblings, l)
		}
	}
	sort.SliceStable(siblings, func(i, j int) bool { return siblings[i].Weight < siblings[j].Weight })

	items := make([]MenuTreeItem, 0, len(siblings))
	for _, l := range siblings {
		if !l.Enabled {
			continue
		}
		url, ok := urls[l.ID]
		children := s.buildMenuTree(links, urls, &l.ID)
		nodeID := 0
		if l.LinkType == entity.MenuLinkNode && l.LinkNodeID != nil {
			nodeID = *l.LinkNodeID
		}
		items = append(items, MenuTreeItem{
			ID:          l.ID,
			Title:       l.Title,
			URL:         url,
			NodeID:      nodeID,
			Broken:      !ok,
			HasChildren: len(children) > 0,
			Children:    children,
		})
	}
	for i := range items {
		items[i].First = i == 0
		items[i].Last = i == len(items)-1
	}
	return items
}

// samePtrInt compares two *int by value: both nil, or both non-nil
// with the same underlying int.
func samePtrInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
