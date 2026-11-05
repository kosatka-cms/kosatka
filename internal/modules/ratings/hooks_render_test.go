package ratings

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/ratings/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestTypeFormSection - the content type form shows the module's section
// (the switch, the votes so far) while it is on, nothing while off.
func TestTypeFormSection(t *testing.T) {
	testkit.UseEngineRoot(t)
	data := map[string]any{
		"contentType":    &modulekit.ContentTypeDef{ID: 4, MachineName: "article", Label: "Article"},
		"moduleSettings": map[string]any{manifest.Name: typeForm{Rated: true, Votes: 12, Nodes: 3}},
	}
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	out := testkit.RenderPageBlock(t, "admin/content-types/edit.html", "content", data)
	for _, want := range []string{`id="ratings_rated" name="ratings_rated" value="1" class="form-check-input" checked`, "12 votes · 3 content items", `href="/admin/config/ratings"`} {
		if !strings.Contains(out, want) {
			t.Errorf("content type form lacks %s", want)
		}
	}
	testkit.WithModules(t, map[string]bool{manifest.Name: false})
	if out := testkit.RenderPageBlock(t, "admin/content-types/edit.html", "content", data); strings.Contains(out, "ratings_rated") {
		t.Error("the section while the module is off")
	}
}

// TestWidgetRender - the widget: the total, a star button per score (the
// voter's lit), taking it back; a guest who may not rate is offered to
// log in; no votes - said so.
func TestWidgetRender(t *testing.T) {
	testkit.UseEngineRoot(t)
	w := Widget{NodeID: 7, NodeURL: "/article/sea", Scale: 5, Count: 23, Average: "4.6", Fill: 92, Stars: []int{1, 2, 3, 4, 5}, Mine: 4, CanVote: true, CanWithdraw: true}
	out := testkit.RenderNamed(t, "ratings_widget", w, widgetTemplates)
	for _, want := range []string{
		`<span class="rating-average">4.6</span>`, "23 votes", `style="width: 92.0%"`,
		`action="/node/7/rating"`, `name="score" value="5"`, `class="rating-star is-on" aria-label="4 stars" aria-pressed="true"`,
		`name="withdraw" value="1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("widget lacks %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, `value="5" class="rating-star is-on"`) {
		t.Error("a star above the vote is lit")
	}

	guest := Widget{NodeID: 7, NodeURL: "/article/sea", Scale: 5, Stars: []int{1, 2, 3, 4, 5}, LogIn: true}
	out = testkit.RenderNamed(t, "ratings_widget", guest, widgetTemplates)
	if !strings.Contains(out, "No votes yet") || !strings.Contains(out, `href="/login?destination=%2Farticle%2Fsea"`) || strings.Contains(out, "<form") {
		t.Errorf("a guest's widget:\n%s", out)
	}
}

// TestSummaryRender - the View field: the stars with the average and the
// count, or one of them.
func TestSummaryRender(t *testing.T) {
	testkit.UseEngineRoot(t)
	sum := Summary{Scale: 5, Count: 23, Average: "4.6", Fill: 92}
	for look, want := range map[string]string{
		lookFull:    `<span class="rating-average">4.6</span> <span class="rating-count">· 23</span>`,
		lookAverage: `<span class="rating-summary"><span class="rating-average">4.6</span></span>`,
		lookCount:   `<span class="rating-summary"><span class="rating-count">23 votes</span></span>`,
	} {
		sum.Look = look
		if out := testkit.RenderNamed(t, "ratings_summary", sum, widgetTemplates); !strings.Contains(out, want) {
			t.Errorf("%s: lacks %s:\n%s", look, want, out)
		}
	}
}

// TestNodeRatingBlock - the block on a rated content page: the total and
// the endpoint the widget loads from; nothing on another page.
func TestNodeRatingBlock(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	id := manifest.PluginBlockNodeRating
	placement := modulekit.BlockPlacement{ID: 1, PlacementType: modulekit.BlockPlacementPlugin, PluginID: &id, Region: "content_inline", VisibilityScope: modulekit.BlockVisibilityAll}
	page := map[string]any{"moduleNode": map[string]any{manifest.Name: nodePage{NodeID: 7, Summary: Summary{Scale: 5, Count: 2, Average: "4.5", Fill: 90}}}}
	out := testkit.RenderPlacement(t, placement, page)
	for _, want := range []string{`data-rating-endpoint="/node/7/rating"`, `<span class="rating-average">4.5</span>`, "2 votes"} {
		if !strings.Contains(out, want) {
			t.Errorf("block lacks %s:\n%s", want, out)
		}
	}
	if out := testkit.RenderPlacement(t, placement, map[string]any{}); strings.Contains(out, "rating-block") {
		t.Errorf("the block on a page without a rating:\n%s", out)
	}
}
