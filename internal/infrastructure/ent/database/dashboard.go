package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/comment"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/user"
)

// DashboardRepository - entity.DashboardRepository: counts over the
// nodes, comments, users and node_counter.
type DashboardRepository struct {
	db     *sql.DB
	client *generated.Client
}

// NewDashboardRepository creates a DashboardRepository.
func NewDashboardRepository(db *sql.DB) *DashboardRepository {
	return &DashboardRepository{db: db, client: clientFor(db)}
}

// Stats - see entity.DashboardRepository.Stats.
func (r *DashboardRepository) Stats(ctx context.Context, since, today time.Time) (entity.DashboardStats, error) {
	var st entity.DashboardStats
	client := GetClient(ctx, r.client)
	counts := []struct {
		dst  *int
		what string
		run  func() (int, error)
	}{
		{&st.Published, "published content", func() (int, error) { return client.Node.Query().Where(node.Status(1)).Count(ctx) }},
		{&st.PublishedSince, "recent content", func() (int, error) {
			return client.Node.Query().Where(node.Status(1), node.CreatedAtGTE(since)).Count(ctx)
		}},
		{&st.Comments, "comments", func() (int, error) { return client.Comment.Query().Count(ctx) }},
		{&st.CommentsSince, "recent comments", func() (int, error) {
			return client.Comment.Query().Where(comment.CreatedAtGTE(since)).Count(ctx)
		}},
		{&st.Users, "users", func() (int, error) { return client.User.Query().Count(ctx) }},
		{&st.UsersSince, "new users", func() (int, error) {
			return client.User.Query().Where(user.CreatedAtGTE(since)).Count(ctx)
		}},
	}
	for _, c := range counts {
		n, err := c.run()
		if err != nil {
			return st, fmt.Errorf("counting %s: %w", c.what, err)
		}
		*c.dst = n
	}
	// A node's daycount is of the day it was last viewed: today's only.
	err := r.db.QueryRowContext(ctx, rebind(`SELECT COALESCE(SUM(totalcount), 0),
		COALESCE(SUM(CASE WHEN last_viewed >= ? THEN daycount ELSE 0 END), 0) FROM node_counter`), today).
		Scan(&st.ViewsTotal, &st.ViewsToday)
	if err != nil {
		return st, fmt.Errorf("summing the views: %w", err)
	}
	return st, nil
}

// OnModeration - see entity.DashboardRepository.OnModeration.
func (r *DashboardRepository) OnModeration(ctx context.Context) (int, error) {
	n, err := GetClient(ctx, r.client).Node.Query().Where(node.OnModeration(1)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting the content on moderation: %w", err)
	}
	return n, nil
}

// Popular - entity.DashboardRepository.Popular: a day's and all views
// from node_counter, a week's and a month's summed from node_view_days.
func (r *DashboardRepository) Popular(ctx context.Context, q entity.PopularQuery) ([]entity.PopularNode, int, error) {
	from, where, args := popularSource(q)
	if q.Type != "" {
		where += " AND n.type = ?"
		args = append(args, q.Type)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, rebind("SELECT COUNT(*) "+from+" WHERE "+where), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting the popular content (%s): %w", q.Period, err)
	}
	if total == 0 {
		return nil, 0, nil
	}
	rows, err := r.db.QueryContext(ctx, rebind(`SELECT n.id, n.type, n.title, n.slug, n.langcode, p.views, p.total `+from+
		` WHERE `+where+` ORDER BY p.views DESC, n.id LIMIT ? OFFSET ?`), append(args, q.Limit, q.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing the popular content (%s): %w", q.Period, err)
	}
	defer func() { _ = rows.Close() }()
	var out []entity.PopularNode
	for rows.Next() {
		var p entity.PopularNode
		var slug sql.NullString
		var typ string
		if err := rows.Scan(&p.Node.ID, &typ, &p.Node.Title, &slug, &p.Node.Langcode, &p.Views, &p.Total); err != nil {
			return nil, 0, fmt.Errorf("reading the popular content: %w", err)
		}
		p.Node.Type, p.Node.Slug = entity.ContentType(typ), slug.String
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading the popular content: %w", err)
	}
	return out, total, nil
}

// popularSource - the FROM of q's period's views as p (p.views - in the
// period, p.total - all) joined to its published node n, the condition
// and its arguments.
func popularSource(q entity.PopularQuery) (from, where string, args []any) {
	if days := q.Period.Days(); days > 0 {
		since := entity.ViewDay(q.Today.AddDate(0, 0, 1-days))
		return `FROM (SELECT d.nid, SUM(d.views) AS views, COALESCE(MAX(c.totalcount), 0) AS total
			FROM node_view_days d LEFT JOIN node_counter c ON c.nid = d.nid
			WHERE d.day >= ? GROUP BY d.nid) p JOIN nodes n ON n.id = p.nid`,
			"n.status = 1", []any{since}
	}
	if q.Period == entity.PopularPeriodAll {
		return `FROM (SELECT nid, totalcount AS views, totalcount AS total FROM node_counter WHERE totalcount > 0) p
			JOIN nodes n ON n.id = p.nid`, "n.status = 1", nil
	}
	// A node's daycount is of the day it was last viewed: today's only.
	return `FROM (SELECT nid, daycount AS views, totalcount AS total FROM node_counter
		WHERE last_viewed >= ? AND daycount > 0) p JOIN nodes n ON n.id = p.nid`, "n.status = 1", []any{q.Today}
}
