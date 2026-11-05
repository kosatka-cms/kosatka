package errorreports

import (
	"context"
	"time"
)

// ErrorReport is a user-submitted report about incorrect text on a page.
type ErrorReport struct {
	ID           int
	UserID       *int
	PageURL      string
	SelectedText string
	Comment      string
	ClientIP     string
	UserAgent    string
	CreatedAt    time.Time
	// ResolvedAt - when an administrator marked the report fixed; nil
	// while it is new.
	ResolvedAt *time.Time
	// Populated for admin list when UserID is set.
	Username string
	UserSlug string
}

// ErrorReports is a slice of ErrorReport.
type ErrorReports []ErrorReport

// ErrorReportListOpts filters admin listing.
type ErrorReportListOpts struct {
	Limit  int
	Offset int
	// All - the fixed reports too; otherwise only the new ones.
	All bool
}

// ErrorReportRepository persists error reports.
type ErrorReportRepository interface {
	Create(ctx context.Context, report ErrorReport) (*ErrorReport, error)
	ListForAdmin(ctx context.Context, opts ErrorReportListOpts) (ErrorReports, int, error)
	// CountNew - the reports not marked fixed.
	CountNew(ctx context.Context) (int, error)
	// Resolve marks a report fixed (now); a fixed one keeps its time.
	Resolve(ctx context.Context, id int) error
	// Reopen returns a fixed report to the new ones.
	Reopen(ctx context.Context, id int) error
}
