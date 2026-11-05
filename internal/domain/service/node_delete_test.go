package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeDeleteNodesRepo struct {
	entity.NodesRepository
	nodes map[int]entity.Node
}

func (f *fakeDeleteNodesRepo) ByID(_ context.Context, id int) (*entity.Node, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	return &n, nil
}

type fakeNodeDeleteRepo struct {
	deleted []int
	err     error
}

func (f *fakeNodeDeleteRepo) DeleteNode(_ context.Context, id int) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeNodeFileCleaner struct {
	nodeFiles map[int][]int
	uris      map[int]string
	deleted   []int
}

func (f *fakeNodeFileCleaner) NodeFileIDs(_ context.Context, ids []int) ([]int, error) {
	var out []int
	for _, id := range ids {
		out = append(out, f.nodeFiles[id]...)
	}
	return out, nil
}

func (f *fakeNodeFileCleaner) DeleteUnreferencedFiles(_ context.Context, ids []int) ([]string, error) {
	var uris []string
	for _, id := range ids {
		f.deleted = append(f.deleted, id)
		uris = append(uris, f.uris[id])
	}
	return uris, nil
}

func TestNodeDeleteServiceRemovesFilesAndRefreshesCaches(t *testing.T) {
	nodes := &fakeDeleteNodesRepo{nodes: map[int]entity.Node{5: {ID: 5, Type: "events"}}}
	repo := &fakeNodeDeleteRepo{}
	cleaner := &fakeNodeFileCleaner{nodeFiles: map[int][]int{5: {50, 51}}, uris: map[int]string{50: "public://a.jpg", 51: "public://b.jpg"}}
	storage := &fakeFileRemover{}
	refreshed := 0
	s := NewNodeDeleteService(nodes, repo, cleaner, storage, func() { refreshed++ }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.Delete(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	if len(cleaner.deleted) != 2 || len(storage.removed) != 2 || refreshed != 1 {
		t.Fatalf("files deleted=%v removed=%v refreshed=%d", cleaner.deleted, storage.removed, refreshed)
	}

	repo.err = errors.New("db down")
	if err := s.Delete(context.Background(), 5); err == nil {
		t.Fatal("expected error")
	}
	if len(storage.removed) != 2 || refreshed != 1 {
		t.Fatal("a failed delete must not remove files nor refresh caches")
	}
}

func TestNodeDeleteServiceAnyType(t *testing.T) {
	nodes := &fakeDeleteNodesRepo{nodes: map[int]entity.Node{
		1: {ID: 1, Type: "form"}, 2: {ID: 2, Type: "poll"}, 3: {ID: 3, Type: "some_new_admin_type"},
	}}
	repo := &fakeNodeDeleteRepo{}
	s := NewNodeDeleteService(nodes, repo, &fakeNodeFileCleaner{}, &fakeFileRemover{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, id := range []int{1, 2, 3} {
		if err := s.Delete(context.Background(), id); err != nil {
			t.Fatalf("delete %d: %v", id, err)
		}
	}
	if len(repo.deleted) != 3 {
		t.Fatalf("every type goes through the same repository delete, got %v", repo.deleted)
	}
	if err := s.Delete(context.Background(), 99); !errors.Is(err, entity.ErrEntityNotFound) {
		t.Fatalf("missing node: want ErrEntityNotFound, got %v", err)
	}
	repo.err = errors.New("db down")
	if err := s.Delete(context.Background(), 1); err == nil {
		t.Fatal("repository error must be returned")
	}
}

// fakeTrash - the trash over fakeDeleteNodesRepo's nodes.
type fakeTrash struct {
	nodes *fakeDeleteNodesRepo
}

func (f fakeTrash) TrashNode(_ context.Context, id, by int, at time.Time) error {
	n := f.nodes.nodes[id]
	n.DeletedAt, n.DeletedBy = at, by
	f.nodes.nodes[id] = n
	return nil
}

func (f fakeTrash) RestoreNode(_ context.Context, id int) error {
	n := f.nodes.nodes[id]
	n.DeletedAt, n.DeletedBy = time.Time{}, 0
	f.nodes.nodes[id] = n
	return nil
}

func (f fakeTrash) TrashedNodes(context.Context, int, int) (entity.Nodes, int, error) {
	return nil, 0, nil
}

func (f fakeTrash) TrashedBefore(_ context.Context, before time.Time) ([]int, error) {
	var ids []int
	for id, n := range f.nodes.nodes {
		if n.InTrash() && n.DeletedAt.Before(before) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// TestNodeTrash - Remove puts a content type's node in the trash (by
// whom, when) and a module's type's - no trash - deletes it for good;
// Restore takes it out; Purge and the job delete for good, the job only
// what stayed longer than the days; a node not in the trash is neither
// restored nor purged.
func TestNodeTrash(t *testing.T) {
	entity.Modules = append(entity.Modules, entity.Module{Name: "trash_test", NodeTypes: []entity.ModuleNodeType{{Name: "trash_test_topic"}}})
	t.Cleanup(func() { entity.Modules = entity.Modules[:len(entity.Modules)-1] })
	nodes := &fakeDeleteNodesRepo{nodes: map[int]entity.Node{1: {ID: 1, Type: "article"}, 2: {ID: 2, Type: "article"}, 3: {ID: 3, Type: "trash_test_topic"}}}
	repo := &fakeNodeDeleteRepo{}
	refreshed := 0
	s := NewNodeDeleteService(nodes, repo, &fakeNodeFileCleaner{}, &fakeFileRemover{}, func() { refreshed++ }, slog.New(slog.DiscardHandler))
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	s.SetTrash(fakeTrash{nodes}, 30)
	ctx := context.Background()
	user := &entity.User{ID: 9}

	if trashed, err := s.Remove(ctx, 1, user); err != nil || !trashed || !nodes.nodes[1].DeletedAt.Equal(now) || nodes.nodes[1].DeletedBy != 9 {
		t.Fatalf("trash: %v %v %+v", trashed, err, nodes.nodes[1])
	}
	if trashed, err := s.Remove(ctx, 3, user); err != nil || trashed || len(repo.deleted) != 1 || repo.deleted[0] != 3 {
		t.Fatalf("a module's type: %v %v %v", trashed, err, repo.deleted)
	}
	if err := s.Purge(ctx, 2); !entity.IsNotFound(err) {
		t.Errorf("purging a node out of the trash: %v", err)
	}
	if err := s.Restore(ctx, 1); err != nil || nodes.nodes[1].InTrash() {
		t.Fatalf("restore: %v %+v", err, nodes.nodes[1])
	}
	if err := s.Restore(ctx, 1); !entity.IsNotFound(err) {
		t.Errorf("restoring a node out of the trash: %v", err)
	}

	// Two in the trash, one long enough to go.
	_, _ = s.Remove(ctx, 1, user)
	now = now.AddDate(0, 0, 20)
	_, _ = s.Remove(ctx, 2, user)
	now = now.AddDate(0, 0, 11)
	if err := s.TrashJob().Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(repo.deleted) != 2 || repo.deleted[1] != 1 {
		t.Errorf("the job deleted %v, want node 1 only", repo.deleted)
	}
	if err := s.Purge(ctx, 2); err != nil || repo.deleted[len(repo.deleted)-1] != 2 {
		t.Errorf("purge: %v %v", err, repo.deleted)
	}
	if refreshed == 0 {
		t.Error("the cache wasn't refreshed")
	}
}
