package service

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// PermBypassContentAccess - viewing the content of every type, whoever
// its type lets view it (entity.ContentTypeDef.ViewRoles).
const PermBypassContentAccess = "bypass content access"

// contentAccessTTL - how long the types' view roles are kept in memory: a
// change on this server clears them at once (Invalidate), another server
// of the site picks it up within this.
const contentAccessTTL = 5 * time.Second

// ContentAccess decides whose eyes the content is for: a type with view
// roles (entity.ContentTypeDef.ViewRoles) is shown only to visitors with
// one of them, to those who may edit any of its content, and to those
// with PermBypassContentAccess; an item with view roles of its own
// (entity.Node.ViewRoles) the same, and to its author. Pages, lists,
// search, feeds and files ask it.
type ContentAccess struct {
	// Types - the content types (ContentTypeService.ContentTypes).
	Types func(ctx context.Context) (entity.ContentTypeDefs, error)
	// ItemRoles - whether some content item has view roles of its own
	// (NodesRepository.AnyWithViewRoles); nil - none has.
	ItemRoles func(ctx context.Context) (bool, error)
	// HasPermission - the permission check (AuthService.HasPermission).
	HasPermission func(ctx context.Context, user *entity.User, key string) bool

	mu         sync.Mutex
	restricted map[string][]string // type → its view roles, restricted types only
	types      []string            // every type's machine name
	itemRoles  bool
	loaded     time.Time
}

// Invalidate drops the cached view roles: a content type, or an item's
// view roles, changed.
func (a *ContentAccess) Invalidate() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.restricted = nil
}

// load - the types with view roles, by machine name, every type's name,
// and whether some item has view roles of its own.
func (a *ContentAccess) load(ctx context.Context) (restricted map[string][]string, types []string, itemRoles bool, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.restricted != nil && time.Since(a.loaded) < contentAccessTTL {
		return a.restricted, a.types, a.itemRoles, nil
	}
	defs, err := a.Types(ctx)
	if err != nil {
		return nil, nil, false, fmt.Errorf("loading the content types' view roles: %w", err)
	}
	if a.ItemRoles != nil {
		if itemRoles, err = a.ItemRoles(ctx); err != nil {
			return nil, nil, false, fmt.Errorf("checking for the content with view roles of its own: %w", err)
		}
	}
	restricted = make(map[string][]string)
	for _, d := range defs {
		types = append(types, d.MachineName)
		if len(d.ViewRoles) > 0 {
			restricted[d.MachineName] = d.ViewRoles
		}
	}
	a.restricted, a.types, a.itemRoles, a.loaded = restricted, types, itemRoles, time.Now()
	return restricted, types, itemRoles, nil
}

// HiddenTypes - the types whose content user (nil - a guest) may not
// view, sorted; none when every type is open to them. Lists leave these
// types out.
func (a *ContentAccess) HiddenTypes(ctx context.Context, user *entity.User) ([]string, error) {
	restricted, _, _, err := a.load(ctx)
	if err != nil || len(restricted) == 0 {
		return nil, err
	}
	if a.HasPermission(ctx, user, PermBypassContentAccess) {
		return nil, nil
	}
	roles := entity.UserRoleNames(user)
	var hidden []string
	for t, viewRoles := range restricted {
		if !(entity.ContentTypeDef{ViewRoles: viewRoles}).ViewableBy(roles) && !a.HasPermission(ctx, user, PermEditAny(t)) {
			hidden = append(hidden, t)
		}
	}
	slices.Sort(hidden)
	return hidden, nil
}

// CanViewType reports whether user may view the content of nodeType; an
// error loading the types denies (closed, not open).
func (a *ContentAccess) CanViewType(ctx context.Context, user *entity.User, nodeType string) (bool, error) {
	hidden, err := a.HiddenTypes(ctx, user)
	if err != nil {
		return false, err
	}
	return !slices.Contains(hidden, nodeType), nil
}

// Visitor - what of the content user (nil - a guest) may not view
// (entity.ContentVisitor), for their requests' queries
// (entity.WithContentVisitor).
func (a *ContentAccess) Visitor(ctx context.Context, user *entity.User) (entity.ContentVisitor, error) {
	hidden, err := a.HiddenTypes(ctx, user)
	if err != nil {
		return entity.ContentVisitor{}, err
	}
	v := entity.ContentVisitor{HiddenTypes: hidden}
	_, types, itemRoles, err := a.load(ctx)
	if err != nil || !itemRoles {
		return v, err
	}
	v.ItemRoles = true
	if a.HasPermission(ctx, user, PermBypassContentAccess) {
		v.AllItems = true
		return v, nil
	}
	v.Roles = slices.Sorted(slices.Values(entity.UserRoleNames(user)))
	if user != nil {
		v.UserID = user.ID
	}
	for _, t := range types {
		if a.HasPermission(ctx, user, PermEditAny(t)) {
			v.ItemTypes = append(v.ItemTypes, t)
		}
	}
	slices.Sort(v.ItemTypes)
	return v, nil
}
