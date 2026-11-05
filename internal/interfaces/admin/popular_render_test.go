package admin

import (
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestPopularPage - the periods' tabs (the current one active, the type
// kept), the type filter, the rows with their place, type label, views in
// the period and in all; the notes when views aren't counted or the views
// by day are kept shorter than the period; none without rows.
func TestPopularPage(t *testing.T) {
	render := func(data map[string]any) string {
		t.Helper()
		var buf strings.Builder
		ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
		if err := ts.RenderPageBlock(&buf, "admin/reports/popular.html", "content", data); err != nil {
			t.Fatal(err)
		}
		return strings.Join(strings.Fields(buf.String()), " ")
	}
	q := url.Values{"period": {"month"}, "type": {"news"}}
	out := render(map[string]any{
		"period":       "month",
		"periods":      entity.PopularPeriods,
		"type":         "news",
		"contentTypes": entity.ContentTypeDefs{{MachineName: "news", Label: "News"}, {MachineName: "page", Label: "Page"}},
		"typeLabels":   map[string]string{"news": "News"},
		"rows":         []entity.PopularNode{{Node: entity.Node{ID: 3, Type: "news", Title: "Wolverine", Slug: "wolverine"}, Views: 412, Total: 9000}},
		"rankFrom":     51,
		"pagination":   pagination.New(2, 51, popularPerPage, popularPath, q),
		"counting":     false,
		"keepDays":     7,
	})
	for _, want := range []string{
		`<a class="nav-link" href="/admin/reports/popular?period=today&amp;type=news">Today</a>`,
		`<a class="nav-link active" aria-current="page" href="/admin/reports/popular?period=month&amp;type=news">30 days</a>`,
		`<input type="hidden" name="period" value="month">`, `<option value="news" selected>News</option>`,
		`<th class="text-end">Views, 30 days</th>`,
		`<td class="text-end admin-table-muted">51</td>`, `<a href="/wolverine" class="admin-table-title">Wolverine</a> <span class="admin-table-sub">News</span>`,
		`<td class="text-end text-nowrap">412</td>`, `<td class="text-end text-nowrap admin-table-muted">9000</td>`,
		`Views aren't counted now`, `Views by day are kept for 7 days`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	out = render(map[string]any{"period": "today", "periods": entity.PopularPeriods, "counting": true,
		"pagination": pagination.New(1, 0, popularPerPage, popularPath, url.Values{})})
	if !strings.Contains(out, "No views in this period.") || strings.Contains(out, "counted now") || strings.Contains(out, "are kept for") {
		t.Errorf("an empty report: %s", out)
	}
}
