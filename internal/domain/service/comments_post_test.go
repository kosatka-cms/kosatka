package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// postCommentRepo keeps what Post and Revise write.
type postCommentRepo struct {
	entity.CommentsRepository
	saved *entity.Comment
	// status - the status SetStatus last gave.
	status int8
}

func (r *postCommentRepo) SetStatus(_ context.Context, _ int, status int8, _ string) error {
	r.status = status
	return nil
}

func (r *postCommentRepo) Create(_ context.Context, c *entity.Comment) (*entity.Comment, error) {
	r.saved = c
	return c, nil
}

func (r *postCommentRepo) ByID(_ context.Context, id int) (*entity.Comment, error) {
	return &entity.Comment{ID: id, Nid: 5, CreatedBy: 7, Status: entity.CommentPublished}, nil
}

func (r *postCommentRepo) Update(_ context.Context, _ int, c *entity.Comment) (*entity.Comment, error) {
	r.saved = c
	return c, nil
}

// TestCommentPost - a comment is posted sanitized, with the start of its
// text for the subject, published; nothing is for an empty text or on a
// node whose comments are closed or hidden.
func TestCommentPost(t *testing.T) {
	ctx := context.Background()
	repo := &postCommentRepo{}
	s := NewCommentsService(repo, slog.Default())
	open := entity.Node{ID: 5, CommentMode: entity.CommentModeOpen}

	c, err := s.Post(ctx, open, nil, 7, `<p>Hello <script>alert(1)</script>world, a comment long enough to be cut</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(c.Content, "script") || c.Nid != 5 || c.CreatedBy != 7 || c.Status != entity.CommentPublished {
		t.Errorf("posted %+v", c)
	}
	if c.Subject == nil || *c.Subject != "Hello world, a comment long en…" {
		t.Errorf("subject %v", c.Subject)
	}

	repo.saved = nil
	if _, err := s.Post(ctx, open, nil, 7, "<script>x</script>"); !errors.Is(err, ErrCommentEmpty) {
		t.Errorf("empty text: %v", err)
	}
	for _, mode := range []entity.CommentMode{entity.CommentModeClosed, entity.CommentModeHidden} {
		if _, err := s.Post(ctx, entity.Node{ID: 5, CommentMode: mode}, nil, 7, "text"); !errors.Is(err, ErrCommentsClosed) {
			t.Errorf("%s: %v", mode, err)
		}
	}
	if repo.saved != nil {
		t.Errorf("saved a refused comment: %+v", repo.saved)
	}
}

// TestCommentRevise - an edit replaces the text (sanitized) and the
// subject; an empty one is refused. The author's edit is judged as a new
// comment: not on a node closed to comments, refused or held by the
// checks; a moderator's isn't.
func TestCommentRevise(t *testing.T) {
	ctx := context.Background()
	open := entity.Node{ID: 5, CommentMode: entity.CommentModeOpen}
	repo := &postCommentRepo{}
	s := NewCommentsService(repo, slog.Default())
	if _, err := s.Revise(ctx, open, 3, "<b>New</b> text", true); err != nil {
		t.Fatal(err)
	}
	if repo.saved == nil || repo.saved.Content != "<b>New</b> text" || *repo.saved.Subject != "New text" {
		t.Errorf("saved %+v", repo.saved)
	}
	if _, err := s.Revise(ctx, open, 3, "  ", true); !errors.Is(err, ErrCommentEmpty) {
		t.Errorf("empty text: %v", err)
	}

	closed := entity.Node{ID: 5, CommentMode: entity.CommentModeClosed}
	repo.saved = nil
	if _, err := s.Revise(ctx, closed, 3, "text", true); !errors.Is(err, ErrCommentsClosed) || repo.saved != nil {
		t.Errorf("the author's edit on a closed node: %v, saved %+v", err, repo.saved)
	}
	if _, err := s.Revise(ctx, closed, 3, "text", false); err != nil || repo.saved == nil {
		t.Errorf("a moderator's edit on a closed node: %v", err)
	}

	s.AddCheck(func(_ context.Context, d CommentDraft) (CommentVerdict, error) {
		switch {
		case strings.Contains(d.Content, "spam"):
			return CommentVerdict{Reject: ErrCommentRejected}, nil
		case strings.Contains(d.Content, "link"):
			return CommentVerdict{Hold: true, Reason: "links"}, nil
		}
		return CommentVerdict{}, nil
	})
	repo.saved = nil
	if _, err := s.Revise(ctx, open, 3, "now spam", true); !errors.Is(err, ErrCommentRejected) || repo.saved != nil {
		t.Errorf("spam by the author: %v, saved %+v", err, repo.saved)
	}
	c, err := s.Revise(ctx, open, 3, "now a link", true)
	if err != nil || repo.status != entity.CommentPending || c.Status != entity.CommentPending {
		t.Errorf("a held edit: %v, status %d", err, repo.status)
	}
	repo.status = 0
	if _, err := s.Revise(ctx, open, 3, "a moderator's spam link", false); err != nil || repo.status != 0 {
		t.Errorf("a moderator's edit judged: %v, status %d", err, repo.status)
	}
}
