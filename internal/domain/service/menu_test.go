package service

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestResolveMenuBlockRoot covers the "initial visibility level" descent.
// Trees are hand-built with ActiveTrail pre-set, bypassing
// AnnotateActiveMenuTree, to test the descent logic in isolation.
func TestResolveMenuBlockRoot(t *testing.T) {
	t.Run("startLevel 1 is identity", func(t *testing.T) {
		tree := []MenuTreeItem{{Title: "A"}, {Title: "B"}}
		got := ResolveMenuBlockRoot(tree, 1)
		if len(got) != 2 || got[0].Title != "A" || got[1].Title != "B" {
			t.Fatalf("expected tree unchanged, got %+v", got)
		}
	})

	t.Run("startLevel 2 descends into the active-trail branch", func(t *testing.T) {
		tree := []MenuTreeItem{
			{Title: "A"},
			{
				Title:       "B",
				ActiveTrail: true,
				Children: []MenuTreeItem{
					{Title: "B1"},
					{Title: "B2"},
				},
			},
		}
		got := ResolveMenuBlockRoot(tree, 2)
		if len(got) != 2 || got[0].Title != "B1" || got[1].Title != "B2" {
			t.Fatalf("expected B's children, got %+v", got)
		}
	})

	t.Run("startLevel 3 descends two levels deep", func(t *testing.T) {
		tree := []MenuTreeItem{
			{
				Title:       "A",
				ActiveTrail: true,
				Children: []MenuTreeItem{
					{Title: "A1"},
					{
						Title:       "A2",
						ActiveTrail: true,
						Children: []MenuTreeItem{
							{Title: "A2a"},
						},
					},
				},
			},
		}
		got := ResolveMenuBlockRoot(tree, 3)
		if len(got) != 1 || got[0].Title != "A2a" {
			t.Fatalf("expected A2's children, got %+v", got)
		}
	})

	t.Run("startLevel 2 with no active-trail match returns nil, not an error", func(t *testing.T) {
		tree := []MenuTreeItem{
			{Title: "A"},
			{Title: "B", Children: []MenuTreeItem{{Title: "B1"}}},
		}
		got := ResolveMenuBlockRoot(tree, 2)
		if got != nil {
			t.Fatalf("expected nil (current page outside every branch), got %+v", got)
		}
	})

	t.Run("startLevel 2 on an empty tree returns nil, no panic", func(t *testing.T) {
		got := ResolveMenuBlockRoot(nil, 2)
		if got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})
}

type fakeTreeMenus struct {
	entity.MenuRepository
	links entity.MenuLinkDefs
}

func (f fakeTreeMenus) MenuByMachineName(context.Context, string) (*entity.MenuDef, error) {
	return &entity.MenuDef{ID: 1}, nil
}

func (f fakeTreeMenus) MenuLinksByMenuID(context.Context, int) (entity.MenuLinkDefs, error) {
	return f.links, nil
}

type fakeTreeNodes struct {
	entity.NodesRepository
	calls int
}

func (f *fakeTreeNodes) URLsByIDs(_ context.Context, ids []int) (map[int]string, error) {
	f.calls++
	out := map[int]string{}
	for _, id := range ids {
		if id != 404 {
			out[id] = fmt.Sprintf("/article/%d", id)
		}
	}
	return out, nil
}

type fakeTreeTerms struct {
	entity.TaxonomyRepository
	calls int
}

func (f *fakeTreeTerms) TermsByIDs(_ context.Context, ids []int) (map[int]entity.TaxonomyTerm, error) {
	f.calls++
	return map[int]entity.TaxonomyTerm{7: {ID: 7, Slug: "tags/whales"}}, nil
}

// TestResolvedTree - a menu's links with their current URLs, the nodes'
// and the terms' in one query each; a link to what's gone is broken; a
// disabled link is left out with its children.
func TestResolvedTree(t *testing.T) {
	id := func(v int) *int { return &v }
	nodes, terms := &fakeTreeNodes{}, &fakeTreeTerms{}
	links := entity.MenuLinkDefs{
		{ID: 1, Title: "Whale", LinkType: entity.MenuLinkNode, LinkNodeID: id(5), Enabled: true, Weight: 1},
		{ID: 2, Title: "Gone", LinkType: entity.MenuLinkNode, LinkNodeID: id(404), Enabled: true, Weight: 2},
		{ID: 3, Title: "Whales", LinkType: entity.MenuLinkTerm, LinkTermID: id(7), Enabled: true, Weight: 3},
		{ID: 4, Title: "Off", LinkType: entity.MenuLinkURL, LinkURL: "/off", Enabled: false, Weight: 4},
		{ID: 5, Title: "Under off", LinkType: entity.MenuLinkURL, LinkURL: "/x", Enabled: true, ParentID: id(4)},
		{ID: 6, Title: "Ship", LinkType: entity.MenuLinkNode, LinkNodeID: id(6), Enabled: true, ParentID: id(1)},
	}
	s := NewMenuService(fakeTreeMenus{links: links}, nodes, terms, nil, nil, slog.New(slog.DiscardHandler))
	tree, err := s.ResolvedTree(t.Context(), "main")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, item := range tree {
		got = append(got, fmt.Sprintf("%s=%s/%v", item.Title, item.URL, item.Broken))
		for _, child := range item.Children {
			got = append(got, fmt.Sprintf("  %s=%s", child.Title, child.URL))
		}
	}
	if want := "[Whale=/article/5/false   Ship=/article/6 Gone=/true Whales=/tags/whales/false]"; fmt.Sprint(got) != want {
		t.Errorf("tree %v, want %s", got, want)
	}
	if nodes.calls != 1 || terms.calls != 1 {
		t.Errorf("queries: nodes %d, terms %d - want one each", nodes.calls, terms.calls)
	}
}
