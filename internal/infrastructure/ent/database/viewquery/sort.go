package viewquery

import (
	"context"
	"fmt"
	mathrand "math/rand"
	"sort"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/nodecounter"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/nodeviewday"
)

// sortNodeIDsByCriteria orders nodeIDs by sorts in list order (sorts[0]
// is primary, later ones are tie-breakers, like a multi-column ORDER BY).
// Resolved in Go because generic field values live in separate
// field_value_* tables per FieldType and a module's values come from the
// module. A single builtin-only criterion never gets here - RunView sorts
// it in SQL.
func sortNodeIDsByCriteria(ctx context.Context, client *generated.Client, ctRepo *database.ContentTypeRepository, sorts []entity.ViewRunSort, nodeIDs []int) ([]int, error) {
	keys, err := sortKeysForCriteria(ctx, client, ctRepo, sorts, nodeIDs)
	if err != nil {
		return nil, err
	}
	return sortIDsByKeys(nodeIDs, keys), nil
}

// sortKeysForCriteria resolves sorts into sortIDsByKeys' keys for nodeIDs
// (one bulk lookup per criterion).
func sortKeysForCriteria(ctx context.Context, client *generated.Client, ctRepo *database.ContentTypeRepository, sorts []entity.ViewRunSort, nodeIDs []int) ([]sortKey, error) {
	keys := make([]sortKey, len(sorts))
	for i, s := range sorts {
		desc := s.Direction != "asc"
		if s.Module != nil {
			v, err := s.Module.Values(ctx, nodeIDs, s.Params)
			if err != nil {
				return nil, fmt.Errorf("sorting by %s: %w", s.Module.Key, err)
			}
			keys[i] = sortKey{num: v, isNum: true, desc: desc}
			continue
		}
		if s.FieldStorage != nil {
			switch s.FieldStorage.FieldType {
			case entity.FieldTypeText:
				v, err := ctRepo.FieldTextValuesForNodes(ctx, s.FieldStorage.ID, nodeIDs)
				if err != nil {
					return nil, err
				}
				keys[i] = sortKey{str: v, desc: desc}
			case entity.FieldTypeNumber:
				v, err := ctRepo.FieldNumberValuesForNodes(ctx, s.FieldStorage.ID, nodeIDs)
				if err != nil {
					return nil, err
				}
				keys[i] = sortKey{num: v, isNum: true, desc: desc}
			case entity.FieldTypeDate:
				v, err := ctRepo.FieldDateValuesForNodes(ctx, s.FieldStorage.ID, nodeIDs)
				if err != nil {
					return nil, err
				}
				keys[i] = sortKey{tim: v, isTime: true, desc: desc}
			default:
				return nil, fmt.Errorf("RunView: field type %q can't be used to sort a view", s.FieldStorage.FieldType)
			}
			continue
		}
		switch s.Builtin {
		case entity.ViewSortBuiltinViews:
			v, err := nodeViewCountsForIDs(ctx, client, nodeIDs)
			if err != nil {
				return nil, err
			}
			keys[i] = sortKey{num: v, isNum: true, desc: desc}
		case entity.ViewSortBuiltinViewsToday:
			v, err := nodeTodayViewCountsForIDs(ctx, client, nodeIDs)
			if err != nil {
				return nil, err
			}
			keys[i] = sortKey{num: v, isNum: true, desc: desc}
		case entity.ViewSortBuiltinViewsWeek, entity.ViewSortBuiltinViewsMonth:
			v, err := nodeViewDaysForIDs(ctx, client, nodeIDs, database.ViewDaysSince(s.Builtin.ViewDays()))
			if err != nil {
				return nil, err
			}
			keys[i] = sortKey{num: v, isNum: true, desc: desc}
		case entity.ViewSortBuiltinTitle:
			v, err := nodeTitlesForIDs(ctx, client, nodeIDs)
			if err != nil {
				return nil, err
			}
			keys[i] = sortKey{str: v, desc: desc}
		case entity.ViewSortBuiltinSticky:
			v, err := nodeStickyForIDs(ctx, client, nodeIDs)
			if err != nil {
				return nil, err
			}
			keys[i] = sortKey{num: v, isNum: true, desc: desc}
		case entity.ViewSortBuiltinUpdatedAt:
			v, err := nodeUpdatedAtForIDs(ctx, client, nodeIDs)
			if err != nil {
				return nil, err
			}
			keys[i] = sortKey{tim: v, isTime: true, desc: desc}
		case entity.ViewSortBuiltinRandom:
			// Defensive only: the UI never combines Random with other criteria and
			// the single-criterion fast path orders randomly in SQL. If it does get
			// here, assign random keys rather than falling through to created_at.
			v := make(map[int]float64, len(nodeIDs))
			for _, id := range nodeIDs {
				v[id] = mathrand.Float64()
			}
			keys[i] = sortKey{num: v, isNum: true, desc: desc}
		default:
			v, err := nodeCreatedAtForIDs(ctx, client, nodeIDs)
			if err != nil {
				return nil, err
			}
			keys[i] = sortKey{tim: v, isTime: true, desc: desc}
		}
	}
	return keys, nil
}

// sortKey holds one resolved sort criterion's per-node values for
// sortIDsByKeys. Exactly one of str/num/tim is populated, selected by
// isNum/isTime (neither set means str).
type sortKey struct {
	str    map[int]string
	num    map[int]float64
	tim    map[int]time.Time
	isNum  bool
	isTime bool
	desc   bool
}

// sortIDsByKeys stably reorders ids by keys: keys[0] is primary, later
// keys are compared only on ties, like a multi-column ORDER BY. The pure
// comparator core of sortNodeIDsByCriteria, testable without a DB.
func sortIDsByKeys(ids []int, keys []sortKey) []int {
	sorted := make([]int, len(ids))
	copy(sorted, ids)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		for _, k := range keys {
			switch {
			case k.isTime:
				ta, tb := k.tim[a], k.tim[b]
				if ta.Equal(tb) {
					continue
				}
				if k.desc {
					return ta.After(tb)
				}
				return ta.Before(tb)
			case k.isNum:
				na, nb := k.num[a], k.num[b]
				if na == nb {
					continue
				}
				if k.desc {
					return na > nb
				}
				return na < nb
			default:
				sa, sb := k.str[a], k.str[b]
				if sa == sb {
					continue
				}
				if k.desc {
					return sa > sb
				}
				return sa < sb
			}
		}
		return false
	})
	return sorted
}

// nodeTitlesForIDs/nodeCreatedAtForIDs/nodeViewCountsForIDs are bulk
// lookups (one query, returned as a map) for sortNodeIDsByCriteria's
// builtin criteria.
func nodeTitlesForIDs(ctx context.Context, client *generated.Client, ids []int) (map[int]string, error) {
	rows, err := client.Node.Query().Where(node.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodeTitlesForIDs: %w", err)
	}
	m := make(map[int]string, len(rows))
	for _, n := range rows {
		m[n.ID] = n.Title
	}
	return m, nil
}

// nodeStickyForIDs - Node.Sticky as 1/0, a numeric sort key.
func nodeStickyForIDs(ctx context.Context, client *generated.Client, ids []int) (map[int]float64, error) {
	rows, err := client.Node.Query().Where(node.IDIn(ids...)).Select(node.FieldID, node.FieldSticky).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodeStickyForIDs: %w", err)
	}
	m := make(map[int]float64, len(rows))
	for _, n := range rows {
		if n.Sticky {
			m[n.ID] = 1
		} else {
			m[n.ID] = 0
		}
	}
	return m, nil
}

func nodeUpdatedAtForIDs(ctx context.Context, client *generated.Client, ids []int) (map[int]time.Time, error) {
	rows, err := client.Node.Query().Where(node.IDIn(ids...)).Select(node.FieldID, node.FieldUpdatedAt).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodeUpdatedAtForIDs: %w", err)
	}
	m := make(map[int]time.Time, len(rows))
	for _, n := range rows {
		m[n.ID] = n.UpdatedAt
	}
	return m, nil
}

func nodeCreatedAtForIDs(ctx context.Context, client *generated.Client, ids []int) (map[int]time.Time, error) {
	rows, err := client.Node.Query().Where(node.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodeCreatedAtForIDs: %w", err)
	}
	m := make(map[int]time.Time, len(rows))
	for _, n := range rows {
		m[n.ID] = n.CreatedAt
	}
	return m, nil
}

func nodeViewCountsForIDs(ctx context.Context, client *generated.Client, ids []int) (map[int]float64, error) {
	rows, err := client.Node.Query().
		Where(node.IDIn(ids...)).
		WithNodeCounter(func(ncq *generated.NodeCounterQuery) {
			ncq.Select(nodecounter.FieldTotalcount)
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodeViewCountsForIDs: %w", err)
	}
	m := make(map[int]float64, len(rows))
	for _, n := range rows {
		if n.Edges.NodeCounter != nil {
			m[n.ID] = float64(n.Edges.NodeCounter.Totalcount)
		}
	}
	return m, nil
}

// nodeViewDaysForIDs - the views of the nodes from the day since on
// (node_view_days), none for a node not viewed then.
func nodeViewDaysForIDs(ctx context.Context, client *generated.Client, ids []int, since int) (map[int]float64, error) {
	rows, err := client.NodeViewDay.Query().
		Where(nodeviewday.NidIn(ids...), nodeviewday.DayGTE(since)).
		Select(nodeviewday.FieldNid, nodeviewday.FieldViews).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodeViewDaysForIDs: %w", err)
	}
	m := make(map[int]float64, len(rows))
	for _, d := range rows {
		m[d.Nid] += float64(d.Views)
	}
	return m, nil
}

// nodeTodayViewCountsForIDs - today's views (node_counter.daycount) of the
// nodes viewed today in the site's time zone; an older count is a previous
// day's, 0.
func nodeTodayViewCountsForIDs(ctx context.Context, client *generated.Client, ids []int) (map[int]float64, error) {
	rows, err := client.NodeCounter.Query().
		Where(nodecounter.NidIn(ids...), nodecounter.LastViewedGTE(database.TodayStartSite())).
		Select(nodecounter.FieldNid, nodecounter.FieldDaycount).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodeTodayViewCountsForIDs: %w", err)
	}
	m := make(map[int]float64, len(rows))
	for _, c := range rows {
		m[c.Nid] = float64(c.Daycount)
	}
	return m, nil
}
