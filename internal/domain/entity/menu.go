package entity

import (
	"context"
	"time"
)

// MenuDef - a named menu container (e.g. main menu, footer), each with its
// own tree of MenuLinkDef. Any menu can be placed as a block in any region
// (BlockPlacementMenu).
type MenuDef struct {
	ID          int
	MachineName string
	Label       string
	Description string
	// InSitemap - the menu's links go into the sitemap.
	InSitemap bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MenuDefs - a list of menus.
type MenuDefs []MenuDef

// MenuLinkType - which of LinkURL/LinkNodeID/LinkTermID/LinkViewID the link uses.
type MenuLinkType string

// The menu links' targets (MenuLinkType): a URL, a node, a term, a View.
const (
	MenuLinkURL  MenuLinkType = "url"
	MenuLinkNode MenuLinkType = "node"
	MenuLinkTerm MenuLinkType = "term"
	MenuLinkView MenuLinkType = "view"
)

// MenuLinkDef - a menu item. ParentID forms a tree of arbitrary depth;
// nil means a top-level item.
type MenuLinkDef struct {
	ID       int
	MenuID   int
	ParentID *int
	Title    string
	LinkType MenuLinkType
	// LinkURL - used only for MenuLinkURL: an internal path (starting with "/")
	// or a full external URL, used as is.
	LinkURL string
	// LinkNodeID/LinkTermID/LinkViewID - the target ID for the matching LinkType.
	// The URL is resolved from the target's current state on each request
	// (service.DefaultMenuService.ResolveMenuLinkURL), so renaming the target
	// doesn't break the link; a deleted target resolves to ok=false, not an error.
	LinkNodeID *int
	LinkTermID *int
	LinkViewID *int
	// Weight - order among siblings, set by drag-and-drop.
	Weight int
	// Enabled - false hides the item and its subtree from public rendering
	// without deleting it.
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LinkNodeIDValue (and LinkTermIDValue, LinkViewIDValue) return the
// matching *int dereferenced (0 = unset), since templates can't compare an
// int with a *int (used by admin/menus/link_target_fields.html).
func (l MenuLinkDef) LinkNodeIDValue() int {
	if l.LinkNodeID == nil {
		return 0
	}
	return *l.LinkNodeID
}

// LinkTermIDValue - LinkTermID or 0 (see LinkNodeIDValue).
func (l MenuLinkDef) LinkTermIDValue() int {
	if l.LinkTermID == nil {
		return 0
	}
	return *l.LinkTermID
}

// LinkViewIDValue - LinkViewID or 0 (see LinkNodeIDValue).
func (l MenuLinkDef) LinkViewIDValue() int {
	if l.LinkViewID == nil {
		return 0
	}
	return *l.LinkViewID
}

// MenuLinkDefs - a list of menu links.
type MenuLinkDefs []MenuLinkDef

// MenuRepository - storage port for menus and their links.
type MenuRepository interface {
	Menus(ctx context.Context) (MenuDefs, error)
	MenuByID(ctx context.Context, id int) (*MenuDef, error)
	MenuByMachineName(ctx context.Context, machineName string) (*MenuDef, error)
	CreateMenu(ctx context.Context, m MenuDef) (*MenuDef, error)
	UpdateMenu(ctx context.Context, m MenuDef) (*MenuDef, error)
	// DeleteMenu deletes a menu with all its links (cascaded in application
	// code, not by FK).
	DeleteMenu(ctx context.Context, id int) error

	MenuLinksByMenuID(ctx context.Context, menuID int) (MenuLinkDefs, error)
	MenuLinkByID(ctx context.Context, id int) (*MenuLinkDef, error)
	CreateMenuLink(ctx context.Context, l MenuLinkDef) (*MenuLinkDef, error)
	UpdateMenuLink(ctx context.Context, l MenuLinkDef) (*MenuLinkDef, error)
	// DeleteMenuLink moves the deleted item's children up to its own level;
	// the subtree is not deleted.
	DeleteMenuLink(ctx context.Context, id int) error
	// ReorderMenuLinks bulk-updates Weight/ParentID after drag-and-drop; all
	// updates in one call belong to the same menu.
	ReorderMenuLinks(ctx context.Context, updates []MenuLinkOrderUpdate) error
}

// MenuLinkOrderUpdate - one row of a bulk reorder
// (see MenuRepository.ReorderMenuLinks).
type MenuLinkOrderUpdate struct {
	ID       int
	ParentID *int
	Weight   int
}
