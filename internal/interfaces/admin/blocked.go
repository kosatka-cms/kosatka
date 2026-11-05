package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// BlockedBotList handles GET /admin/config/robots: the blocked User-Agent
// patterns and IP addresses, their add and import forms, and the site's
// own robots.txt rules.
func (c *Controller) BlockedBotList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bots, err := c.blockedBotService.List(ctx)
	if err != nil {
		c.LogFailure(err, "error listing blocked bots")
		httputil.InternalServerError(w)
		return
	}
	blockedIPs, err := c.blockedIPService.List(ctx)
	if err != nil {
		c.LogFailure(err, "error listing blocked IPs")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Access blocking")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["bots"] = bots
	data["blockedIPs"] = blockedIPs
	data["success"] = r.URL.Query().Get("success") == "1"
	data["deleted"] = r.URL.Query().Get("deleted") == "1"
	data["imported"] = r.URL.Query().Get("imported")
	data["error"] = r.URL.Query().Get("error")
	data["success_ip"] = r.URL.Query().Get("success_ip") == "1"
	data["deleted_ip"] = r.URL.Query().Get("deleted_ip") == "1"
	data["imported_ip"] = r.URL.Query().Get("imported_ip")
	data["error_ip"] = r.URL.Query().Get("error_ip")
	data["robots_saved"] = r.URL.Query().Get("robots_saved") == "1"
	site, err := c.siteInformationRepo.Get(ctx)
	if err != nil {
		c.LogFailure(err, "error loading the robots.txt rules")
		httputil.InternalServerError(w)
		return
	}
	data["robotsExtra"] = site.RobotsExtra

	c.RenderOrFail(w, r, "admin/config/blocked_bots.html", data)
}

// RobotsRulesSave handles POST /admin/config/robots/rules: the site's own
// robots.txt rules (service.RobotsTxt).
func (c *Controller) RobotsRulesSave(w http.ResponseWriter, r *http.Request) {
	rules := strings.TrimSpace(strings.ReplaceAll(r.FormValue("robots_extra"), "\r\n", "\n"))
	if err := c.siteInformationRepo.SetRobotsExtra(r.Context(), rules); err != nil {
		c.LogFailure(err, "error saving the robots.txt rules")
		http.Redirect(w, r, "/admin/config/robots?error=robots", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/robots?robots_saved=1", http.StatusSeeOther)
}

// BlockedBotCreate handles POST /admin/config/robots: a User-Agent
// pattern blocked, with its comment.
func (c *Controller) BlockedBotCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/robots", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	pattern := strings.TrimSpace(r.FormValue("pattern"))
	if pattern == "" {
		http.Redirect(w, r, "/admin/config/robots?error=empty", http.StatusSeeOther)
		return
	}
	comment := strings.TrimSpace(r.FormValue("comment"))

	if err := c.blockedBotService.Create(ctx, pattern, comment); err != nil {
		c.LogFailure(err, "error creating blocked bot")
		http.Redirect(w, r, "/admin/config/robots?error=create", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/robots?success=1", http.StatusSeeOther)
}

// BlockedBotImport handles POST /admin/config/robots/import: User-Agent
// patterns blocked, one per line ("#" - a comment line); a duplicate is
// skipped.
func (c *Controller) BlockedBotImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/robots", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	bulk := strings.TrimSpace(r.FormValue("bulk"))
	if bulk == "" {
		http.Redirect(w, r, "/admin/config/robots?error=empty", http.StatusSeeOther)
		return
	}

	comment := strings.TrimSpace(r.FormValue("comment"))
	patterns := parseBlockedBotBulk(bulk)
	imported := 0
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if err := c.blockedBotService.Create(ctx, p, comment); err != nil {
			c.logger.Warn("blocked bot import skip duplicate or error", "pattern", p, "error", err)
			continue
		}
		imported++
	}
	http.Redirect(w, r, "/admin/config/robots?imported="+strconv.Itoa(imported), http.StatusSeeOther)
}

// BlockedBotDelete handles POST /admin/config/robots/delete/{id}: the
// User-Agent pattern no longer blocked.
func (c *Controller) BlockedBotDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/robots", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/robots?error=invalid", http.StatusSeeOther)
		return
	}

	if err := c.blockedBotService.Delete(ctx, id); err != nil {
		c.LogFailure(err, "error deleting blocked bot", "id", id)
		http.Redirect(w, r, "/admin/config/robots?error=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/robots?deleted=1", http.StatusSeeOther)
}

func parseBlockedBotBulk(text string) []string {
	var patterns []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		p := strings.TrimSpace(line)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}
		patterns = append(patterns, p)
	}
	return patterns
}

// BlockedIPCreate handles POST /admin/config/robots/ip: an IP address
// blocked, with its comment.
func (c *Controller) BlockedIPCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/robots", http.StatusSeeOther)
		return
	}
	ctx := r.Context()

	ip := strings.TrimSpace(r.FormValue("ip"))
	if ip == "" {
		http.Redirect(w, r, "/admin/config/robots?error_ip=empty", http.StatusSeeOther)
		return
	}
	comment := strings.TrimSpace(r.FormValue("comment"))

	if err := c.blockedIPService.Create(ctx, ip, comment); err != nil {
		c.LogFailure(err, "error creating blocked IP")
		http.Redirect(w, r, "/admin/config/robots?error_ip=create", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/robots?success_ip=1", http.StatusSeeOther)
}

// BlockedIPImport handles POST /admin/config/robots/ip/import: IP
// addresses blocked, one per line ("#" - a comment line); a duplicate is
// skipped.
func (c *Controller) BlockedIPImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/robots", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	bulk := strings.TrimSpace(r.FormValue("bulk_ip"))
	if bulk == "" {
		http.Redirect(w, r, "/admin/config/robots?error_ip=empty", http.StatusSeeOther)
		return
	}

	comment := strings.TrimSpace(r.FormValue("comment"))
	ips := parseBlockedBotBulk(bulk)
	imported := 0
	for _, ip := range ips {
		if ip == "" {
			continue
		}
		if err := c.blockedIPService.Create(ctx, ip, comment); err != nil {
			c.logger.Warn("blocked IP import skip duplicate or error", "ip", ip, "error", err)
			continue
		}
		imported++
	}
	http.Redirect(w, r, "/admin/config/robots?imported_ip="+strconv.Itoa(imported), http.StatusSeeOther)
}

// BlockedIPDelete handles POST /admin/config/robots/ip/delete/{id}: the
// IP address no longer blocked.
func (c *Controller) BlockedIPDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config/robots", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, "/admin/config/robots?error_ip=invalid", http.StatusSeeOther)
		return
	}

	if err := c.blockedIPService.Delete(ctx, id); err != nil {
		c.LogFailure(err, "error deleting blocked IP", "id", id)
		http.Redirect(w, r, "/admin/config/robots?error_ip=delete", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/robots?deleted_ip=1", http.StatusSeeOther)
}
