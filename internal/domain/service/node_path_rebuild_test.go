package service

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// fakePathNodes - the nodes a rebuild walks, by ID; their slugs are the
// taken ones.
type fakePathNodes map[int]*entity.Node

func (f fakePathNodes) PathBatch(_ context.Context, nodeType string, afterID, limit int, onlyAuto bool) (entity.Nodes, error) {
	var ids []int
	for id, n := range f {
		if string(n.Type) == nodeType && id > afterID && (!onlyAuto || n.PathAuto) {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	var out entity.Nodes
	for _, id := range ids {
		if len(out) == limit {
			break
		}
		out = append(out, *f[id])
	}
	return out, nil
}

func (f fakePathNodes) CountPathNodes(ctx context.Context, nodeType string, onlyAuto bool) (int, error) {
	nodes, err := f.PathBatch(ctx, nodeType, 0, len(f), onlyAuto)
	return len(nodes), err
}

func (f fakePathNodes) SetPath(_ context.Context, nodeID int, slug string, auto bool) error {
	f[nodeID].Slug, f[nodeID].PathAuto = slug, auto
	return nil
}

func (f fakePathNodes) SlugExistsExcludingNode(_ context.Context, slug string, excludeNodeID int) (bool, error) {
	for id, n := range f {
		if n.Slug == slug && id != excludeNodeID {
			return true, nil
		}
	}
	return false, nil
}

type fakePathMoves []entity.Redirect

func (f *fakePathMoves) MoveSlugs(_ context.Context, moves []entity.Redirect) error {
	*f = append(*f, moves...)
	return nil
}

type noPathFieldValues struct{}

func (noPathFieldValues) FieldTaxonomyValuesForNodes(context.Context, int, []int) (map[int]int, error) {
	return nil, nil
}

func (noPathFieldValues) FieldTaxonomyMultiValuesForNodes(context.Context, int, []int) (map[int][]int, error) {
	return nil, nil
}

// TestNodePathRebuild - a preview counts and shows the changes, writing
// nothing; a rebuild moves the automatic addresses (the old ones
// redirecting) and leaves those set by hand, unless asked to include them -
// then they follow the pattern; a step past its deadline stops, to be
// continued.
func TestNodePathRebuild(t *testing.T) {
	ctx := context.Background()
	newNodes := func() fakePathNodes {
		return fakePathNodes{
			1: {ID: 1, Type: "news", Title: "Alpha", Slug: "news-old/alpha", PathAuto: true},
			2: {ID: 2, Type: "news", Title: "Beta", Slug: "custom-beta"},
			3: {ID: 3, Type: "news", Title: "Gamma", Slug: "news/gamma", PathAuto: true},
			4: {ID: 4, Type: "page", Title: "Other", Slug: "page/other", PathAuto: true},
		}
	}
	def := entity.ContentTypeDef{MachineName: "news", PathPattern: "news/[node:title]"}
	rebuilder := func(nodes fakePathNodes, moves *fakePathMoves) *NodePathRebuilder {
		return NewNodePathRebuilder(NewNodePathService(fakePathTerms{}, nodes), nodes, noPathFieldValues{}, moves)
	}
	later := time.Now().Add(time.Minute)

	nodes, moves := newNodes(), &fakePathMoves{}
	res, err := rebuilder(nodes, moves).Step(ctx, NodePathRebuild{Type: def, DryRun: true}, later)
	if err != nil || res.Checked != 2 || res.Changed != 1 || res.More ||
		!reflect.DeepEqual(res.Samples, []NodePathChange{{NodeID: 1, Title: "Alpha", From: "news-old/alpha", To: "news/alpha"}}) {
		t.Errorf("preview: %+v %v", res, err)
	}
	if nodes[1].Slug != "news-old/alpha" || len(*moves) != 0 {
		t.Errorf("a preview wrote: %q %v", nodes[1].Slug, *moves)
	}

	if res, err := rebuilder(nodes, moves).Step(ctx, NodePathRebuild{Type: def}, later); err != nil || res.Changed != 1 || res.Last != 3 {
		t.Errorf("rebuild: %+v %v", res, err)
	}
	if nodes[1].Slug != "news/alpha" || nodes[2].Slug != "custom-beta" || nodes[2].PathAuto || nodes[4].Slug != "page/other" ||
		!reflect.DeepEqual(*moves, fakePathMoves{{FromPath: "news-old/alpha", ToPath: "news/alpha"}}) {
		t.Errorf("rebuilt: %+v %+v %v", nodes[1], nodes[2], *moves)
	}

	if res, err := rebuilder(nodes, moves).Step(ctx, NodePathRebuild{Type: def, IncludeManual: true}, later); err != nil || res.Checked != 3 || res.Changed != 1 {
		t.Errorf("with the manual ones: %+v %v", res, err)
	}
	if nodes[2].Slug != "news/beta" || !nodes[2].PathAuto {
		t.Errorf("a manual address: %+v", nodes[2])
	}

	nodes = newNodes()
	res, err = rebuilder(nodes, &fakePathMoves{}).Step(ctx, NodePathRebuild{Type: def}, time.Now().Add(-time.Second))
	if err != nil || !res.More || res.Checked != 0 || nodes[1].Slug != "news-old/alpha" {
		t.Errorf("past the deadline: %+v %v", res, err)
	}
}
