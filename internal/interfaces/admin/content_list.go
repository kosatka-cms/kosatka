package admin

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// nodeTypeLabels - node type machine_name → label (the content types' and
// the modules' node types'). A type missing here shows its machine_name.
func (c *Controller) nodeTypeLabels(ctx context.Context) map[string]string {
	return c.contentTypeService.NodeTypeLabels(ctx)
}

// adminFilterableContentTypes - the "Type" filter options on
// /admin/content and /admin/moderation: every node type, sorted by label.
func (c *Controller) adminFilterableContentTypes(ctx context.Context) []entity.ContentType {
	return entity.NodeTypesByLabel(c.nodeTypeLabels(ctx))
}

// historyTypes - the types whose content's rows get a History button:
// those keeping their history, for a user who may view it (the history
// page checks the content itself); none otherwise.
func (c *Controller) historyTypes(r *http.Request) map[string]bool {
	ctx := r.Context()
	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil || !c.authService.HasPermission(ctx, user, service.PermViewRevisions) {
		return nil
	}
	defs, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		c.LogFailure(err, "listing the content types for the History buttons")
		return nil
	}
	out := map[string]bool{}
	for _, d := range defs {
		if d.Revisions {
			out[d.MachineName] = true
		}
	}
	return out
}

// ContentPerPage - content items on a page of the content and moderation
// lists.
const ContentPerPage = 25

// CommentsPerPage - comments on a page of the comments list.
const CommentsPerPage = 25

// ContentList handles GET /admin/content: the content, filtered by status
// (or scheduled), moderation, type and a part of the title or address
// (q), sorted by the column chosen, a page at a time.
func (c *Controller) ContentList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := c.GetPageNumber(r)

	filter := entity.AdminNodesFilter{
		Limit:  ContentPerPage,
		Offset: (page - 1) * ContentPerPage,
	}

	sortKey := strings.TrimSpace(r.URL.Query().Get("sort"))
	validSort := sortKey == entity.AdminNodesSortStatus || sortKey == entity.AdminNodesSortModeration ||
		sortKey == entity.AdminNodesSortCreated || sortKey == entity.AdminNodesSortUpdated ||
		sortKey == entity.AdminNodesSortViews
	orderKey := strings.TrimSpace(r.URL.Query().Get("order"))
	displayOrder := orderKey
	if displayOrder != "asc" && displayOrder != "desc" {
		displayOrder = "desc"
	}
	desc := displayOrder == "desc"
	if validSort {
		filter.SortBy = sortKey
		filter.Desc = desc
	}

	if s := r.URL.Query().Get("status"); s == "scheduled" {
		filter.Scheduled = true
	} else if v, err := strconv.Atoi(s); err == nil && (v == 0 || v == 1) {
		filter.Status = &v
	}
	if m := r.URL.Query().Get("moderation"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && (v == 0 || v == 1) {
			filter.OnModeration = &v
		}
	}
	if t := r.URL.Query().Get("type"); t != "" {
		filter.Type = &t
	}
	filter.Search = strings.TrimSpace(r.URL.Query().Get("q"))

	nodes, totalCount, err := c.nodeRepo.ListForAdmin(ctx, filter)
	if err != nil {
		c.LogFailure(err, "error listing nodes for admin")
		httputil.InternalServerError(w)
		return
	}

	pager := pagination.New(page, totalCount, ContentPerPage, "/admin/content", r.URL.Query())

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Content")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["nodes"] = nodes
	data["pagination"] = pager
	data["totalCount"] = totalCount
	data["search"] = filter.Search
	data["filtered"] = filter.Search != "" || filter.Status != nil || filter.Scheduled || filter.OnModeration != nil || filter.Type != nil
	data["filterScheduled"] = filter.Scheduled
	data["contentTypes"] = c.adminFilterableContentTypes(ctx)
	data["typeLabels"] = c.nodeTypeLabels(ctx)
	data["historyTypes"] = c.historyTypes(r)
	if filter.Status != nil {
		data["filterStatus"] = *filter.Status
	}
	if filter.OnModeration != nil {
		data["filterModeration"] = *filter.OnModeration
	}
	if filter.Type != nil {
		data["filterType"] = *filter.Type
	}
	if validSort {
		data["sort"] = sortKey
		data["order"] = displayOrder
	} else {
		data["sort"] = entity.AdminNodesSortUpdated
		data["order"] = "desc"
	}
	data["sortURLStatus"] = contentListSortURL(r, entity.AdminNodesSortStatus)
	data["sortURLCreated"] = contentListSortURL(r, entity.AdminNodesSortCreated)
	data["sortURLUpdated"] = contentListSortURL(r, entity.AdminNodesSortUpdated)
	data["sortURLViews"] = contentListSortURL(r, entity.AdminNodesSortViews)

	c.RenderOrFail(w, r, "admin/content/index.html", data)
}

// ModerationList handles GET /admin/moderation: the content awaiting
// moderation, filtered by status and type, sorted by the column chosen,
// a page at a time.
func (c *Controller) ModerationList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := c.GetPageNumber(r)

	onMod := 1
	filter := entity.AdminNodesFilter{
		OnModeration: &onMod,
		Limit:        ContentPerPage,
		Offset:       (page - 1) * ContentPerPage,
	}

	sortKey := strings.TrimSpace(r.URL.Query().Get("sort"))
	validSort := sortKey == entity.AdminNodesSortTitle || sortKey == entity.AdminNodesSortStatus ||
		sortKey == entity.AdminNodesSortCreated || sortKey == entity.AdminNodesSortUpdated ||
		sortKey == entity.AdminNodesSortViews
	orderKey := strings.TrimSpace(r.URL.Query().Get("order"))
	displayOrder := orderKey
	if displayOrder != "asc" && displayOrder != "desc" {
		displayOrder = "desc"
	}
	desc := displayOrder == "desc"

	if s := r.URL.Query().Get("status"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && (v == 0 || v == 1) {
			filter.Status = &v
		}
	}
	if t := r.URL.Query().Get("type"); t != "" {
		filter.Type = &t
	}

	if validSort {
		filter.SortBy = sortKey
		filter.Desc = desc
	} else {
		filter.SortBy = entity.AdminNodesSortCreated
		filter.Desc = true
		displayOrder = "desc"
		sortKey = entity.AdminNodesSortCreated
	}

	nodes, totalCount, err := c.nodeRepo.ListForAdmin(ctx, filter)
	if err != nil {
		c.LogFailure(err, "error listing moderation queue")
		httputil.InternalServerError(w)
		return
	}

	pager := pagination.New(page, totalCount, ContentPerPage, "/admin/moderation", r.URL.Query())

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Content awaiting moderation")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", i18n.T(ctx, "Moderation"))}
	data["nodes"] = nodes
	data["pagination"] = pager
	data["filtered"] = filter.Status != nil || filter.Type != nil
	data["contentTypes"] = c.adminFilterableContentTypes(ctx)
	data["typeLabels"] = c.nodeTypeLabels(ctx)
	if filter.Status != nil {
		data["filterStatus"] = *filter.Status
	}
	if filter.Type != nil {
		data["filterType"] = *filter.Type
	}
	data["sort"] = sortKey
	data["order"] = displayOrder
	data["sortURLTitle"] = moderationListSortURL(r, entity.AdminNodesSortTitle)
	data["sortURLStatus"] = moderationListSortURL(r, entity.AdminNodesSortStatus)
	data["sortURLCreated"] = moderationListSortURL(r, entity.AdminNodesSortCreated)
	data["sortURLUpdated"] = moderationListSortURL(r, entity.AdminNodesSortUpdated)
	data["sortURLViews"] = moderationListSortURL(r, entity.AdminNodesSortViews)
	// Everyone here may take items off moderation in bulk; publishing
	// takes its own permission (mayBulkContent).
	user, _ := c.GetCurrentUser(r)
	data["bulkPublish"] = c.authService.CanPublishNode(ctx, user) || c.authService.HasPermission(ctx, user, service.PermAdministerContent)

	c.RenderOrFail(w, r, "admin/content/moderation.html", data)
}

// CommentList handles GET /admin/content/comment: the comments of a status
// tab (?status=), sorted by creation or update, a page at a time.
func (c *Controller) CommentList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := c.GetPageNumber(r)

	sortKey := strings.TrimSpace(r.URL.Query().Get("sort"))
	validSort := sortKey == entity.AdminCommentsSortCreated || sortKey == entity.AdminCommentsSortUpdated
	orderKey := strings.TrimSpace(r.URL.Query().Get("order"))
	displayOrder := orderKey
	if displayOrder != "asc" && displayOrder != "desc" {
		displayOrder = "desc"
	}
	desc := displayOrder == "desc"

	tabs, tab, err := c.commentTabs(ctx, r.URL.Query().Get("status"))
	if err != nil {
		c.LogFailure(err, "error counting comments by status")
		httputil.InternalServerError(w)
		return
	}
	filter := entity.AdminCommentsListFilter{
		Status: tab.Status,
		Limit:  CommentsPerPage,
		Offset: (page - 1) * CommentsPerPage,
		Desc:   desc,
	}
	if validSort {
		filter.SortBy = sortKey
	}

	comments, totalCount, err := c.commentsRepo.ListForAdmin(ctx, filter)
	if err != nil {
		c.LogFailure(err, "error listing comments for admin")
		httputil.InternalServerError(w)
		return
	}

	q := r.URL.Query()
	q.Set("status", tab.Key)
	pager := pagination.New(page, totalCount, CommentsPerPage, commentsPath, q)

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Comments")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["comments"] = c.commentRows(ctx, comments, tab)
	data["tabs"] = tabs
	data["tab"] = tab
	data["back"] = commentsPath + "?" + q.Encode()
	data["pagination"] = pager
	if validSort {
		data["sort"] = sortKey
		data["order"] = displayOrder
	} else {
		data["sort"] = entity.AdminCommentsSortCreated
		data["order"] = "desc"
	}
	data["sortURLCreated"] = commentListSortURL(r, entity.AdminCommentsSortCreated, tab.Key)
	data["sortURLUpdated"] = commentListSortURL(r, entity.AdminCommentsSortUpdated, tab.Key)

	c.RenderOrFail(w, r, "admin/content/comments.html", data)
}

// contentListSortURL builds /admin/content?sort=&order= preserving filters (resets to page 1).
func contentListSortURL(r *http.Request, column string) string {
	q := r.URL.Query()
	currentSort := strings.TrimSpace(q.Get("sort"))
	if currentSort != entity.AdminNodesSortStatus && currentSort != entity.AdminNodesSortModeration &&
		currentSort != entity.AdminNodesSortCreated && currentSort != entity.AdminNodesSortUpdated &&
		currentSort != entity.AdminNodesSortViews {
		currentSort = entity.AdminNodesSortUpdated
	}
	currentOrder := strings.TrimSpace(q.Get("order"))
	if currentOrder != "asc" && currentOrder != "desc" {
		currentOrder = "desc"
	}

	nextOrder := "desc"
	if currentSort == column {
		if currentOrder == "desc" {
			nextOrder = "asc"
		} else {
			nextOrder = "desc"
		}
	}

	v := url.Values{}
	v.Set("sort", column)
	v.Set("order", nextOrder)
	if s := strings.TrimSpace(q.Get("status")); s != "" {
		v.Set("status", s)
	}
	if m := strings.TrimSpace(q.Get("moderation")); m != "" {
		v.Set("moderation", m)
	}
	if t := strings.TrimSpace(q.Get("type")); t != "" {
		v.Set("type", t)
	}
	return "/admin/content?" + v.Encode()
}

// moderationListSortURL builds /admin/moderation?sort=&order= (resets to page 1).
func moderationListSortURL(r *http.Request, column string) string {
	q := r.URL.Query()
	currentSort := strings.TrimSpace(q.Get("sort"))
	if currentSort != entity.AdminNodesSortTitle && currentSort != entity.AdminNodesSortStatus &&
		currentSort != entity.AdminNodesSortCreated && currentSort != entity.AdminNodesSortUpdated &&
		currentSort != entity.AdminNodesSortViews {
		currentSort = entity.AdminNodesSortCreated
	}
	currentOrder := strings.TrimSpace(q.Get("order"))
	if currentOrder != "asc" && currentOrder != "desc" {
		currentOrder = "desc"
	}

	nextOrder := "desc"
	if currentSort == column {
		if currentOrder == "desc" {
			nextOrder = "asc"
		} else {
			nextOrder = "desc"
		}
	}

	v := url.Values{}
	v.Set("sort", column)
	v.Set("order", nextOrder)
	if s := strings.TrimSpace(q.Get("status")); s != "" {
		v.Set("status", s)
	}
	if t := strings.TrimSpace(q.Get("type")); t != "" {
		v.Set("type", t)
	}
	return "/admin/moderation?" + v.Encode()
}

// commentListSortURL builds /admin/content/comment?sort=&order= (resets to page 1).
func commentListSortURL(r *http.Request, column, status string) string {
	q := r.URL.Query()
	currentSort := strings.TrimSpace(q.Get("sort"))
	if currentSort != entity.AdminCommentsSortCreated && currentSort != entity.AdminCommentsSortUpdated {
		currentSort = entity.AdminCommentsSortCreated
	}
	currentOrder := strings.TrimSpace(q.Get("order"))
	if currentOrder != "asc" && currentOrder != "desc" {
		currentOrder = "desc"
	}

	nextOrder := "desc"
	if currentSort == column {
		if currentOrder == "desc" {
			nextOrder = "asc"
		} else {
			nextOrder = "desc"
		}
	}

	v := url.Values{}
	v.Set("status", status)
	v.Set("sort", column)
	v.Set("order", nextOrder)
	return commentsPath + "?" + v.Encode()
}
