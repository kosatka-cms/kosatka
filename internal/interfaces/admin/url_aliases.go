package admin

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// The URL aliases pages, with the Patterns and Rebuild tabs: every node's
// address (its
// slug) with its title, type and whether it follows its type's pattern
// (Node.PathAuto); every content type's pattern on one page; a type's
// existing addresses rebuilt by its pattern.

// URLAliasesPerPage - the addresses a page of the list shows.
const URLAliasesPerPage = 50

// URLAliasList handles GET /admin/config/url-aliases: the nodes' addresses
// by address, filtered by a part of it (q) and the type.
func (c *Controller) URLAliasList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := c.GetPageNumber(r)
	filter := entity.AdminNodesFilter{
		Path:   strings.ToLower(strings.Trim(r.URL.Query().Get("q"), "/ ")),
		SortBy: entity.AdminNodesSortPath,
		Limit:  URLAliasesPerPage,
		Offset: (page - 1) * URLAliasesPerPage,
	}
	if t := r.URL.Query().Get("type"); t != "" {
		filter.Type = &t
	}
	nodes, total, err := c.nodeRepo.ListForAdmin(ctx, filter)
	if err != nil {
		c.LogFailure(err, "error listing the nodes' addresses")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "URL aliases")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["activeTab"] = "aliases"
	data["nodes"] = nodes
	data["total"] = total
	data["pagination"] = pagination.New(page, total, URLAliasesPerPage, "/admin/config/url-aliases", r.URL.Query())
	data["contentTypes"] = c.adminFilterableContentTypes(ctx)
	data["typeLabels"] = c.nodeTypeLabels(ctx)
	data["filterQ"] = filter.Path
	if filter.Type != nil {
		data["filterType"] = *filter.Type
	}
	c.RenderOrFail(w, r, "admin/url-aliases/index.html", data)
}

// urlPatternRow - a content type on the patterns page: its pattern (as
// submitted, when the page shows a form's errors), the tokens it can take
// (pathTokens), the pattern's error.
type urlPatternRow struct {
	Type    entity.ContentTypeDef
	Pattern string
	Tokens  []string
	Error   string
}

// URLPatternList handles GET /admin/config/url-aliases/patterns: every
// content type's URL pattern, edited together.
func (c *Controller) URLPatternList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	types, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		c.LogFailure(err, "error listing the content types for their URL patterns")
		httputil.InternalServerError(w)
		return
	}
	rows := make([]urlPatternRow, 0, len(types))
	for _, t := range types {
		rows = append(rows, urlPatternRow{Type: t, Pattern: t.PathPattern, Tokens: c.pathTokens(ctx, t.ID)})
	}
	c.renderURLPatterns(w, r, rows, r.URL.Query().Get("saved") == "1")
}

// URLPatternsSave handles POST /admin/config/url-aliases/patterns: the
// patterns (pattern_<type ID>) checked all first (ValidateNodePathPattern
// against each type's fields); one invalid - the page again with the
// submitted patterns and the errors, nothing saved; else the changed ones
// saved.
func (c *Controller) URLPatternsSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	types, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		c.LogFailure(err, "error listing the content types for their URL patterns")
		httputil.InternalServerError(w)
		return
	}
	rows := make([]urlPatternRow, 0, len(types))
	invalid := false
	for _, t := range types {
		row := urlPatternRow{Type: t, Pattern: strings.TrimSpace(r.FormValue("pattern_" + strconv.Itoa(t.ID))), Tokens: c.pathTokens(ctx, t.ID)}
		fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, t.ID)
		if err != nil {
			c.LogFailure(err, "error loading the type's fields for its URL pattern", "id", t.ID)
			httputil.InternalServerError(w)
			return
		}
		if service.ValidateNodePathPattern(row.Pattern, fields) != nil {
			row.Error = i18n.T(ctx, "The URL pattern isn't valid: use lowercase Latin letters, digits, - _ . / and the tokens listed under the field.")
			invalid = true
		}
		rows = append(rows, row)
	}
	if invalid {
		c.renderURLPatterns(w, r, rows, false)
		return
	}
	for _, row := range rows {
		if row.Pattern == row.Type.PathPattern {
			continue
		}
		updated := row.Type
		updated.PathPattern = row.Pattern
		if _, err := c.contentTypeService.UpdateContentType(ctx, updated); err != nil {
			c.LogFailure(err, "error saving the URL pattern", "id", row.Type.ID)
			httputil.InternalServerError(w)
			return
		}
	}
	http.Redirect(w, r, "/admin/config/url-aliases/patterns?saved=1", http.StatusSeeOther)
}

func (c *Controller) renderURLPatterns(w http.ResponseWriter, r *http.Request, rows []urlPatternRow, saved bool) {
	ctx := r.Context()
	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "URL aliases")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", i18n.T(ctx, "URL patterns"))}
	data["activeTab"] = "patterns"
	data["rows"] = rows
	data["defaultPathPattern"] = service.DefaultNodePathPattern
	data["saved"] = saved
	c.RenderOrFail(w, r, "admin/url-aliases/patterns.html", data)
}

// urlRebuildRequest - the rebuild tab's choice (type, manual: include the
// addresses set by hand) and the step to continue after (after).
func (c *Controller) urlRebuildRequest(r *http.Request) (*service.NodePathRebuild, bool) {
	ctx := r.Context()
	name := r.FormValue("type")
	if name == "" {
		return nil, true
	}
	def, err := c.contentTypeService.ContentTypeByMachineName(ctx, name)
	if err != nil {
		return nil, false
	}
	fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, def.ID)
	if err != nil {
		c.LogFailure(err, "error loading the type's fields for its URL pattern", "type", name)
		return nil, false
	}
	after, _ := strconv.Atoi(r.FormValue("after"))
	return &service.NodePathRebuild{Type: *def, Fields: fields, IncludeManual: r.FormValue("manual") == "1", After: after}, true
}

// URLRebuildPage handles GET /admin/config/url-aliases/rebuild: a type and
// whether to include the addresses set by hand; with preview=1 the
// addresses that would change (a dry run of one step: the request's time
// may not reach every node); after a rebuild its totals (done, checked,
// changed, more - continue from after).
func (c *Controller) URLRebuildPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	types, err := c.contentTypeService.ContentTypes(ctx)
	if err != nil {
		c.LogFailure(err, "error listing the content types for the URL rebuild")
		httputil.InternalServerError(w)
		return
	}
	req, ok := c.urlRebuildRequest(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/url-aliases/rebuild", http.StatusSeeOther)
		return
	}
	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "URL aliases")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", i18n.T(ctx, "Rebuild addresses"))}
	data["activeTab"] = "rebuild"
	data["types"] = types
	data["defaultPathPattern"] = service.DefaultNodePathPattern
	if req != nil {
		data["req"] = req
		total, err := c.pathRebuilder.Count(ctx, *req)
		if err != nil {
			c.LogFailure(err, "error counting the nodes to rebuild", "type", req.Type.MachineName)
		}
		data["total"] = total
		if q.Get("preview") == "1" {
			preview := *req
			preview.DryRun, preview.After = true, 0
			res, err := c.pathRebuilder.Step(ctx, preview, time.Now().Add(interfaces.StepBudget()))
			if err != nil {
				c.LogFailure(err, "error previewing the URL rebuild", "type", req.Type.MachineName)
				data["previewFailed"] = true
			} else {
				data["preview"] = res
			}
		}
	}
	if q.Get("done") == "1" || q.Get("more") == "1" {
		data["checked"], _ = strconv.Atoi(q.Get("checked"))
		data["changed"], _ = strconv.Atoi(q.Get("changed"))
		data["more"] = q.Get("more") == "1"
		data["after"] = q.Get("after")
		data["done"] = q.Get("done") == "1"
	}
	data["error"] = q.Get("error")
	c.RenderOrFail(w, r, "admin/url-aliases/rebuild.html", data)
}

// URLRebuild handles POST /admin/config/url-aliases/rebuild: one step of
// the type's rebuild (until the request's time runs out), its totals added
// to the earlier steps' (checked, changed); the page then offers to
// continue (more) or reports it done.
func (c *Controller) URLRebuild(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, ok := c.urlRebuildRequest(r)
	if !ok || req == nil {
		http.Redirect(w, r, "/admin/config/url-aliases/rebuild", http.StatusSeeOther)
		return
	}
	res, err := c.pathRebuilder.Step(ctx, *req, time.Now().Add(interfaces.StepBudget()))
	checked, _ := strconv.Atoi(r.FormValue("checked"))
	changed, _ := strconv.Atoi(r.FormValue("changed"))
	checked += res.Checked
	changed += res.Changed
	c.logger.Info("URL aliases rebuilt", "type", req.Type.MachineName, "manual", req.IncludeManual, "checked", res.Checked, "changed", res.Changed, "more", res.More, "error", err)
	v := url.Values{"type": {req.Type.MachineName}, "checked": {strconv.Itoa(checked)}, "changed": {strconv.Itoa(changed)}, "after": {strconv.Itoa(res.Last)}}
	if req.IncludeManual {
		v.Set("manual", "1")
	}
	switch {
	case err != nil:
		c.LogFailure(err, "error rebuilding the URL aliases", "type", req.Type.MachineName)
		v.Set("error", "rebuild")
		v.Set("more", "1")
	case res.More:
		v.Set("more", "1")
	default:
		v.Set("done", "1")
	}
	http.Redirect(w, r, "/admin/config/url-aliases/rebuild?"+v.Encode(), http.StatusSeeOther)
}
