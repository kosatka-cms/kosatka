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

// TestCommentsInPage - with the comments in the page's data
// (commentsList: the comment settings' "with the page") the section shows
// them, marked loaded, without the loading skeleton; without them the
// skeleton waits for ajax; a module showing the comments (commentsModule)
// replaces the section; a type hiding empty comments shows nothing while
// the page has none; an item previewed from its form, nothing.
func TestCommentsInPage(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	render := func(data map[string]any) string {
		t.Helper()
		var buf strings.Builder
		if err := ts.RenderPageBlock(&buf, "dynamic/page.html", "comments-placeholder", data); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	item := &entity.DynamicContent{Node: entity.Node{ID: 7, Type: "article", Slug: "articles/harbour", CommentMode: entity.CommentModeOpen}}
	comment := entity.Comment{ID: 3, Nid: 7, Content: "Fair winds to the harbour", CreatedAt: time.Now(), Author: &entity.User{ID: 2, Username: "sailor", Slug: "sailor"}}
	list := func(comments entity.Comments) map[string]any {
		return map[string]any{"comments": comments, "commentMode": "open",
			"pagination": pagination.NewWithAnchor(1, len(comments), entity.CommentsPerPage, "", nil, "comments")}
	}

	out := render(map[string]any{"item": item, "commentsList": list(entity.Comments{comment})})
	for _, want := range []string{`data-loaded="1"`, "comments-loaded", "Fair winds to the harbour", `id="comment-3"`} {
		if !strings.Contains(out, want) {
			t.Errorf("with the page: no %q in %s", want, out)
		}
	}
	if strings.Contains(out, "comments-loading") {
		t.Error("with the page: the loading skeleton shown")
	}

	out = render(map[string]any{"item": item})
	if !strings.Contains(out, "comments-loading") || strings.Contains(out, "data-loaded") {
		t.Errorf("by ajax: %s", out)
	}

	// A module's comments in place of the core's: its comments_<module>
	// (none here: an unknown module), never the core's section.
	if out := render(map[string]any{"item": item, "commentsModule": "example", "commentsList": list(entity.Comments{comment})}); strings.Contains(out, `id="comments"`) || strings.Contains(out, "Fair winds") {
		t.Errorf("a module's comments: the core's shown: %s", out)
	}

	hiding := &entity.ContentTypeDef{MachineName: "article", HideEmptyComments: true}
	if out := render(map[string]any{"item": item, "contentType": hiding, "commentsList": list(nil)}); strings.Contains(out, `id="comments"`) || strings.Contains(out, "auth-notice") {
		t.Errorf("empty, the type hiding empty comments: %s", out)
	}
	if out := render(map[string]any{"item": item, "contentType": hiding, "commentsList": list(entity.Comments{comment})}); !strings.Contains(out, `id="comments"`) {
		t.Error("a type hiding empty comments hides them when there are some")
	}

	// Previewed from the form (a new item: no ID): no section to load
	// comments into, no form to post one.
	draft := &entity.DynamicContent{Node: entity.Node{Type: "article", CommentMode: entity.CommentModeOpen}}
	for _, it := range []*entity.DynamicContent{draft, item} {
		if out := render(map[string]any{"item": it, "unsavedPreview": &UnsavedPreview{}}); strings.TrimSpace(out) != "" {
			t.Errorf("a preview of node %d: %s", it.Node.ID, out)
		}
	}
}
