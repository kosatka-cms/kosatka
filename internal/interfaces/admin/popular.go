package admin

import (
	"net/http"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// popularPerPage - the rows of a page of the popular content.
const popularPerPage = 50

// popularPath - the popular content's report.
const popularPath = "/admin/reports/popular"

// PopularContent renders GET /admin/reports/popular: the published
// content viewed most in a period (?period= - entity.PopularPeriods,
// today's by default) of a content type (?type=), most first, its views
// in the period and in all. keepDays - how long the views by day are
// kept: a longer period than that counts only those.
func (c *Controller) PopularContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if c.dashboard == nil {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	period := entity.ParsePopularPeriod(q.Get("period"))
	typ := q.Get("type")
	page := c.GetPageNumber(r)
	loc := sitetime.Location()
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	rows, total, err := c.dashboard.Popular(ctx, entity.PopularQuery{Period: period, Today: today, Type: typ,
		Limit: popularPerPage, Offset: (page - 1) * popularPerPage})
	if err != nil {
		c.RenderLoadFailure(w, r, err, "listing the popular content")
		return
	}
	types, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		c.LogFailure(err, "the popular content's type filter")
	}
	labels := make(map[string]string, len(types))
	for _, t := range types {
		labels[t.MachineName] = t.Label
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Popular content")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["period"] = string(period)
	data["periods"] = entity.PopularPeriods
	data["type"] = typ
	data["contentTypes"] = types
	data["typeLabels"] = labels
	data["rows"] = rows
	data["rankFrom"] = (page-1)*popularPerPage + 1
	data["pagination"] = pagination.New(page, total, popularPerPage, popularPath, q)
	data["counting"] = c.viewCountingService != nil && c.viewCountingService.IsEnabled()
	if c.viewCountingService != nil && period.Days() > 0 {
		if s, err := c.viewCountingService.GetSettings(ctx); err != nil {
			c.LogFailure(err, "the popular content's statistics settings")
		} else if s.DailyViewsKeepDays > 0 && s.DailyViewsKeepDays < period.Days() {
			data["keepDays"] = s.DailyViewsKeepDays
		}
	}
	c.RenderOrFail(w, r, "admin/reports/popular.html", data)
}
