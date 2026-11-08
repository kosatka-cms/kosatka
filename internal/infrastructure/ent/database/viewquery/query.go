package viewquery

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/nodecounter"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/nodeviewday"
)

// RunView implements entity.ViewQuery.RunView. Filters are
// AND-combined: built-in fields go straight into the Node WHERE, generic
// fields first resolve to a node ID set via their field_value_* table and
// are intersected. Results are then sorted and paginated.
func (e *Engine) RunView(ctx context.Context, params entity.ViewRunParams) (entity.Nodes, int, error) {
	client := database.GetClient(ctx, e.client)

	predicates, impossible, similar, err := buildFilterPredicates(ctx, client, params.Filters, params.FieldStorages, params.QueryParams, params.ContextCreatedByID, params.ContextNodeID, params.ContextTermID)
	if err != nil {
		return nil, 0, err
	}
	if impossible {
		return entity.Nodes{}, 0, nil
	}
	// A node never appears in its own View block when a "currently viewed
	// node" context is present. Not admin-configurable on purpose.
	if params.ContextNodeID > 0 {
		predicates = append(predicates, node.IDNEQ(params.ContextNodeID))
	}
	predicates = database.WithContentListFilter(ctx, predicates)
	predicates, ok, err := withModuleFilters(ctx, client, predicates, params.ModuleFilters, params.QueryParams)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return entity.Nodes{}, 0, nil
	}

	skip, offset, pageSize := pageWindow(params)

	// similar is set only by a similar_to_current_node contextual filter:
	// its ranking orders the nodes the View's filters allow, the View's own
	// sort criteria (none - node ID ascending) only break its ties. The
	// candidates are already bounded by the filter's IN predicate.
	if similar != nil {
		// By ID: the sort is stable, so the last ties keep this order.
		ids, err := client.Node.Query().Where(predicates...).Order(generated.Asc(node.FieldID)).IDs(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("running the view: %w", err)
		}
		if len(ids) == 0 {
			return entity.Nodes{}, 0, nil
		}
		tieBreaks, err := sortKeysForCriteria(ctx, client, database.NewContentTypeRepository(e.client), params.Sorts, ids)
		if err != nil {
			return nil, 0, err
		}
		return nodesPage(ctx, client, afterSkip(sortIDsByKeys(ids, append(similar.sortKeys(), tieBreaks...)), skip), offset-skip, pageSize)
	}

	// Empty sorts means "created_at desc". A single builtin criterion uses
	// the SQL ORDER BY + LIMIT/OFFSET fast path; multiple criteria or any
	// generic-field or module's criterion go through sortNodeIDsByCriteria.
	sorts := params.Sorts
	if len(sorts) == 0 {
		sorts = []entity.ViewRunSort{{Builtin: entity.ViewSortBuiltinCreatedAt, Direction: "desc"}}
	}

	// Several criteria on the node's own columns (sticky first, then the
	// newest…): one SQL ORDER BY too, not every matching ID sorted in Go.
	if orders, ok := nodeColumnOrders(sorts); ok && len(sorts) > 1 {
		total, err := client.Node.Query().Where(predicates...).Count(ctx)
		total = max(total-skip, 0)
		if err != nil || total == 0 {
			return entity.Nodes{}, total, err
		}
		res, err := client.Node.Query().
			Where(predicates...).
			Order(append(orders, idTieBreak(sorts[len(sorts)-1].Direction != "asc"))...).
			WithUsers().
			WithNodeCounter().
			Limit(pageSize).
			Offset(offset).
			All(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("running the view: %w", err)
		}
		return nodesFromGenerated(res), total, nil
	}

	// "Viewed today", alone or with other criteria, also filters:
	// node_counter.daycount isn't reset daily, so nodes with a stale count
	// from a previous day must be excluded, not just sorted last. Added
	// before Count() so the total matches.
	// The views of the last days likewise: the nodes viewed in them only.
	for _, s := range sorts {
		if s.FieldStorage == nil && s.Builtin == entity.ViewSortBuiltinViewsToday {
			predicates = append(predicates, node.HasNodeCounterWith(nodecounter.LastViewedGTE(database.TodayStartSite())))
			break
		}
	}
	for _, s := range sorts {
		if days := s.Builtin.ViewDays(); s.FieldStorage == nil && s.Module == nil && days > 0 {
			predicates = append(predicates, node.HasViewDaysWith(nodeviewday.DayGTE(database.ViewDaysSince(days))))
		}
	}

	if len(sorts) == 1 && sorts[0].FieldStorage == nil && sorts[0].Module == nil {
		total, err := client.Node.Query().Where(predicates...).Count(ctx)
		total = max(total-skip, 0)
		if err != nil || total == 0 {
			return entity.Nodes{}, total, err
		}

		desc := sorts[0].Direction != "asc"
		var order node.OrderOption
		switch {
		case sorts[0].Builtin == entity.ViewSortBuiltinViews:
			// Same node_counter.total_count sort as NodeRepository.ListForAdmin.
			order = database.OrderByCounter(nodecounter.FieldTotalcount, desc)
		case sorts[0].Builtin == entity.ViewSortBuiltinTitle:
			if desc {
				order = generated.Desc(node.FieldTitle)
			} else {
				order = generated.Asc(node.FieldTitle)
			}
		case sorts[0].Builtin == entity.ViewSortBuiltinUpdatedAt, sorts[0].Builtin == entity.ViewSortBuiltinSticky:
			order, _ = nodeColumnOrder(sorts[0])
		case sorts[0].Builtin == entity.ViewSortBuiltinViewsToday:
			// The "viewed today" scoping is the predicate added above; this only
			// sets the order.
			order = database.OrderByCounter(nodecounter.FieldDaycount, desc)
		case sorts[0].Builtin.ViewDays() > 0:
			order = database.OrderByViewDays(database.ViewDaysSince(sorts[0].Builtin.ViewDays()), desc)
		case sorts[0].Builtin == entity.ViewSortBuiltinRandom:
			// Direction is meaningless for a shuffle and is ignored.
			order = database.OrderRandom
		case desc:
			order = generated.Desc(node.FieldCreatedAt)
		default:
			order = generated.Asc(node.FieldCreatedAt)
		}
		// Ties by ID, in the same direction: rows with equal keys keep
		// their place, so no page repeats or skips one (a shuffle needs none).
		orders := []node.OrderOption{order}
		if sorts[0].Builtin != entity.ViewSortBuiltinRandom {
			orders = append(orders, idTieBreak(desc))
		}
		res, err := client.Node.Query().
			Where(predicates...).
			Order(orders...).
			WithUsers().
			WithNodeCounter().
			Limit(pageSize).
			Offset(offset).
			All(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("running the view: %w", err)
		}
		return nodesFromGenerated(res), total, nil
	}

	// Multi-criterion or generic-field sort: no single SQL ORDER BY can span
	// the field_value_* tables, so fetch every matching ID, sort in Go and
	// slice the page. Acceptable for curated views with bounded result sets.
	// Validating that a generic criterion is sortable is the service layer's job.
	// By ID: the sort below is stable, so ties keep this order on every page.
	allIDs, err := client.Node.Query().Where(predicates...).Order(generated.Asc(node.FieldID)).IDs(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("running the view: %w", err)
	}
	if len(allIDs) <= skip {
		return entity.Nodes{}, 0, nil
	}

	sortedIDs, err := sortNodeIDsByCriteria(ctx, client, database.NewContentTypeRepository(e.client), sorts, allIDs)
	if err != nil {
		return nil, 0, err
	}
	return nodesPage(ctx, client, afterSkip(sortedIDs, skip), offset-skip, pageSize)
}

// pageWindow - the run's skipped items (ViewRunParams.Skip), the offset
// of its page past them and the pages before it, and the page size
// (30 when unset).
func pageWindow(params entity.ViewRunParams) (skip, offset, pageSize int) {
	pageSize = params.PageSize
	if pageSize <= 0 {
		pageSize = 30
	}
	page := max(params.Page, 1)
	skip = max(params.Skip, 0)
	return skip, skip + (page-1)*pageSize, pageSize
}

// afterSkip - ids without the first skip of them.
func afterSkip(ids []int, skip int) []int {
	return ids[min(skip, len(ids)):]
}

// nodesPage loads the page of sortedIDs (every matching node, in the
// View's order) at offset, in that order, and returns it with the total.
func nodesPage(ctx context.Context, client *generated.Client, sortedIDs []int, offset, pageSize int) (entity.Nodes, int, error) {
	total := len(sortedIDs)
	if offset >= total {
		return entity.Nodes{}, total, nil
	}
	pageIDs := sortedIDs[offset:min(offset+pageSize, total)]

	res, err := client.Node.Query().
		Where(node.IDIn(pageIDs...)).
		WithUsers().
		WithNodeCounter().
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("running the view: %w", err)
	}

	// IDIn doesn't preserve order - reorder to match pageIDs.
	byID := make(map[int]*generated.Node, len(res))
	for _, n := range res {
		byID[n.ID] = n
	}
	ordered := make([]*generated.Node, 0, len(pageIDs))
	for _, id := range pageIDs {
		if n, ok := byID[id]; ok {
			ordered = append(ordered, n)
		}
	}
	return nodesFromGenerated(ordered), total, nil
}

func nodesFromGenerated(res []*generated.Node) entity.Nodes {
	nodes := make(entity.Nodes, 0, len(res))
	for _, n := range res {
		nodes = append(nodes, dao.ToNodeEntity(n))
	}
	return nodes
}

// nodeColumnOrder - the SQL order of a criterion on one of the node's own
// columns (created_at, updated_at, title, sticky); false for the others
// (counters, random, fields).
func nodeColumnOrder(s entity.ViewRunSort) (node.OrderOption, bool) {
	if s.FieldStorage != nil {
		return nil, false
	}
	var column string
	switch s.Builtin {
	case entity.ViewSortBuiltinCreatedAt:
		column = node.FieldCreatedAt
	case entity.ViewSortBuiltinUpdatedAt:
		column = node.FieldUpdatedAt
	case entity.ViewSortBuiltinTitle:
		column = node.FieldTitle
	case entity.ViewSortBuiltinSticky:
		column = node.FieldSticky
	default:
		return nil, false
	}
	if s.Direction == "asc" {
		return generated.Asc(column), true
	}
	return generated.Desc(column), true
}

// nodeColumnOrders - nodeColumnOrder of every criterion, false when one
// isn't on a node column.
func nodeColumnOrders(sorts []entity.ViewRunSort) ([]node.OrderOption, bool) {
	out := make([]node.OrderOption, 0, len(sorts))
	for _, s := range sorts {
		o, ok := nodeColumnOrder(s)
		if !ok {
			return nil, false
		}
		out = append(out, o)
	}
	return out, true
}

// idTieBreak - the last ORDER BY key of a View's page: the node ID in the
// last criterion's direction, so rows with equal keys keep their place
// and no page repeats or skips one.
func idTieBreak(desc bool) node.OrderOption {
	if desc {
		return generated.Desc(node.FieldID)
	}
	return generated.Asc(node.FieldID)
}
