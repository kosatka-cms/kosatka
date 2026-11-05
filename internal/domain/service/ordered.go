package service

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// The weighted lists the admin orders by dragging (SEO rules, breadcrumb
// rules, searched types): a new item goes last, a reorder numbers them
// 0, 10, 20…

// lastWeight - the weight of an item added after items (the largest
// weight + 10; 0 for none).
func lastWeight[T any](items []T, weight func(T) int) int {
	w := 0
	for _, it := range items {
		if weight(it) >= w {
			w = weight(it) + 10
		}
	}
	return w
}

// reorder gives keys the weights 0, 10, 20… in their order (setWeight),
// in one transaction: a failure halfway leaves the old order; what names
// the list in the error.
func reorder[K any](ctx context.Context, uow entity.UnitOfWork, keys []K, setWeight func(ctx context.Context, key K, weight int) error, what string) error {
	return inUnitOfWork(ctx, uow, func(ctx context.Context) error {
		for i, k := range keys {
			if err := setWeight(ctx, k, i*10); err != nil {
				return fmt.Errorf("reordering %s (%v): %w", what, k, err)
			}
		}
		return nil
	})
}
