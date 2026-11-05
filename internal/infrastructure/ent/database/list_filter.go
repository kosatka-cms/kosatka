package database

import (
	"context"
	"sync"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
)

// The content lists' own narrowing (SetContentListFilter): e.g. the
// languages module keeps a list to the page's language.
var (
	listFilterMu sync.RWMutex
	listFilter   func(ctx context.Context) func(*entsql.Selector)
)

// SetContentListFilter sets what narrows the content lists' nodes (every
// content View's, the search's) for ctx's request: fn returns the condition to add to the nodes' query
// (its Where on the nodes' selector), nil - none. nil fn - no narrowing.
func SetContentListFilter(fn func(ctx context.Context) func(*entsql.Selector)) {
	listFilterMu.Lock()
	defer listFilterMu.Unlock()
	listFilter = fn
}

// ContentListPredicate - the content lists' narrowing for ctx (nil -
// none).
func ContentListPredicate(ctx context.Context) predicate.Node {
	listFilterMu.RLock()
	fn := listFilter
	listFilterMu.RUnlock()
	if fn == nil {
		return nil
	}
	if p := fn(ctx); p != nil {
		return predicate.Node(p)
	}
	return nil
}

// WithContentListFilter - predicates with the content lists' narrowing
// for ctx.
func WithContentListFilter(ctx context.Context, predicates []predicate.Node) []predicate.Node {
	if p := ContentListPredicate(ctx); p != nil {
		return append(predicates, p)
	}
	return predicates
}
