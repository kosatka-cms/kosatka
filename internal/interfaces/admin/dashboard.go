package admin

import (
	"context"
	"net/http"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// dashboardRecentDays - the recent period of the Overview's figures.
const dashboardRecentDays = 7

// dashboardListSize - the rows of each of the Overview's lists.
const dashboardListSize = 5

// dashboardTile - a figure of the Overview: its value, a line under it, the
// list it opens.
type dashboardTile struct {
	Label string
	Value int
	Note  string
	URL   string
}

// dashboardBlock - a module's block as the Overview renders it.
type dashboardBlock struct {
	service.DashboardBlock
	Title     string
	LinkLabel string
	Data      any
}

// Dashboard renders GET /admin, the Overview: the figures (the content,
// the comments, the users for who manages them, the views while they are
// counted); what waits (every admin menu item with a count); the recent
// changes (for who may see the history), today's popular content, the
// latest comments; the modules' blocks (DashboardBlocks), each for who
// has its permission. A part that can't load is left out (logged).
func (c *Controller) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := c.GetCurrentUser(r)
	has := func(p string) bool { return user != nil && c.authService.HasPermission(ctx, user, p) }
	loc := sitetime.Location()
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	counting := c.viewCountingService != nil && c.viewCountingService.IsEnabled()

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Overview")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["today"] = now

	if c.dashboard != nil {
		if st, err := c.dashboard.Stats(ctx, now.AddDate(0, 0, -dashboardRecentDays), today); err != nil {
			c.LogFailure(err, "the Overview's figures")
		} else {
			data["tiles"] = c.dashboardTiles(ctx, st, has, counting)
		}
		if counting {
			if popular, _, err := c.dashboard.Popular(ctx, entity.PopularQuery{Period: entity.PopularPeriodToday, Today: today, Limit: dashboardListSize}); err != nil {
				c.LogFailure(err, "the Overview's popular content")
			} else {
				data["popular"] = popular
			}
		}
	}
	data["waiting"] = dashboardWaiting(data["adminMenu"])
	if c.revisions != nil && user != nil && has(service.PermViewRevisions) {
		if f, err := c.revisions.RecentScope(ctx, user.ID, has); err != nil {
			c.LogFailure(err, "the Overview's changes scope")
		} else {
			f.Limit = dashboardListSize
			if rows, _, err := c.revisions.Recent(ctx, f); err != nil {
				c.LogFailure(err, "the Overview's recent changes")
			} else {
				data["changes"] = rows
				data["showChanges"] = true
			}
		}
	}
	if c.commentsRepo != nil {
		if comments, _, err := c.commentsRepo.ListForAdmin(ctx, entity.AdminCommentsListFilter{Status: entity.CommentPublished, Desc: true, Limit: dashboardListSize}); err != nil {
			c.LogFailure(err, "the Overview's latest comments")
		} else {
			data["comments"] = comments
		}
	}
	data["mainBlocks"], data["sideBlocks"] = c.dashboardModuleBlocks(ctx, has)

	c.RenderOrFail(w, r, "admin/dashboard.html", data)
}

// dashboardTiles - the figures the user gets: the users' only for who
// manages them, the views only while they are counted.
func (c *Controller) dashboardTiles(ctx context.Context, st entity.DashboardStats, has func(string) bool, counting bool) []dashboardTile {
	tiles := []dashboardTile{
		{Label: i18n.T(ctx, "Content"), Value: st.Published, URL: "/admin/content",
			Note: i18n.T(ctx, "+@count published in @days days", "@count", st.PublishedSince, "@days", dashboardRecentDays)},
		{Label: i18n.T(ctx, "Comments in @days days", "@days", dashboardRecentDays), Value: st.CommentsSince, URL: "/admin/content/comment",
			Note: i18n.T(ctx, "@count in all", "@count", st.Comments)},
	}
	if has(service.PermAdministerUsers) {
		tiles = append(tiles, dashboardTile{Label: i18n.T(ctx, "New users in @days days", "@days", dashboardRecentDays), Value: st.UsersSince,
			URL: "/admin/people", Note: i18n.T(ctx, "@count in all", "@count", st.Users)})
	}
	if counting {
		tiles = append(tiles, dashboardTile{Label: i18n.T(ctx, "Views today"), Value: st.ViewsToday,
			URL: "/admin/content?sort=views&order=desc", Note: i18n.T(ctx, "@count in all", "@count", st.ViewsTotal)})
	}
	return tiles
}

// dashboardWaiting - the admin menu's items (the user's, counted) with
// something waiting.
func dashboardWaiting(menu any) []entity.AdminMenuItem {
	sections, _ := menu.([]entity.AdminMenuSection)
	var out []entity.AdminMenuItem
	for _, s := range sections {
		for _, it := range s.Items {
			if it.Count > 0 {
				out = append(out, it)
			}
		}
	}
	return out
}

// dashboardModuleBlocks - the enabled modules' blocks the user may see,
// with their data, by column.
func (c *Controller) dashboardModuleBlocks(ctx context.Context, has func(string) bool) (main, side []dashboardBlock) {
	for _, b := range service.DashboardBlocks() {
		if (b.Module != "" && !service.ModuleEnabled(b.Module)) || (b.Permission != "" && !has(b.Permission)) {
			continue
		}
		block := dashboardBlock{DashboardBlock: b, Title: i18n.T(ctx, b.Title)}
		if b.LinkLabel != "" {
			block.LinkLabel = i18n.T(ctx, b.LinkLabel)
		}
		if b.Data != nil {
			d, err := b.Data(ctx)
			if err != nil {
				c.LogFailure(err, "a module's Overview block", "block", b.Name)
				continue
			}
			block.Data = d
		}
		if b.Side {
			side = append(side, block)
		} else {
			main = append(main, block)
		}
	}
	return main, side
}
