package admin

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// RedirectsPerPage - redirects on a page of the list.
const RedirectsPerPage = 50

// RedirectList renders GET /admin/config/redirects: the form adding one,
// the import, and the redirects whose from or to path contains the
// search (q), by from path, a page at a time.
func (c *Controller) RedirectList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page := c.GetPageNumber(r)
	redirects, total, err := c.redirectService.Search(ctx, q, (page-1)*RedirectsPerPage, RedirectsPerPage)
	if err != nil {
		c.LogFailure(err, "error listing redirects")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Redirects")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["redirects"] = redirects
	data["search"] = q
	// The pages keep the search, not the last action's notice.
	pageQuery := url.Values{}
	if q != "" {
		pageQuery.Set("q", q)
	}
	data["pagination"] = pagination.New(page, total, RedirectsPerPage, "/admin/config/redirects", pageQuery)
	data["success"] = r.URL.Query().Get("success") == "1"
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	data["imported"] = r.URL.Query().Get("imported")
	data["error"] = r.URL.Query().Get("error")

	c.RenderOrFail(w, r, "admin/config/redirects.html", data)
}

// RedirectCreate handles POST /admin/config/redirects: a redirect added
// from one path to another.
func (c *Controller) RedirectCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/redirects", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	from := strings.TrimSpace(r.FormValue("from"))
	to := strings.TrimSpace(r.FormValue("to"))
	if from == "" || to == "" {
		http.Redirect(w, r, "/admin/config/redirects?error=empty", http.StatusSeeOther)
		return
	}

	if err := c.redirectService.Create(ctx, from, to); err != nil {
		c.LogFailure(err, "error creating redirect")
		http.Redirect(w, r, "/admin/config/redirects?error=create", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/redirects?success=1", http.StatusSeeOther)
}

// RedirectImport handles POST /admin/config/redirects/import: redirects
// added, one per line: "from -> to" or a RewriteRule line
// (parseRedirectBulk); a duplicate is skipped.
func (c *Controller) RedirectImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/redirects", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	bulk := strings.TrimSpace(r.FormValue("bulk"))
	if bulk == "" {
		http.Redirect(w, r, "/admin/config/redirects?error=empty", http.StatusSeeOther)
		return
	}

	lines := parseRedirectBulk(bulk)
	imported := 0
	for _, pair := range lines {
		if pair.From == "" || pair.To == "" {
			continue
		}
		if err := c.redirectService.Create(ctx, pair.From, pair.To); err != nil {
			c.logger.Warn("redirect import skip duplicate or error", "from", pair.From, "error", err)
			continue
		}
		imported++
	}
	http.Redirect(w, r, "/admin/config/redirects?imported="+strconv.Itoa(imported), http.StatusSeeOther)
}

// RedirectDelete handles POST /admin/config/redirects/delete/{id}: the
// redirect deleted.
func (c *Controller) RedirectDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/redirects", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/redirects?error=invalid", http.StatusSeeOther)
		return
	}

	if err := c.redirectService.Delete(ctx, id); err != nil {
		c.LogFailure(err, "error deleting redirect", "id", id)
		http.Redirect(w, r, "/admin/config/redirects?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/redirects?deleted=1", http.StatusSeeOther)
}

type redirectPair struct {
	From, To string
}

var htaccessRegex = regexp.MustCompile(`(?i)RewriteRule\s+([^\s$]+)\$?\s+([^\s\[]+)`)

func parseRedirectBulk(text string) []redirectPair {
	var pairs []redirectPair
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Try htaccess format: RewriteRule from$ to [R=301,L]
		if m := htaccessRegex.FindStringSubmatch(line); len(m) >= 3 {
			from := strings.TrimPrefix(strings.Trim(m[1], " "), "/")
			to := strings.Trim(m[2], " ")
			if to != "" && !strings.HasPrefix(to, "/") {
				to = "/" + to
			}
			pairs = append(pairs, redirectPair{From: from, To: to})
			continue
		}
		// Try simple format: from -> to
		if idx := strings.Index(line, "->"); idx >= 0 {
			from := strings.TrimSpace(line[:idx])
			to := strings.TrimSpace(line[idx+2:])
			from = strings.TrimPrefix(from, "/")
			if to != "" && !strings.HasPrefix(to, "/") {
				to = "/" + to
			}
			pairs = append(pairs, redirectPair{From: from, To: to})
		}
	}
	return pairs
}
