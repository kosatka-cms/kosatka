package galleries

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestOverviewTypeLabels - the overview's "Type" filter and row show
// every node type by its label, a module's type too (as the core's content
// list does), the chosen one selected.
func TestOverviewTypeLabels(t *testing.T) {
	testkit.UseEngineRoot(t)
	withModule(t, true)
	labels := map[string]string{"article": "Article", "forum": "Forum topic"}
	out := testkit.RenderPageBlock(t, "admin/galleries.html", "content", map[string]any{
		"contentTypes": modulekit.NodeTypesByLabel(labels),
		"typeLabels":   labels,
		"filterType":   "forum",
		"rows":         []overviewRow{{Node: modulekit.Node{ID: 3, Title: "Welcome", Type: "forum"}}},
		"pagination":   pagination.New(1, 1, overviewPerPage, overviewPath, nil),
	})
	for _, want := range []string{
		`<option value="article" >Article</option>`,
		`<option value="forum" selected>Forum topic</option>`,
		`<span class="admin-table-sub">Forum topic</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in:\n%s", want, out)
		}
	}
}
