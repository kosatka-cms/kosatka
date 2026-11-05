package admin

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

// TestDashboardWaiting - what waits: the menu's items with a count, by
// their count's label when they have one.
func TestDashboardWaiting(t *testing.T) {
	menu := []entity.AdminMenuSection{
		{Label: "Content", Items: []entity.AdminMenuItem{{Label: "Pending moderation", Path: "/admin/moderation", Count: 3}, {Label: "Comments", Path: "/c"}}},
		{Label: "Configuration", Items: []entity.AdminMenuItem{{Label: "Redirects", Path: "/r", Count: 37, CountLabel: "Not found (404)"}}},
	}
	got := dashboardWaiting(menu)
	if len(got) != 2 || got[0].Path != "/admin/moderation" || got[1].CountLabel != "Not found (404)" {
		t.Errorf("waiting: %+v", got)
	}
	if dashboardWaiting(nil) != nil {
		t.Error("no menu, nothing waits")
	}
}

// TestDashboardModuleBlocks - a module's block shows while the module is
// on and to who has its permission, with its data, in its column; a
// block whose data fails is left out.
func TestDashboardModuleBlocks(t *testing.T) {
	t.Cleanup(service.ResetDashboardBlocks)
	service.ResetDashboardBlocks()
	data := func(v any, err error) func(context.Context) (any, error) {
		return func(context.Context) (any, error) { return v, err }
	}
	service.RegisterDashboardBlock(service.DashboardBlock{Name: "log", Module: "action_log", Title: "Recent actions", Side: true, Permission: "view action log", Data: data("rows", nil)})
	service.RegisterDashboardBlock(service.DashboardBlock{Name: "core", Title: "Core", Data: data(1, nil)})
	service.RegisterDashboardBlock(service.DashboardBlock{Name: "broken", Title: "Broken", Data: data(nil, context.Canceled)})
	service.RegisterDashboardBlock(service.DashboardBlock{Name: "off", Module: "polls", Title: "Off"})
	service.SetModules(map[string]bool{"action_log": true, "polls": false})
	t.Cleanup(func() {
		all := map[string]bool{}
		for _, m := range entity.Modules {
			all[m.Name] = true
		}
		service.SetModules(all)
	})
	c := testController(interfaces.BaseDeps{}, nil)
	main, side := c.dashboardModuleBlocks(context.Background(), func(p string) bool { return p == "view action log" })
	if len(main) != 1 || main[0].Name != "core" || len(side) != 1 || side[0].Data != "rows" || side[0].Title != "Recent actions" {
		t.Errorf("blocks: main %+v side %+v", main, side)
	}
	if _, side := c.dashboardModuleBlocks(context.Background(), func(string) bool { return false }); len(side) != 0 {
		t.Error("a block needing a permission shown without it")
	}
}

// TestDashboardPage - the figures as links, what waits (a count's own
// label over its item's), the recent changes, today's popular (with the
// link to the report), the
// latest comments, a module's block in its card; "nothing waits".
func TestDashboardPage(t *testing.T) {
	at := time.Date(2026, 10, 26, 9, 30, 0, 0, time.UTC)
	data := map[string]any{
		"tiles": []dashboardTile{{Label: "Content", Value: 8214, Note: "+12 published in 7 days", URL: "/admin/content"}},
		"waiting": []entity.AdminMenuItem{
			{Label: "Pending moderation", Path: "/admin/moderation", Count: 3},
			{Label: "Redirects", Path: "/admin/config/redirects", Count: 37, CountLabel: "Not found (404)"},
		},
		"showChanges": true,
		"changes": []service.RecentRow{{RecentRevision: entity.RecentRevision{NodeRevision: entity.NodeRevision{CreatedAt: at, User: &entity.User{Username: "ann", Slug: "ann"}},
			NodeTitle: "Darkhold", NodeURL: "/news/darkhold"}, Summary: service.RevisionSummary{Kind: entity.RevisionEdit, Labels: []string{"Body"}}}},
		"popular":    []entity.PopularNode{{Node: entity.Node{ID: 3, Type: "news", Title: "Wolverine", Slug: "wolverine"}, Views: 412}},
		"comments":   entity.Comments{{ID: 9, Content: "Great!", CreatedAt: at, Node: &entity.Node{ID: 3, Title: "Wolverine", Slug: "wolverine"}, Author: &entity.User{Username: "bob", Slug: "bob"}}},
		"sideBlocks": []dashboardBlock{{DashboardBlock: service.DashboardBlock{Name: "test_block", Link: "/log"}, Title: "Recent actions", LinkLabel: "Action log"}},
	}
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	if err := ts.RenderPageBlock(&buf, "admin/dashboard.html", "content", data); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		`<a href="/admin/content" class="admin-dash-tile">`, `<span class="admin-dash-tile-value">8214</span>`,
		`<a href="/admin/moderation" class="admin-dash-row admin-dash-waiting"> <span class="admin-dash-grow">Pending moderation</span> <span class="admin-nav-count">3</span>`,
		`<span class="admin-dash-grow">Not found (404)<span class="admin-dash-sub">Redirects</span></span>`,
		`<a href="/news/darkhold" class="admin-dash-title">Darkhold</a>`, `<a href="/users/ann">ann</a>`, `href="/admin/content/changes"`,
		`<a href="/wolverine" class="admin-dash-grow admin-dash-title">Wolverine</a> <span class="admin-dash-num">412</span>`,
		`<h2 id="dash-popular">Popular today</h2><a href="/admin/reports/popular">All</a>`,
		`<a href="/wolverine#comment-9">Great!</a>`, `<a href="/users/bob">bob</a>`,
		`<h2 id="dash-test_block">Recent actions</h2><a href="/log">Action log</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	buf.Reset()
	if err := ts.RenderPageBlock(&buf, "admin/dashboard.html", "content", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "Nothing is waiting.") || strings.Contains(out, "dash-changes") || strings.Contains(out, "dash-popular") {
		t.Errorf("an empty Overview: %s", out)
	}
}
