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
// with "automatically" off; an edited node keeps its own setting when the
// form has no checkbox (URL settings hidden from the editor), follows the
// pattern when it is on, keeps or takes the typed slug when it is off.
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
		{"edited, automatic", form.DynamicForm{PathAutoShown: true, PathAuto: true}, node(7, "news/old", false), "news/hello-world", true},
		{"edited, typed", form.DynamicForm{PathAutoShown: true, Slug: "news/new"}, node(7, "news/old", true), "news/new", false},
		{"edited, no URL settings, automatic node", form.DynamicForm{}, node(7, "news/old", true), "news/hello-world", true},
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
