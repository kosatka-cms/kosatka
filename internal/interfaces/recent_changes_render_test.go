package interfaces

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestRecentChangesPage - the types without history named with a link to
// the types; a day's heading; a row: the time, the content (a link) and
// its type, who, what changed (the publishing alone as such), the
// comparison with the version before (none for a first one) and the
// history.
func TestRecentChangesPage(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	edit := service.RecentRow{
		RecentRevision: entity.RecentRevision{NodeRevision: entity.NodeRevision{ID: 7, NodeID: 3, User: &entity.User{Username: "ann", Slug: "ann"}, Kind: entity.RevisionEdit},
			NodeTitle: "Cache <CDN>", NodeURL: "/article/cache", PreviousID: 5},
		Summary: service.RevisionSummary{Kind: entity.RevisionEdit, Labels: []string{"Body", "Tags"}}, TypeLabel: "Article",
	}
	first := service.RecentRow{
		RecentRevision: entity.RecentRevision{NodeRevision: entity.NodeRevision{ID: 4, NodeID: 9, Kind: entity.RevisionCreate}, NodeTitle: "About", NodeURL: "/about"},
		Summary:        service.RevisionSummary{Kind: entity.RevisionCreate}, TypeLabel: "Page",
	}
	unpub := service.RecentRow{
		RecentRevision: entity.RecentRevision{NodeRevision: entity.NodeRevision{ID: 8, NodeID: 3, Kind: entity.RevisionEdit}, NodeTitle: "Cache", NodeURL: "/article/cache", PreviousID: 7},
		Summary:        service.RevisionSummary{Kind: entity.RevisionEdit, Status: "unpublished"}, TypeLabel: "Article",
	}
	data := map[string]any{
		"days": []recentDay{
			{Day: time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), Today: true, Rows: []recentRow{{RecentRow: unpub, Time: "22:14"}, {RecentRow: edit, Time: "21:40"}}},
			{Day: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), Rows: []recentRow{{RecentRow: first, Time: "09:12"}}},
		},
		"total": 3, "filter": entity.RecentRevisionsFilter{Type: "article"},
		"pagination":     pagination.New(1, 3, RecentChangesPerPage, RecentChangesPath, nil),
		"historyTypes":   entity.ContentTypeDefs{{MachineName: "article", Label: "Article"}},
		"noHistoryTypes": entity.ContentTypeDefs{{MachineName: "news", Label: "News"}, {MachineName: "blog", Label: "Blog"}},
	}
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	if err := ts.RenderPageBlock(&buf, "admin/content/changes.html", "content", data); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		"The history of changes is off for 2 types:</strong> News, Blog", `<a href="/admin/config/content-types">`,
		`<option value="article" selected>Article</option>`,
		`<th colspan="5" scope="rowgroup">Today</th>`,
		`<td class="text-nowrap admin-table-muted">21:40</td>`, `<a href="/article/cache" class="admin-table-title">Cache &lt;CDN&gt;</a>`,
		`<span class="admin-table-sub">Article</span>`, `<a href="/users/ann">ann</a>`, "Body, Tags",
		`href="/node/3/revisions/compare?from=5&amp;to=7"`, `href="/node/3/revisions"`,
		"Unpublished", "Content created", "1-3 of 3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if strings.Contains(out, "compare?from=0") || strings.Count(out, "/revisions/compare") != 2 {
		t.Error("a first version has nothing to compare with")
	}
}
