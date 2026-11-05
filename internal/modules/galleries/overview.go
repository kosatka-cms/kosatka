package galleries

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// overviewPath - the galleries overview in the admin.
const overviewPath = "/admin/content/galleries"

// overviewPerPage - the overview's page size.
const overviewPerPage = 50

// overviewRow - a content item on the galleries overview: whether it can
// have galleries, its galleries and their images.
type overviewRow struct {
	Node    modulekit.Node
	Enabled bool
	NodeGalleryCounts
}

// GalleriesOverview handles GET /admin/content/galleries: the content with
// its galleries - whether each can have them, how many and their images -
// by title, filtered by type and, with ?show=on, to the content that can
// have galleries.
func (c *GalleryController) GalleriesOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := c.GetPageNumber(r)
	q := r.URL.Query()
	typ, onlyOn := q.Get("type"), q.Get("show") == "on"

	enabledIDs, err := c.settings.EnabledNodeIDs(ctx)
	if err != nil {
		c.LogFailure(err, "the nodes that can have galleries")
		httputil.InternalServerError(w)
		return
	}
	enabled := make(map[int]bool, len(enabledIDs))
	for _, id := range enabledIDs {
		enabled[id] = true
	}

	var nodes []modulekit.Node
	var total int
	if onlyOn {
		byID, err := c.NodeRepo().ByIDs(ctx, enabledIDs)
		if err != nil {
			c.LogFailure(err, "the nodes that can have galleries")
			httputil.InternalServerError(w)
			return
		}
		for _, n := range byID {
			if typ == "" || string(n.Type) == typ {
				nodes = append(nodes, n)
			}
		}
		slices.SortFunc(nodes, func(a, b modulekit.Node) int {
			return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
		})
		total = len(nodes)
		from := min((page-1)*overviewPerPage, total)
		nodes = nodes[from:min(from+overviewPerPage, total)]
	} else {
		filter := modulekit.AdminNodesFilter{SortBy: modulekit.AdminNodesSortTitle, Limit: overviewPerPage, Offset: (page - 1) * overviewPerPage}
		if typ != "" {
			filter.Type = &typ
		}
		list, n, err := c.NodeRepo().ListForAdmin(ctx, filter)
		if err != nil {
			c.LogFailure(err, "the content for the galleries overview")
			httputil.InternalServerError(w)
			return
		}
		if list != nil {
			nodes = *list
		}
		total = n
	}

	ids := make([]int, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	counts, err := c.galleryService.CountsByNodeIDs(ctx, ids)
	if err != nil {
		c.LogFailure(err, "the galleries of the content")
		httputil.InternalServerError(w)
		return
	}
	rows := make([]overviewRow, len(nodes))
	for i, n := range nodes {
		rows[i] = overviewRow{Node: n, Enabled: enabled[n.ID], NodeGalleryCounts: counts[n.ID]}
	}

	labels := c.contentTypes.NodeTypeLabels(ctx)

	data := c.AdminPageData(r)
	title := i18n.T(ctx, "Galleries")
	data["title"] = title
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["rows"] = rows
	data["total"] = total
	data["contentTypes"] = modulekit.NodeTypesByLabel(labels)
	data["typeLabels"] = labels
	data["filterType"] = typ
	data["filterOn"] = onlyOn
	data["query"] = r.URL.RawQuery
	data["pagination"] = pagination.New(page, total, overviewPerPage, overviewPath, q)
	c.RenderOrFail(w, r, "admin/galleries.html", data)
}

// GalleriesOverviewToggle handles POST /admin/content/galleries/{nid}:
// whether the content can have galleries (enabled=1), as its form's switch
// sets it; back to the overview's page it came from (query).
func (c *GalleryController) GalleriesOverviewToggle(w http.ResponseWriter, r *http.Request) {
	nid, err := strconv.Atoi(r.PathValue("nid"))
	if err != nil || nid <= 0 {
		c.RenderNotFound(w, r)
		return
	}
	if _, err := c.NodeRepo().ByID(r.Context(), nid); err != nil {
		c.RenderLoadFailure(w, r, err, "the node for its galleries switch", "nid", nid)
		return
	}
	if err := c.settings.SetNodeEnabled(r.Context(), nid, r.FormValue("enabled") == "1"); err != nil {
		c.LogFailure(err, "setting the node's galleries switch", "nid", nid)
		httputil.InternalServerError(w)
		return
	}
	back := overviewPath
	if q, err := url.ParseQuery(r.FormValue("query")); err == nil && len(q) > 0 {
		back += "?" + q.Encode()
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}
