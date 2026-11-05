package notfoundlog

import (
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/not_found_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestListPage - the Redirects page's tabs (the log's active, with its
// count), a path with its hits, where the last visitor came from (this
// site's page as its path), its folded redirect form and Ignore; an
// ignored path offers Restore.
func TestListPage(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	at := time.Date(2026, 10, 25, 19, 40, 0, 0, time.UTC)
	rows := []row{
		{Path: Path{ID: 7, Path: "/news/old-<post>", Hits: 41, FirstSeen: at, LastSeen: at}, RefURL: "/heroes/", RefLabel: "/heroes/", OwnRef: true},
		{Path: Path{ID: 8, Path: "/gone", Hits: 3, FirstSeen: at, LastSeen: at, Ignored: true}, RefURL: "https://search.example.org/?q=gone", RefLabel: "search.example.org/?q=gone"},
	}
	back := manifest.AdminPath + "?status=all"
	out := testkit.RenderPageContent(t, "admin/not_found_log.html", map[string]any{"rows": rows, "total": 2, "keepDays": 90,
		"filter": Filter{Status: StatusAll}, "back": back, "notFoundCurrent": true, "notFoundCount": 5,
		"pagination": pagination.New(1, 2, perPage, manifest.AdminPath, nil)})
	for _, want := range []string{
		`<a class="nav-link" href="/admin/config/redirects">Redirects</a>`,
		`class="nav-link active" href="/admin/config/redirects/not-found" aria-current="page">Not found (404) <span class="admin-nav-count">5</span>`,
		`<option value="all" selected>All</option>`,
		`<code>/news/old-&lt;post&gt;</code>`, `<td class="text-end admin-table-num"><b>41</b></td>`,
		`<a href="/heroes/" rel="noopener noreferrer">/heroes/</a> <span class="admin-table-muted">(on this site)</span>`,
		`<a href="https://search.example.org/?q=gone" rel="noopener noreferrer" target="_blank">search.example.org/?q=gone</a>`,
		`data-not-found-toggle="nf-7"`, `<tr id="nf-7" class="not-found-form-row" hidden>`,
		`action="/admin/config/redirects/not-found/7/redirect"`, `data-not-found-suggest="/news/old-&lt;post&gt;"`,
		`action="/admin/config/redirects/not-found/7/ignore"`, `action="/admin/config/redirects/not-found/8/restore"`,
		`name="back" value="/admin/config/redirects/not-found?status=all"`,
		"1-2 of 2 · kept 90 days",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}

// TestSettingsPage - the keeping as saved, the menu count switch, the
// patterns one per line, the Clear form.
func TestSettingsPage(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	st := Settings{KeepDays: 180, Ignore: []string{"/wp-*", "*.php"}}
	out := testkit.RenderPageContent(t, "admin/not_found_log_settings.html", map[string]any{"settings": st, "ignoreText": "/wp-*\n*.php", "keepChoices": keepChoices, "cleared": "12"})
	for _, want := range []string{
		`<option value="180" selected>180 days</option>`, `<option value="30">30 days</option>`,
		`id="show_count" name="show_count" value="1" class="form-check-input">`,
		">/wp-*\n*.php</textarea>", `action="/admin/config/redirects/not-found/clear"`, "Addresses forgotten: 12",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}

// TestRedirectsTab - the tab on the Redirects page: not active there, the
// new paths' count.
func TestRedirectsTab(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	out := testkit.RenderNamed(t, "redirects_tabs_not_found_log", map[string]any{"notFoundCount": 3}, "partials/not_found_log_hooks.html")
	if !strings.Contains(out, `<a class="nav-link" href="/admin/config/redirects/not-found"`) || !strings.Contains(out, `<span class="admin-nav-count">3</span>`) {
		t.Errorf("tab: %s", out)
	}
}
