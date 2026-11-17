package admin

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestBlockedIPsPageRender - the IP section of the robots page: its
// anchor, the search keeping what was asked, a page of the IPs with which
// of how many and links to the others that keep the search and open at
// the section; nothing found for a search that matches nothing.
func TestBlockedIPsPageRender(t *testing.T) {
	added := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	query := url.Values{"ip_q": {"10.0"}}
	out := renderMailPage(t, "admin/config/blocked_bots.html", map[string]any{
		"blockedIPs": []entity.BlockedIP{{ID: 7, IP: "10.0.0.7", Comment: "spam", CreatedAt: added}},
		"ipSearch":   "10.0",
		"pagination": pagination.NewWithAnchor(2, 120, BlockedIPsPerPage, "/admin/config/robots", query, "blocked-ips"),
	})
	for _, want := range []string{
		`<section class="admin-settings-group" id="blocked-ips">`,
		`<form method="get" action="/admin/config/robots#blocked-ips" class="admin-filters" role="search">`,
		`name="ip_q" value="10.0"`,
		`<a href="/admin/config/robots#blocked-ips" class="admin-filter-reset">Reset</a>`,
		`<td><code>10.0.0.7</code></td>`,
		`51-100 of 120`,
		`href="/admin/config/robots?ip_q=10.0&amp;page=3#blocked-ips"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in\n%s", want, out)
		}
	}

	out = renderMailPage(t, "admin/config/blocked_bots.html", map[string]any{
		"ipSearch":   "nothing",
		"pagination": pagination.NewWithAnchor(1, 0, BlockedIPsPerPage, "/admin/config/robots", url.Values{"ip_q": {"nothing"}}, "blocked-ips"),
	})
	if !strings.Contains(out, `<td colspan="4" class="admin-table-empty">Nothing found</td>`) {
		t.Errorf("an empty search:\n%s", out)
	}
}
