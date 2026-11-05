package viewquery

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
)

// moduleFilterPredicate narrows predicates by the modules' filters: the
// nodes they already match whose module values pass (one lookup per
// filter), as an ID IN predicate. False - no node passes. A filter with
// nothing to apply (an empty value, an exposed one not asked) is skipped;
// no filter to apply - nil.
func moduleFilterPredicate(ctx context.Context, client *generated.Client, predicates []predicate.Node, filters []entity.ViewRunModuleFilter, queryParams map[string]string) (predicate.Node, bool, error) {
	type applied struct {
		f     entity.ViewRunModuleFilter
		value string
	}
	var apply []applied
	for _, f := range filters {
		switch f.Def.Operator {
		case entity.ViewFilterOperatorEmpty, entity.ViewFilterOperatorNotEmpty:
			apply = append(apply, applied{f: f})
		default:
			if v, ok := resolveViewFilterValue(f.Def, queryParams); ok {
				apply = append(apply, applied{f: f, value: v})
			}
		}
	}
	if len(apply) == 0 {
		return nil, true, nil
	}
	ids, err := client.Node.Query().Where(predicates...).IDs(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("listing the nodes for a module's filter: %w", err)
	}
	for _, a := range apply {
		if len(ids) == 0 {
			break
		}
		kept := ids[:0:0]
		if a.f.Filter.Choice() {
			choices, err := a.f.Filter.Choices(ctx, ids)
			if err != nil {
				return nil, false, fmt.Errorf("filtering by %s: %w", a.f.Filter.Key, err)
			}
			for _, id := range ids {
				v, has := choices[id]
				if entity.ViewModuleChoiceMatch(a.f.Def.Operator, a.value, v, has) {
					kept = append(kept, id)
				}
			}
			ids = kept
			continue
		}
		values, err := a.f.Filter.Values(ctx, ids)
		if err != nil {
			return nil, false, fmt.Errorf("filtering by %s: %w", a.f.Filter.Key, err)
		}
		for _, id := range ids {
			v, has := values[id]
			if entity.ViewModuleFilterMatch(a.f.Def.Operator, a.value, v, has) {
				kept = append(kept, id)
			}
		}
		ids = kept
	}
	if len(ids) == 0 {
		return nil, false, nil
	}
	return node.IDIn(ids...), true, nil
}

// withModuleFilters - predicates with the modules' filters' (false - no
// node passes them).
func withModuleFilters(ctx context.Context, client *generated.Client, predicates []predicate.Node, filters []entity.ViewRunModuleFilter, queryParams map[string]string) ([]predicate.Node, bool, error) {
	if len(filters) == 0 {
		return predicates, true, nil
	}
	p, ok, err := moduleFilterPredicate(ctx, client, predicates, filters, queryParams)
	if err != nil || !ok {
		return nil, false, err
	}
	if p != nil {
		predicates = append(predicates, p)
	}
	return predicates, true, nil
}
