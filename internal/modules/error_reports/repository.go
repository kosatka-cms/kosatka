package errorreports

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/error_reports/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/error_reports/ent/generated/errorreport"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// EntErrorReportRepository implements ErrorReportRepository on the
// module's own ent schema (ent/); the reporters - core users - come from
// the core's user repository.
type EntErrorReportRepository struct {
	db    *sql.DB
	users modulekit.UserRepository
}

// NewErrorReportRepository creates a new EntErrorReportRepository on db,
// in the repositories' dialect (modulekit.DBDialect).
func NewErrorReportRepository(db *sql.DB, users modulekit.UserRepository) *EntErrorReportRepository {
	return &EntErrorReportRepository{db: db, users: users}
}

// client - the module's ent client: the context's transaction (InTx - a
// hook running in the core's change, say), else the database.
func (r *EntErrorReportRepository) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// Create inserts a new error report.
func (r *EntErrorReportRepository) Create(ctx context.Context, report ErrorReport) (*ErrorReport, error) {
	c := r.client(ctx).ErrorReport.Create().
		SetPageURL(report.PageURL).SetSelectedText(report.SelectedText).
		SetNillableComment(nilIfEmpty(report.Comment)).
		SetNillableClientIP(nilIfEmpty(report.ClientIP)).
		SetNillableUserAgent(nilIfEmpty(report.UserAgent)).
		SetCreatedAt(time.Now())
	if report.UserID != nil && *report.UserID > 0 {
		c.SetUserID(*report.UserID)
	}
	row, err := c.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating an error report: %w", err)
	}
	out := report
	out.ID = row.ID
	out.CreatedAt = row.CreatedAt
	return &out, nil
}

// ListForAdmin returns error reports newest first with total count: the
// new ones, or all with opts.All.
func (r *EntErrorReportRepository) ListForAdmin(ctx context.Context, opts ErrorReportListOpts) (ErrorReports, int, error) {
	query := func() *generated.ErrorReportQuery {
		q := r.client(ctx).ErrorReport.Query()
		if !opts.All {
			q.Where(errorreport.ResolvedAtIsNil())
		}
		return q
	}
	total, err := query().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting error reports: %w", err)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := max(opts.Offset, 0)
	rows, err := query().
		Order(generated.Desc(errorreport.FieldCreatedAt)).
		Limit(limit).Offset(offset).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("listing error reports: %w", err)
	}

	// The reporters' names, for the reports sent signed in.
	var userIDs []int
	for _, er := range rows {
		if er.UserID != nil {
			userIDs = append(userIDs, *er.UserID)
		}
	}
	users, err := r.users.ByIDs(ctx, userIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("loading the reporters: %w", err)
	}

	var list ErrorReports
	for _, row := range rows {
		er := ErrorReport{
			ID: row.ID, UserID: row.UserID, PageURL: row.PageURL, SelectedText: row.SelectedText,
			Comment: deref(row.Comment), ClientIP: deref(row.ClientIP), UserAgent: deref(row.UserAgent), CreatedAt: row.CreatedAt,
			ResolvedAt: row.ResolvedAt,
		}
		if row.UserID != nil {
			if u, ok := users[*row.UserID]; ok {
				er.Username, er.UserSlug = u.Username, u.Slug
			}
		}
		list = append(list, er)
	}
	return list, total, nil
}

// CountNew returns the number of reports not marked fixed.
func (r *EntErrorReportRepository) CountNew(ctx context.Context) (int, error) {
	n, err := r.client(ctx).ErrorReport.Query().Where(errorreport.ResolvedAtIsNil()).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting new error reports: %w", err)
	}
	return n, nil
}

// Resolve marks the report fixed now; an already fixed one is left as is.
func (r *EntErrorReportRepository) Resolve(ctx context.Context, id int) error {
	if _, err := r.client(ctx).ErrorReport.Update().
		Where(errorreport.ID(id), errorreport.ResolvedAtIsNil()).
		SetResolvedAt(time.Now()).Save(ctx); err != nil {
		return fmt.Errorf("resolving error report (id %v): %w", id, err)
	}
	return nil
}

// Reopen returns a fixed report to the new ones (its fixed time cleared).
func (r *EntErrorReportRepository) Reopen(ctx context.Context, id int) error {
	if _, err := r.client(ctx).ErrorReport.Update().
		Where(errorreport.ID(id), errorreport.ResolvedAtNotNil()).
		ClearResolvedAt().Save(ctx); err != nil {
		return fmt.Errorf("reopening error report (id %v): %w", id, err)
	}
	return nil
}

// nilIfEmpty - s for a nullable column (nil when empty).
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// deref - a nullable column's value ("" for NULL).
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
