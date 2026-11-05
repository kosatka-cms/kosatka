package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/nodecounter"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/nodeviewday"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// NodeCounterRepository implements entity.NodeCounterRepository.
type NodeCounterRepository struct {
	client *generated.Client
}

// NewNodeCounterRepository creates a new NodeCounterRepository.
func NewNodeCounterRepository(db *sql.DB) *NodeCounterRepository {
	return &NodeCounterRepository{client: clientFor(db)}
}

// IncrementView counts a view of the node: its totals (incrementCounter)
// and its views of the site's day (node_view_days).
func (r *NodeCounterRepository) IncrementView(ctx context.Context, nid int) error {
	client := GetClient(ctx, r.client)
	if err := incrementCounter(ctx, client, nid); err != nil {
		return err
	}
	return incrementViewDay(ctx, client, nid, entity.ViewDay(time.Now()))
}

// DeleteViewDaysBefore drops the views by day older than day.
func (r *NodeCounterRepository) DeleteViewDaysBefore(ctx context.Context, day int) error {
	if _, err := GetClient(ctx, r.client).NodeViewDay.Delete().Where(nodeviewday.DayLT(day)).Exec(ctx); err != nil {
		return fmt.Errorf("deleting the views by day before %d: %w", day, err)
	}
	return nil
}

// incrementViewDay counts a view of the node on day (entity.ViewDay).
func incrementViewDay(ctx context.Context, client *generated.Client, nid, day int) error {
	for attempt := 0; ; attempt++ {
		n, err := client.NodeViewDay.Update().
			Where(nodeviewday.Nid(nid), nodeviewday.Day(day)).
			AddViews(1).
			Save(ctx)
		if err != nil || n > 0 {
			return err
		}
		// The day's first view of the node. A concurrent view may create
		// the row first: then count again on that row.
		err = client.NodeViewDay.Create().SetNid(nid).SetDay(day).SetViews(1).Exec(ctx)
		if !generated.IsConstraintError(err) || attempt > 0 {
			return err
		}
	}
}

// incrementCounter increments totalcount and daycount and sets
// last_viewed for the given node id; daycount starts over with the first
// view of the site's day (the day "viewed today" Views filter by).
func incrementCounter(ctx context.Context, client *generated.Client, nid int) error {
	today := TodayStartSite()
	for attempt := 0; ; attempt++ {
		now := time.Now()
		// Viewed earlier today: both counters go up.
		n, err := client.NodeCounter.Update().
			Where(nodecounter.Nid(nid), nodecounter.LastViewedGTE(today)).
			AddTotalcount(1).AddDaycount(1).SetLastViewed(now).
			Save(ctx)
		if err != nil || n > 0 {
			return err
		}
		// The first view today: the day count starts over.
		n, err = client.NodeCounter.Update().
			Where(nodecounter.Nid(nid), nodecounter.LastViewedLT(today)).
			AddTotalcount(1).SetDaycount(1).SetLastViewed(now).
			Save(ctx)
		if err != nil || n > 0 {
			return err
		}
		// No row yet. A concurrent view may create it first: then count
		// again on that row.
		err = client.NodeCounter.Create().SetNid(nid).SetTotalcount(1).SetDaycount(1).SetLastViewed(now).Exec(ctx)
		if !generated.IsConstraintError(err) || attempt > 0 {
			return err
		}
	}
}

// TodayStartSite returns midnight in the site's time zone (pkg/sitetime):
// where "viewed today" starts (node_counter.daycount, ViewSortBuiltinViewsToday).
func TodayStartSite() time.Time {
	loc := sitetime.Location()
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
}

// OrderByCounter orders nodes by a node_counter column (totalcount,
// daycount), 0 for a node without a counter row. Not ent's
// node.ByNodeCounterField: for an edge keyed on node_counter it joins a
// "GROUP BY nid" subquery selecting the column, which PostgreSQL rejects
// (the column is neither grouped nor aggregated).
func OrderByCounter(column string, desc bool) node.OrderOption {
	return func(s *entsql.Selector) {
		s.OrderExprFunc(func(b *entsql.Builder) {
			b.WriteString("COALESCE((SELECT ").Ident(column).
				WriteString(" FROM ").Ident(nodecounter.Table).
				WriteString(" WHERE ").Ident(nodecounter.Table).WriteByte('.').Ident(nodecounter.FieldNid).
				WriteString(" = ").WriteString(s.C(node.FieldID)).
				WriteString("), 0)")
			if desc {
				b.WriteString(" DESC")
			}
		})
	}
}

// ViewDaysSince - the first day (entity.ViewDay) of the last days of
// views, today included.
func ViewDaysSince(days int) int {
	return entity.ViewDay(time.Now().AddDate(0, 0, 1-days))
}

// OrderByViewDays orders nodes by their views from the day since on
// (node_view_days), 0 for a node not viewed then. since is written as a
// number, not an argument: ent drops an ORDER BY expression's arguments
// (MySQL: "expected 2 arguments, got 1"); it is an int, never text.
func OrderByViewDays(since int, desc bool) node.OrderOption {
	return func(s *entsql.Selector) {
		s.OrderExprFunc(func(b *entsql.Builder) {
			b.WriteString("COALESCE((SELECT SUM(").Ident(nodeviewday.FieldViews).
				WriteString(") FROM ").Ident(nodeviewday.Table).
				WriteString(" WHERE ").Ident(nodeviewday.Table).WriteByte('.').Ident(nodeviewday.FieldNid).
				WriteString(" = ").WriteString(s.C(node.FieldID)).
				WriteString(" AND ").Ident(nodeviewday.Table).WriteByte('.').Ident(nodeviewday.FieldDay).
				WriteString(" >= ").WriteString(strconv.Itoa(since)).
				WriteString("), 0)")
			if desc {
				b.WriteString(" DESC")
			}
		})
	}
}
