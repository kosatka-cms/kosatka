package service

import (
	"context"
	"errors"
	"html/template"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func intPtr(v int) *int { return &v }

func strPtr(v string) *string { return &v }

func TestBuildCommentTree_Empty(t *testing.T) {
	s := &DefaultCommentsService{}
	got := s.BuildCommentTree(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty (non-nil) tree, got %#v", got)
	}
}

func TestBuildCommentTree_FlatRootsOnly(t *testing.T) {
	s := &DefaultCommentsService{}
	comments := entity.Comments{
		{ID: 1, ParentID: nil},
		{ID: 2, ParentID: intPtr(0)}, // 0 also means "no parent"
		{ID: 3, ParentID: nil},
	}

	got := s.BuildCommentTree(comments)

	if len(got) != 3 {
		t.Fatalf("expected 3 root comments, got %d: %+v", len(got), got)
	}
	for i, want := range []int{1, 2, 3} {
		if got[i].ID != want {
			t.Errorf("root[%d].ID = %d, want %d (order should be preserved)", i, got[i].ID, want)
		}
		if len(got[i].Children) != 0 {
			t.Errorf("root[%d] should have no children, got %d", i, len(got[i].Children))
		}
	}
}

func TestBuildCommentTree_SimpleNesting(t *testing.T) {
	s := &DefaultCommentsService{}
	comments := entity.Comments{
		{ID: 1, ParentID: nil},
		{ID: 2, ParentID: intPtr(1)},
		{ID: 3, ParentID: intPtr(1)},
	}

	got := s.BuildCommentTree(comments)

	if len(got) != 1 {
		t.Fatalf("expected 1 root comment, got %d", len(got))
	}
	root := got[0]
	if root.ID != 1 {
		t.Fatalf("root.ID = %d, want 1", root.ID)
	}
	if len(root.Children) != 2 {
		t.Fatalf("expected 2 children under root, got %d", len(root.Children))
	}
	if root.Children[0].ID != 2 || root.Children[1].ID != 3 {
		t.Errorf("children order = [%d, %d], want [2, 3]", root.Children[0].ID, root.Children[1].ID)
	}
}

func TestBuildCommentTree_MultiLevelNesting(t *testing.T) {
	s := &DefaultCommentsService{}
	comments := entity.Comments{
		{ID: 1, ParentID: nil},
		{ID: 2, ParentID: intPtr(1)},
		{ID: 3, ParentID: intPtr(2)}, // grandchild of 1
	}

	got := s.BuildCommentTree(comments)

	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("expected single root with ID 1, got %+v", got)
	}
	child := got[0].Children
	if len(child) != 1 || child[0].ID != 2 {
		t.Fatalf("expected root's only child to be ID 2, got %+v", child)
	}
	grandchild := child[0].Children
	if len(grandchild) != 1 || grandchild[0].ID != 3 {
		t.Fatalf("expected grandchild ID 3, got %+v", grandchild)
	}
}

// TestBuildCommentTree_OrphanedChildIsDropped: a comment whose parent is
// not in the input (deleted, or a filtered slice) is silently excluded -
// it is neither a root nor reachable from one.
func TestBuildCommentTree_OrphanedChildIsDropped(t *testing.T) {
	s := &DefaultCommentsService{}
	comments := entity.Comments{
		{ID: 1, ParentID: nil},
		{ID: 2, ParentID: intPtr(999)}, // parent 999 not present in the slice
	}

	got := s.BuildCommentTree(comments)

	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("expected only root ID 1 to survive, got %+v", got)
	}
	if len(got[0].Children) != 0 {
		t.Errorf("root should have no children (orphan is dropped, not attached), got %+v", got[0].Children)
	}
}

func TestBuildCommentTree_MultipleRootsWithOwnSubtrees(t *testing.T) {
	s := &DefaultCommentsService{}
	comments := entity.Comments{
		{ID: 1, ParentID: nil},
		{ID: 2, ParentID: intPtr(1)},
		{ID: 10, ParentID: nil},
		{ID: 11, ParentID: intPtr(10)},
	}

	got := s.BuildCommentTree(comments)

	if len(got) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(got))
	}
	if got[0].ID != 1 || len(got[0].Children) != 1 || got[0].Children[0].ID != 2 {
		t.Errorf("first root subtree mismatch: %+v", got[0])
	}
	if got[1].ID != 10 || len(got[1].Children) != 1 || got[1].Children[0].ID != 11 {
		t.Errorf("second root subtree mismatch: %+v", got[1])
	}
}

type fakeReplyRepo struct {
	entity.CommentsRepository
	descendants []int
	byAuthor    []int
}

func (f fakeReplyRepo) DescendantIDs(context.Context, int) ([]int, error) {
	return f.descendants, nil
}

func (f fakeReplyRepo) NodeIDsByAuthor(context.Context, int) ([]int, error) { return f.byAuthor, nil }

func TestCountReplies(t *testing.T) {
	s := &DefaultCommentsService{repository: fakeReplyRepo{descendants: []int{2, 3, 5}}}
	n, err := s.CountReplies(t.Context(), 1)
	if err != nil || n != 3 {
		t.Fatalf("CountReplies = %d, %v", n, err)
	}
}

// TestCommentDeleteTemplateReplyCount - the delete confirmation names how
// many replies go with the comment, in the right plural form. Russian is
// requested explicitly: the test checks the catalog's Russian plural forms.
func TestCommentDeleteTemplateReplyCount(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	for count, want := range map[int]string{0: "", 1: "удалится 1 ответ.", 3: "удалятся 3 ответа.", 5: "удалятся 5 ответов."} {
		var buf strings.Builder
		data := map[string]any{"comment": &entity.Comment{ID: 7, Content: "x", CreatedAt: time.Now()}, "replyCount": count, RequestLanguageKey: "ru"}
		if err := svc.RenderNamed(&buf, "content", data, "comment/delete.html"); err != nil {
			t.Fatal(err)
		}
		if want == "" {
			if strings.Contains(buf.String(), "ответ") {
				t.Errorf("0 replies: warning shown")
			}
		} else if !strings.Contains(buf.String(), want) {
			t.Errorf("%d replies: missing %q", count, want)
		}
	}
}

// TestDeleteUserWithComments - deleting a user (their comments go by
// cascade) runs the comment hooks of every node they had comments on;
// a failed delete runs none.
func TestDeleteUserWithComments(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)
	svc := NewCommentsService(fakeReplyRepo{byAuthor: []int{5, 6}}, logger)
	var changed []int
	svc.OnNodeCommentsChanged("", func(_ context.Context, nid int) error {
		changed = append(changed, nid)
		return nil
	})

	if err := deleteUserWithComments(ctx, svc, logger, 3, func() error { return errors.New("db down") }); err == nil || len(changed) != 0 {
		t.Errorf("a failed delete: %v, hooks for %v", err, changed)
	}
	if err := deleteUserWithComments(ctx, svc, logger, 3, func() error { return nil }); err != nil || !slices.Equal(changed, []int{5, 6}) {
		t.Errorf("deleted: %v, hooks for %v, want [5 6]", err, changed)
	}
	if err := deleteUserWithComments(ctx, nil, logger, 3, func() error { return nil }); err != nil {
		t.Errorf("without a comments service: %v", err)
	}
}
