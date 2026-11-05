package entity

import (
	"context"
	"strings"
)

// What of the content a request may not show: the content types closed
// to its visitor (ContentTypeDef.ViewRoles) and the items whose own view
// roles they lack (Node.ViewRoles). Every node query run with such a ctx
// leaves that content out, and every comment query the comments on it -
// the store's interceptors, so that no list, search, feed or count
// forgets it. Set per request for its visitor (the content access
// middleware) and for what is rendered for everyone (the cached blocks,
// feeds - as for a guest).

// ContentVisitor - whose eyes a ctx's content queries are for
// (WithContentVisitor): what of the content they may not view.
type ContentVisitor struct {
	// HiddenTypes - the content types they may not view, sorted.
	HiddenTypes []string
	// ItemRoles - some content item has view roles of its own
	// (Node.ViewRoles): the fields below decide which of them they view.
	ItemRoles bool
	// Roles - their roles (UserRoleNames), sorted: an item with view
	// roles is for them with one of these.
	Roles []string
	// UserID - the signed-in visitor (0 - a guest): their own items they
	// view whatever the items' view roles.
	UserID int
	// AllItems - they view every item of the types they may view,
	// whatever its view roles.
	AllItems bool
	// ItemTypes - the types whose every item they view (they may edit any
	// of them), sorted.
	ItemTypes []string
}

// RestrictsItems - whether an item's own view roles may hide it from
// them.
func (v ContentVisitor) RestrictsItems() bool { return v.ItemRoles && !v.AllItems }

// Restricts - whether anything is hidden from them.
func (v ContentVisitor) Restricts() bool { return len(v.HiddenTypes) > 0 || v.RestrictsItems() }

// Key - what they view, as a string: visitors with the same key view the
// same content, their own items aside (Shared).
func (v ContentVisitor) Key() string {
	key := strings.Join(v.HiddenTypes, ",")
	if v.RestrictsItems() {
		key += "|" + strings.Join(v.Roles, ",") + "|" + strings.Join(v.ItemTypes, ",")
	}
	return key
}

// Shared - v without their own items: what is kept for every visitor
// with their Key.
func (v ContentVisitor) Shared() ContentVisitor {
	v.UserID = 0
	return v
}

type contentVisitorKey struct{}

type allNodeTypesKey struct{}

// WithContentVisitor - ctx whose node and comment queries leave out what
// v may not view.
func WithContentVisitor(ctx context.Context, v ContentVisitor) context.Context {
	return context.WithValue(ctx, contentVisitorKey{}, v)
}

// WithAllNodeTypes - ctx whose queries see all the content whatever the
// request's visitor may view: the site's own lookups, never shown as
// they are (a path taken, a menu link's address).
func WithAllNodeTypes(ctx context.Context) context.Context {
	return context.WithValue(ctx, allNodeTypesKey{}, true)
}

// VisitorOf - ctx's visitor (WithContentVisitor), false for a ctx without
// one (a background job's: it sees everything, and gets what is shared);
// under WithAllNodeTypes one who is hidden nothing.
func VisitorOf(ctx context.Context) (ContentVisitor, bool) {
	v, ok := ctx.Value(contentVisitorKey{}).(ContentVisitor)
	if all, _ := ctx.Value(allNodeTypesKey{}).(bool); all {
		return ContentVisitor{}, ok
	}
	return v, ok
}

// HiddenNodeTypes - the types ctx's queries leave out (its visitor's;
// none under WithAllNodeTypes).
func HiddenNodeTypes(ctx context.Context) []string {
	v, _ := VisitorOf(ctx)
	return v.HiddenTypes
}

type trashedKey struct{}

// WithTrashed - ctx whose node queries keep the nodes in the trash
// (Node.DeletedAt): the trash's own page, restoring, deleting for good.
func WithTrashed(ctx context.Context) context.Context {
	return context.WithValue(ctx, trashedKey{}, true)
}

// TrashVisible - whether ctx's node queries keep the nodes in the trash
// (WithTrashed, or the site's own lookups - WithAllNodeTypes: a slug a
// node in the trash has stays taken).
func TrashVisible(ctx context.Context) bool {
	if t, _ := ctx.Value(trashedKey{}).(bool); t {
		return true
	}
	all, _ := ctx.Value(allNodeTypesKey{}).(bool)
	return all
}
