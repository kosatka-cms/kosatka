package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeUserContentRepo struct {
	entity.UserContentRepository
	nodes        []int
	nodeFiles    map[int][]int
	creatorFiles []int
	fileURIs     map[int]string
	deletedFiles []int
}

func (f *fakeUserContentRepo) Summary(context.Context, int) (entity.UserContentSummary, error) {
	return entity.UserContentSummary{Nodes: len(f.nodes)}, nil
}

func (f *fakeUserContentRepo) ModuleInstalled(_ context.Context, module string) (bool, error) {
	return module == "notes", nil
}

func (f *fakeUserContentRepo) NodeIDs(_ context.Context, _ int, limit int) ([]int, error) {
	return f.nodes[:min(limit, len(f.nodes))], nil
}

func (f *fakeUserContentRepo) NodeFileIDs(_ context.Context, ids []int) ([]int, error) {
	var out []int
	for _, id := range ids {
		out = append(out, f.nodeFiles[id]...)
	}
	return out, nil
}

func (f *fakeUserContentRepo) FileIDsByCreator(context.Context, int) ([]int, error) {
	return f.creatorFiles, nil
}

func (f *fakeUserContentRepo) DeleteUnreferencedFiles(_ context.Context, ids []int) ([]string, error) {
	var uris []string
	for _, id := range ids {
		f.deletedFiles = append(f.deletedFiles, id)
		uris = append(uris, f.fileURIs[id])
	}
	return uris, nil
}

type fakeDeletionUserRepo struct {
	entity.UserRepository
	user    entity.User
	deleted bool
}

func (f *fakeDeletionUserRepo) ByID(context.Context, int) (*entity.User, error) {
	u := f.user
	return &u, nil
}

func (f *fakeDeletionUserRepo) Delete(context.Context, int) error {
	f.deleted = true
	return nil
}

type fakeFileRemover struct {
	removed    []string
	removedDir int
}

func (f *fakeFileRemover) RemoveStoredFile(_ context.Context, uri string) error {
	f.removed = append(f.removed, uri)
	return nil
}

func (f *fakeFileRemover) RemoveUserDir(_ context.Context, userID int) error {
	f.removedDir = userID
	return nil
}

func newTestUserDeletion(repo *fakeUserContentRepo, users *fakeDeletionUserRepo, files *fakeFileRemover) *UserDeletionService {
	deleteNode := func(_ context.Context, id int) error {
		repo.nodes = slices.DeleteFunc(repo.nodes, func(n int) bool { return n == id })
		return nil
	}
	return NewUserDeletionService(repo, users, deleteNode, files, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestDeleteWithContentStepDeletesEverything(t *testing.T) {
	repo := &fakeUserContentRepo{
		nodes:        []int{1, 2},
		nodeFiles:    map[int][]int{1: {10}},
		creatorFiles: []int{11},
		fileURIs:     map[int]string{10: "public://news/a.jpg", 11: "public://u7/b.png"},
	}
	users := &fakeDeletionUserRepo{user: entity.User{ID: 7, Username: "spam", Avatar: entity.File{URI: "/uploads/styles/user_avatar_list/user_pictures/7-1.webp"}, AvatarStored: "public://user_pictures/7-1.jpg"}}
	files := &fakeFileRemover{}

	s := newTestUserDeletion(repo, users, files)
	// A module's content of the user: counted on the confirmation, deleted
	// before the account.
	var moduleDeleted int
	s.AddModuleContent("notes", ModuleUserContent{
		Label:  "notes: @count",
		Count:  func(context.Context, int) (int, error) { return 3, nil },
		Delete: func(_ context.Context, id int) error { moduleDeleted = id; return nil },
	})
	// A module never installed has no tables: left alone.
	s.AddModuleContent("absent", ModuleUserContent{
		Label:  "absent: @count",
		Count:  func(context.Context, int) (int, error) { return 0, errors.New("no table") },
		Delete: func(context.Context, int) error { return errors.New("no table") },
	})
	summary, err := s.Summary(context.Background(), 7)
	if err != nil || !slices.Equal(summary.Modules, []entity.UserModuleContentCount{{Label: "notes: @count", Count: 3}}) {
		t.Fatalf("summary: %+v %v", summary, err)
	}
	done, err := s.DeleteWithContentStep(context.Background(), 7, time.Minute)
	if err != nil || !done {
		t.Fatalf("done=%v err=%v, want done", done, err)
	}
	if len(repo.nodes) != 0 || moduleDeleted != 7 || !users.deleted || files.removedDir != 7 {
		t.Fatalf("not everything deleted: nodes=%v module=%d user=%v dir=%d", repo.nodes, moduleDeleted, users.deleted, files.removedDir)
	}
	// The node's own files (public://news/a.jpg) are NodeDeleteService's
	// job (deleteNode is faked here); this service removes the user's
	// uploads, avatar and folder.
	want := []string{"public://u7/b.png", "public://user_pictures/7-1.jpg"}
	if !slices.Equal(files.removed, want) {
		t.Fatalf("removed files = %v, want %v", files.removed, want)
	}
}

func TestDeleteWithContentStepPendingWhenBudgetExhausted(t *testing.T) {
	repo := &fakeUserContentRepo{nodes: []int{1, 2, 3}}
	users := &fakeDeletionUserRepo{user: entity.User{ID: 7, Avatar: entity.File{URI: entity.AvatarPlaceholderURL}}}
	files := &fakeFileRemover{}
	svc := newTestUserDeletion(repo, users, files)

	done, err := svc.DeleteWithContentStep(context.Background(), 7, 0)
	if err != nil || done {
		t.Fatalf("done=%v err=%v, want pending", done, err)
	}
	if len(repo.nodes) != 2 || users.deleted {
		t.Fatalf("one node per zero-budget step expected, account kept: nodes=%v deleted=%v", repo.nodes, users.deleted)
	}
	for !done {
		if done, err = svc.DeleteWithContentStep(context.Background(), 7, 0); err != nil {
			t.Fatal(err)
		}
	}
	if !users.deleted || len(files.removed) != 0 {
		t.Fatalf("account deleted=%v, default avatar must not be removed: %v", users.deleted, files.removed)
	}
}

func TestDeleteWithContentStepStuckNodeErrors(t *testing.T) {
	repo := &fakeUserContentRepo{nodes: []int{1}}
	users := &fakeDeletionUserRepo{user: entity.User{ID: 7}}
	svc := NewUserDeletionService(repo, users, func(context.Context, int) error { return nil }, &fakeFileRemover{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := svc.DeleteWithContentStep(context.Background(), 7, time.Minute); err == nil {
		t.Fatal("expected error for a node that never goes away")
	}
	if users.deleted {
		t.Fatal("account must not be deleted while nodes remain")
	}
}
