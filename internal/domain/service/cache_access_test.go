package service

import (
	"context"
	"log/slog"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// accessComments - the CommentsService's GetLatest: the comments of
// every type but those ctx leaves out (as the store's interceptor does).
type accessComments struct {
	CommentsService
	all   entity.Comments
	calls int
}

func (f *accessComments) GetLatest(ctx context.Context, _ int, _ int8) (entity.Comments, error) {
	f.calls++
	hidden := entity.HiddenNodeTypes(ctx)
	var out entity.Comments
	for _, c := range f.all {
		if !slices.Contains(hidden, c.Node.Type.ToString()) {
			out = append(out, c)
		}
	}
	return out, nil
}

// membersClosed - a guest's access with the type "members" closed.
var membersClosed = entity.ContentVisitor{HiddenTypes: []string{"members"}}

// commentIDs - the IDs of comments, in order.
func commentIDs(comments entity.Comments) []int {
	var ids []int
	for _, c := range comments {
		ids = append(ids, c.ID)
	}
	return ids
}

// TestCacheAccessVariants - with a type closed to guests the shared data
// is a guest's; a visitor who may view it gets the latest comments and a
// plugin block's data loaded as theirs, once per set of closed types;
// a visitor with a guest's closed types and a ctx of no visitor get the
// shared data.
func TestCacheAccessVariants(t *testing.T) {
	comments := &accessComments{all: entity.Comments{
		{ID: 1, Node: &entity.Node{Type: "article"}},
		{ID: 2, Node: &entity.Node{Type: "members"}},
	}}
	s := &CacheService{commentsService: comments, itemsLimit: 10, logger: slog.New(slog.DiscardHandler)}
	s.SetGuestVisitor(func(context.Context) (entity.ContentVisitor, error) { return membersClosed, nil })
	if err := s.update(context.Background()); err != nil {
		t.Fatal(err)
	}
	data := s.GetSidebarData()
	if got := commentIDs(data.LatestComments); !slices.Equal(got, []int{1}) || !data.commentsDependent {
		t.Fatalf("shared: %v, dependent %v", got, data.commentsDependent)
	}

	// A plugin block counting what its visitor may view.
	id := entity.PluginBlockID("access_test")
	SetPluginBlockData(id, func(ctx context.Context, _ entity.BlockPlacementDef) (any, error) {
		return len(entity.HiddenNodeTypes(ctx)), nil
	})
	placement := entity.BlockWithPlacement{Placement: entity.BlockPlacementDef{ID: 9, PlacementType: entity.BlockPlacementPlugin, PluginID: &id}}
	placements := s.resolvePluginBlockData(entity.WithContentVisitor(context.Background(), membersClosed), map[entity.Region][]entity.BlockWithPlacement{"sidebar": {placement}})
	data.BlockPlacements = placements
	data.pluginDependent = s.pluginAccessDependent(entity.WithAllNodeTypes(context.Background()), placements)
	if !data.pluginDependent[9] {
		t.Fatal("the plugin block's data isn't access-dependent")
	}
	s.mu.Lock()
	s.data = data
	s.mu.Unlock()

	member := entity.WithContentVisitor(context.Background(), entity.ContentVisitor{})
	before := comments.calls
	for range 2 {
		got := s.SidebarDataFor(member)
		if ids := commentIDs(got.LatestComments); !slices.Equal(ids, []int{1, 2}) {
			t.Errorf("member's comments: %v", ids)
		}
		if pd := got.BlockPlacements["sidebar"][0].PluginData; pd != 0 {
			t.Errorf("member's plugin data: %v", pd)
		}
	}
	if comments.calls-before != 1 {
		t.Errorf("the member's variant loaded %d times, want once", comments.calls-before)
	}
	if pd := s.GetSidebarData().BlockPlacements["sidebar"][0].PluginData; pd != 1 {
		t.Errorf("the shared plugin data changed: %v", pd)
	}
	for name, ctx := range map[string]context.Context{
		"guest":      entity.WithContentVisitor(context.Background(), membersClosed),
		"no visitor": context.Background(),
	} {
		got := s.SidebarDataFor(ctx)
		if ids := commentIDs(got.LatestComments); !slices.Equal(ids, []int{1}) {
			t.Errorf("%s: comments %v", name, ids)
		}
		if pd := got.BlockPlacements["sidebar"][0].PluginData; pd != 1 {
			t.Errorf("%s: plugin data %v", name, pd)
		}
	}

	// A refresh drops the variants.
	if err := s.update(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(s.accessData.entries) != 0 {
		t.Errorf("variants after a refresh: %d", len(s.accessData.entries))
	}
}

// TestCacheNoAccessVariants - with no type closed to guests nothing is
// probed: every visitor gets the shared data.
func TestCacheNoAccessVariants(t *testing.T) {
	comments := &accessComments{all: entity.Comments{{ID: 1, Node: &entity.Node{Type: "article"}}}}
	s := &CacheService{commentsService: comments, itemsLimit: 10, logger: slog.New(slog.DiscardHandler)}
	s.SetGuestVisitor(func(context.Context) (entity.ContentVisitor, error) { return entity.ContentVisitor{}, nil })
	if err := s.update(context.Background()); err != nil {
		t.Fatal(err)
	}
	if comments.calls != 1 {
		t.Errorf("GetLatest called %d times, want once (no probe)", comments.calls)
	}
	s.SidebarDataFor(entity.WithContentVisitor(context.Background(), entity.ContentVisitor{}))
	if comments.calls != 1 || len(s.accessData.entries) != 0 {
		t.Errorf("a variant without closed types: %d calls, %d variants", comments.calls, len(s.accessData.entries))
	}
}
