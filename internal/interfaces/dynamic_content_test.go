package interfaces

import (
	"context"
	"net/http"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
)

func intPtr(v int) *int { return &v }

// noTerms - a TaxonomyService with no terms (only TermsByIDs is called).
type noTerms struct{ service.TaxonomyService }

func (noTerms) TermsByIDs(context.Context, []int) (map[int]entity.TaxonomyTerm, error) {
	return map[int]entity.TaxonomyTerm{}, nil
}

// TestPageDataPathPattern - the form shows the type's URL pattern next
// to "generate automatically", the default one for a type without its own.
func TestPageDataPathPattern(t *testing.T) {
	c := &DynamicContentController{taxonomyService: noTerms{}}
	for pattern, want := range map[string]string{"": service.DefaultNodePathPattern, "blog/[node:title]": "blog/[node:title]"} {
		data := c.pageData(context.Background(), entity.ContentTypeDef{MachineName: "news", PathPattern: pattern}, nil, form.DynamicForm{}, entity.DynamicContent{})
		if data["pathPattern"] != want {
			t.Errorf("%q: pathPattern = %v", pattern, data["pathPattern"])
		}
	}
}

// TestNodeSlug - a new node's slug follows the pattern unless one is typed
// with "automatically" off; an edited node keeps its automatic address
// (renamed, it stays) and its own setting when the form has no checkbox
// (URL settings hidden from the editor), takes a typed slug when
// "automatically" is off and keeps its own when none is typed; one set by
// hand that is made automatic follows the pattern at once.
func TestNodeSlug(t *testing.T) {
	c := &DynamicContentController{paths: service.NewNodePathService(noTerms{}, slugsTaken{taken: map[string]bool{}})}
	def := entity.ContentTypeDef{MachineName: "news"}
	node := func(id int, slug string, auto bool) *entity.DynamicContent {
		return &entity.DynamicContent{Node: entity.Node{ID: id, Title: "Hello world", Slug: slug, PathAuto: auto}}
	}
	for _, tc := range []struct {
		name     string
		f        form.DynamicForm
		e        *entity.DynamicContent
		want     string
		wantAuto bool
	}{
		{"new, automatic", form.DynamicForm{PathAutoShown: true, PathAuto: true}, node(0, "", false), "news/hello-world", true},
		{"new, typed", form.DynamicForm{PathAutoShown: true, Slug: "/my/page/"}, node(0, "", false), "my/page", false},
		{"new, nothing typed", form.DynamicForm{PathAutoShown: true}, node(0, "", false), "news/hello-world", true},
		{"new, no URL settings", form.DynamicForm{Slug: "ignored"}, node(0, "", false), "news/hello-world", true},
		{"edited, automatic", form.DynamicForm{PathAutoShown: true, PathAuto: true}, node(7, "news/old", true), "news/old", true},
		{"edited, by hand made automatic", form.DynamicForm{PathAutoShown: true, PathAuto: true}, node(7, "news/old", false), "news/hello-world", true},
		{"edited, typed", form.DynamicForm{PathAutoShown: true, Slug: "news/new"}, node(7, "news/old", true), "news/new", false},
		{"edited, nothing typed", form.DynamicForm{PathAutoShown: true}, node(7, "news/old", true), "news/old", false},
		{"edited, no URL settings, automatic node", form.DynamicForm{}, node(7, "news/old", true), "news/old", true},
		{"edited, no URL settings, node by hand", form.DynamicForm{}, node(7, "news/old", false), "news/old", false},
	} {
		got, err := c.nodeSlug(context.Background(), def, &tc.f, tc.e)
		if err != nil || got != tc.want || tc.e.Node.PathAuto != tc.wantAuto {
			t.Errorf("%s: %q auto=%v %v, want %q auto=%v", tc.name, got, tc.e.Node.PathAuto, err, tc.want, tc.wantAuto)
		}
	}
}

// slugsTaken - a NodesRepository knowing which slugs other nodes have.
type slugsTaken struct {
	entity.NodesRepository
	taken map[string]bool
}

func (s slugsTaken) SlugExistsExcludingNode(_ context.Context, slug string, _ int) (bool, error) {
	return s.taken[slug], nil
}

type referenceNodes struct{ entity.NodesRepository }

func (referenceNodes) ByIDs(_ context.Context, ids []int) (map[int]entity.Node, error) {
	out := map[int]entity.Node{}
	for _, id := range ids {
		if id != 404 {
			out[id] = entity.Node{ID: id, Title: "Node"}
		}
	}
	return out, nil
}

// TestErrorPageType - a node configured as the 404 page shows no author and
// dates line whatever its type shows; another node of the type keeps it,
// and the type's own settings aren't changed.
func TestErrorPageType(t *testing.T) {
	service.SetErrorPages(http.StatusNotFound, []int{7})
	t.Cleanup(func() { service.SetErrorPages(http.StatusNotFound, nil) })
	def := &entity.ContentTypeDef{MachineName: "page", ShowSubmitted: true}

	shown, status := errorPageType(def, 7)
	if status != http.StatusNotFound || shown.ShowSubmitted {
		t.Errorf("error page: status %d, submitted %v", status, shown.ShowSubmitted)
	}
	if !def.ShowSubmitted {
		t.Error("the type's settings changed")
	}
	if shown, status := errorPageType(def, 8); status != 0 || shown != def {
		t.Errorf("another node: %d %v", status, shown)
	}
}

// countingComments - a CommentsService that counts the published
// comments of node 1 only (only CountByNodeID is called).
type countingComments struct{ service.CommentsService }

func (countingComments) CountByNodeID(_ context.Context, nid int, status int8, _ bool) (int, error) {
	if nid == 1 && status == entity.CommentPublished {
		return 4, nil
	}
	return 0, nil
}

// TestNodeCommentCount - a content page has its comment count while the
// core shows its comments: not when they are hidden or a module's are
// shown in their place.
func TestNodeCommentCount(t *testing.T) {
	c := &DynamicContentController{commentsService: countingComments{}}
	open := entity.Node{ID: 1, CommentMode: entity.CommentModeOpen}
	if n, ok := c.nodeCommentCount(t.Context(), open, ""); !ok || n != 4 {
		t.Errorf("open: %d %v, want 4 true", n, ok)
	}
	hidden := open
	hidden.CommentMode = entity.CommentModeHidden
	if _, ok := c.nodeCommentCount(t.Context(), hidden, ""); ok {
		t.Error("hidden comments are counted")
	}
	if _, ok := c.nodeCommentCount(t.Context(), open, "othercomments"); ok {
		t.Error("a module's comments in place of the core's are counted")
	}
}
