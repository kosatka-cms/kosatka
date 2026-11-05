package interfaces

import (
	"net/http"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// RecentChangesPath - the recent changes of the content (Content →
// Changes).
const RecentChangesPath = "/admin/content/changes"

// RecentChangesPerPage - revisions on a page of the recent changes.
const RecentChangesPerPage = 50

// recentDay - a day of the recent changes (Today, Yesterday - those
// days) and its revisions, newest first, each with its time of day.
type recentDay struct {
	Day              time.Time
	Today, Yesterday bool
	Rows             []recentRow
}

type recentRow struct {
	service.RecentRow
	Time string
}

// byDay - rows (newest first) by their day in loc, now's day being today.
func byDay(rows []service.RecentRow, loc *time.Location, now time.Time) []recentDay {
	day := func(t time.Time) time.Time {
		t = t.In(loc)
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}
	today := day(now)
	var days []recentDay
	for _, row := range rows {
		d := day(row.CreatedAt)
		if len(days) == 0 || !days[len(days)-1].Day.Equal(d) {
			days = append(days, recentDay{Day: d, Today: d.Equal(today), Yesterday: d.Equal(today.AddDate(0, 0, -1))})
		}
		last := &days[len(days)-1]
		last.Rows = append(last.Rows, recentRow{RecentRow: row, Time: row.CreatedAt.In(loc).Format("15:04")})
	}
	return days
}

// RecentChanges renders GET /admin/content/changes: the saves of the
// content of every type keeping its history that the viewer may see the
// history of (editing it: any of the type's, or their own published
// ones), newest first, by days; the filter - a part of the title, the
// type, a part of the saver's name, the days.
func (c *NodeController) RecentChanges(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil || c.revisions == nil || !c.authService.HasPermission(ctx, user, service.PermViewRevisions) {
		c.RenderForbidden(w, r)
		return
	}
	on, off, err := c.revisions.HistoryTypes(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "listing the content types for the recent changes")
		return
	}
	f, err := c.revisions.RecentScope(ctx, user.ID, func(p string) bool { return c.authService.HasPermission(ctx, user, p) })
	if err != nil {
		c.RenderLoadFailure(w, r, err, "the recent changes' scope")
		return
	}
	q := r.URL.Query()
	page := c.GetPageNumber(r)
	f.Title, f.Type, f.Actor = q.Get("title"), q.Get("type"), q.Get("who")
	f.Offset, f.Limit = (page-1)*RecentChangesPerPage, RecentChangesPerPage
	loc := sitetime.FromContext(ctx)
	f.From, _ = time.ParseInLocation(time.DateOnly, q.Get("from"), loc)
	if to, err := time.ParseInLocation(time.DateOnly, q.Get("to"), loc); err == nil {
		f.To = to.AddDate(0, 0, 1)
	}
	rows, total, err := c.revisions.Recent(ctx, f)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "listing the recent changes")
		return
	}
	data := c.AdminPageData(r)
	data["title"] = i18n.T(ctx, "Recent changes")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["days"] = byDay(rows, loc, time.Now())
	data["total"] = total
	data["filter"] = f
	data["fromValue"], data["toValue"] = q.Get("from"), q.Get("to")
	data["filtered"] = f.Title != "" || f.Type != "" || f.Actor != "" || q.Get("from") != "" || q.Get("to") != ""
	data["historyTypes"] = on
	data["noHistoryTypes"] = off
	data["pagination"] = pagination.New(page, total, RecentChangesPerPage, RecentChangesPath, q)
	c.RenderOrFail(w, r, "admin/content/changes.html", data)
}
