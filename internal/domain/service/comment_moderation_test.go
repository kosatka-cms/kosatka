package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// moderationRepo - the comments a moderation test sees: one comment (its
// status set by SetStatus), the author's published count.
type moderationRepo struct {
	postCommentRepo
	comment   entity.Comment
	published int
}

func (r *moderationRepo) ByID(_ context.Context, id int) (*entity.Comment, error) {
	c := r.comment
	c.ID = id
	return &c, nil
}

func (r *moderationRepo) SetStatus(_ context.Context, _ int, status int8, reason string) error {
	r.comment.Status, r.comment.HoldReason = status, reason
	return nil
}

func (r *moderationRepo) TotalCount(_ context.Context, f entity.CommentsFilter) (int, error) {
	if f.Status == nil || *f.Status != entity.CommentPublished || f.CreatedBy == nil {
		return 0, errors.New("unexpected count")
	}
	return r.published, nil
}

// TestCommentPostChecks - the first check holding or refusing a comment
// decides: held, it is saved waiting with the reason; refused, it isn't
// saved; a check's error fails the post.
func TestCommentPostChecks(t *testing.T) {
	ctx := context.Background()
	open := entity.Node{ID: 5, CommentMode: entity.CommentModeOpen}
	pass := func(context.Context, CommentDraft) (CommentVerdict, error) { return CommentVerdict{}, nil }
	hold := func(_ context.Context, d CommentDraft) (CommentVerdict, error) {
		if d.Content != "<p>Hi</p>" || d.AuthorID != 7 {
			t.Errorf("the draft: %+v", d)
		}
		return CommentVerdict{Hold: true, Reason: "why"}, nil
	}
	refused := errors.Join(ErrCommentRejected, errors.New("no"))
	reject := func(context.Context, CommentDraft) (CommentVerdict, error) {
		return CommentVerdict{Reject: refused}, nil
	}

	repo := &postCommentRepo{}
	s := NewCommentsService(repo, slog.Default())
	s.AddCheck(pass)
	s.AddCheck(hold)
	s.AddCheck(reject)
	c, err := s.Post(ctx, open, nil, 7, "<p>Hi</p>")
	if err != nil || c.Status != entity.CommentPending || c.HoldReason != "why" {
		t.Fatalf("held: %+v, %v", c, err)
	}

	repo.saved = nil
	s = NewCommentsService(repo, slog.Default())
	s.AddCheck(reject)
	s.AddCheck(hold)
	if _, err := s.Post(ctx, open, nil, 7, "<p>Hi</p>"); !errors.Is(err, ErrCommentRejected) || repo.saved != nil {
		t.Errorf("refused: %v, saved %+v", err, repo.saved)
	}

	s = NewCommentsService(repo, slog.Default())
	s.AddCheck(func(context.Context, CommentDraft) (CommentVerdict, error) {
		return CommentVerdict{}, errors.New("down")
	})
	if _, err := s.Post(ctx, open, nil, 7, "<p>Hi</p>"); err == nil || repo.saved != nil {
		t.Errorf("a failing check: %v", err)
	}
}

// TestCommentApproveAndSpam - approving publishes the comment and tells
// of it as of one posted; spam keeps the reason it waited for, else says
// it was marked by hand.
func TestCommentApproveAndSpam(t *testing.T) {
	ctx := context.Background()
	repo := &moderationRepo{comment: entity.Comment{Nid: 5, Status: entity.CommentPending, HoldReason: entity.CommentReasonNewAuthor}}
	s := NewCommentsService(repo, slog.Default())
	var told []int
	s.OnCommentPosted("", func(_ context.Context, c entity.Comment) error {
		told = append(told, c.ID)
		return nil
	})
	if err := s.MarkSpam(ctx, 9); err != nil || repo.comment.Status != entity.CommentSpam || repo.comment.HoldReason != entity.CommentReasonNewAuthor {
		t.Fatalf("spam: %+v, %v", repo.comment, err)
	}
	if err := s.Approve(ctx, 9); err != nil || repo.comment.Status != entity.CommentPublished || repo.comment.HoldReason != "" || len(told) != 1 || told[0] != 9 {
		t.Fatalf("approved: %+v, told %v, %v", repo.comment, told, err)
	}
	if err := s.Approve(ctx, 9); err != nil || len(told) != 1 {
		t.Errorf("approved twice: told %v, %v", told, err)
	}
	if err := s.MarkSpam(ctx, 9); err != nil || repo.comment.HoldReason != entity.CommentReasonMarked {
		t.Errorf("a published one marked: %+v, %v", repo.comment, err)
	}
}

// commentSettingsStub - fixed comment settings.
type commentSettingsStub struct{ entity.CommentSettings }

func (s commentSettingsStub) Get(context.Context) (entity.CommentSettings, error) {
	return s.CommentSettings, nil
}

func (commentSettingsStub) Update(context.Context, entity.CommentSettings) error { return nil }

// usersStub - every user is a plain one, but "trusted" may comment
// without approval.
type usersStub struct{}

func (usersStub) ByID(_ context.Context, id int) (*entity.User, error) {
	return &entity.User{ID: id, Username: map[bool]string{true: "trusted", false: "plain"}[id == 1]}, nil
}

type permStub struct{}

func (permStub) HasPermission(_ context.Context, u *entity.User, p string) bool {
	return p == PermSkipCommentApproval && u.Username == "trusted"
}

// TestNewAuthorCheck - an author holds while they have fewer published
// comments than the setting; every one with -1, none with 0; who may
// comment without approval never.
func TestNewAuthorCheck(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name             string
		checks, have, by int
		hold             bool
	}{
		{"first comment", 1, 0, 7, true},
		{"after one published", 1, 1, 7, false},
		{"second of three", 3, 1, 7, true},
		{"every one", -1, 50, 7, true},
		{"off", 0, 0, 7, false},
		{"may skip approval", -1, 0, 1, false},
	} {
		check := NewAuthorCheck(commentSettingsStub{entity.CommentSettings{NewAuthorChecks: c.checks}}, &moderationRepo{published: c.have}, usersStub{}, permStub{})
		v, err := check(ctx, CommentDraft{AuthorID: c.by})
		if err != nil || v.Hold != c.hold || (v.Hold && v.Reason != entity.CommentReasonNewAuthor) {
			t.Errorf("%s: %+v, %v", c.name, v, err)
		}
	}
}

// TestCommentLinks - the different web addresses of a comment: a link's
// and its text the same one, typed ones too.
func TestCommentLinks(t *testing.T) {
	for content, want := range map[string]int{
		`<p>No links here.</p>`: 0,
		`<p><a href="https://shop.example.com/a">https://shop.example.com/a</a></p>`:                        1,
		`<p>See www.example.org, and <a href="http://example.net/x?y=1&amp;z=2">this</a>.</p>`:              2,
		`<p>https://a.example.com/ https://b.example.com/ https://c.example.com/ https://a.example.com</p>`: 3,
	} {
		if got := CommentLinks(content); got != want {
			t.Errorf("%s: %d links, want %d", content, got, want)
		}
	}
}

// commentsDupRepo - the comments a duplicate and a purge see.
type commentsDupRepo struct {
	moderationRepo
	dup     bool
	since   time.Time
	spam    []int
	deleted []int
}

func (r *commentsDupRepo) HasDuplicate(_ context.Context, _ int, _ string, since time.Time) (bool, error) {
	r.since = since
	return r.dup, nil
}

func (r *commentsDupRepo) SpamBefore(_ context.Context, before time.Time) ([]int, error) {
	r.since = before
	return r.spam, nil
}

func (r *commentsDupRepo) Delete(_ context.Context, id int) error {
	r.deleted = append(r.deleted, id)
	return nil
}

// TestCommentContentChecks - links over the limit wait (none with no
// limit); a stop word holds or refuses, in a link's address too; the same
// text within the hours is refused.
func TestCommentContentChecks(t *testing.T) {
	ctx := context.Background()
	draft := func(content string) CommentDraft { return CommentDraft{AuthorID: 7, Content: content} }
	settings := func(s entity.CommentSettings) commentSettingsStub { return commentSettingsStub{s} }

	links := `<p>https://a.example.com https://b.example.com https://c.example.com</p>`
	if v, _ := LinksCheck(settings(entity.CommentSettings{LinksLimit: 2}))(ctx, draft(links)); !v.Hold || v.Reason != "links:3" {
		t.Errorf("3 links over 2: %+v", v)
	}
	if v, _ := LinksCheck(settings(entity.CommentSettings{LinksLimit: 3}))(ctx, draft(links)); v.Hold {
		t.Errorf("3 links, 3 allowed: %+v", v)
	}
	if v, _ := LinksCheck(settings(entity.CommentSettings{LinksLimit: -1}))(ctx, draft(links)); v.Hold {
		t.Errorf("no limit: %+v", v)
	}

	words := entity.CommentSettings{StopWords: "  \nCasino\nquick money\n", StopWordsAction: entity.StopWordsHold}
	if v, _ := StopWordsCheck(settings(words))(ctx, draft(`<p>Best <a href="https://casinos.example.com">games</a></p>`)); !v.Hold || v.Reason != "word:casino" {
		t.Errorf("a stop word in a link: %+v", v)
	}
	if v, _ := StopWordsCheck(settings(words))(ctx, draft(`<p>Quick&nbsp; money</p>`)); !v.Hold || v.Reason != "word:quick money" {
		t.Errorf("a phrase spaced otherwise: %+v", v)
	}
	words.StopWordsAction = entity.StopWordsReject
	if v, _ := StopWordsCheck(settings(words))(ctx, draft(`<p>Quick money!</p>`)); !errors.Is(v.Reject, ErrCommentRejected) {
		t.Errorf("refused: %+v", v)
	}
	if v, _ := StopWordsCheck(settings(words))(ctx, draft(`<p>Nothing to see.</p>`)); v.Reject != nil || v.Hold {
		t.Errorf("a clean comment: %+v", v)
	}

	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	repo := &commentsDupRepo{dup: true}
	check := DuplicateCheck(settings(entity.CommentSettings{DuplicateHours: 24}), repo, func() time.Time { return now })
	if v, _ := check(ctx, draft("<p>Hi</p>")); !errors.Is(v.Reject, ErrCommentRejected) || !repo.since.Equal(now.Add(-24*time.Hour)) {
		t.Errorf("a duplicate: %+v since %v", v, repo.since)
	}
	repo.dup = false
	if v, _ := check(ctx, draft("<p>Hi</p>")); v.Reject != nil {
		t.Errorf("no duplicate: %+v", v)
	}
	repo.dup = true
	if v, _ := DuplicateCheck(settings(entity.CommentSettings{}), repo, time.Now)(ctx, draft("<p>Hi</p>")); v.Reject != nil {
		t.Errorf("duplicates allowed: %+v", v)
	}
}

// TestSpamPurgeJob - spam marked longer ago than the days is deleted;
// with 0 days, nothing.
func TestSpamPurgeJob(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	repo := &commentsDupRepo{spam: []int{4, 5}}
	s := NewCommentsService(repo, slog.Default())
	job := SpamPurgeJob(commentSettingsStub{entity.CommentSettings{SpamKeepDays: 30}}, repo, s, func() time.Time { return now })
	if err := job.Run(ctx); err != nil || len(repo.deleted) != 2 || !repo.since.Equal(now.AddDate(0, 0, -30)) {
		t.Fatalf("purged %v before %v: %v", repo.deleted, repo.since, err)
	}
	repo.deleted = nil
	job = SpamPurgeJob(commentSettingsStub{entity.CommentSettings{}}, repo, s, func() time.Time { return now })
	if err := job.Run(ctx); err != nil || len(repo.deleted) != 0 {
		t.Errorf("kept: deleted %v, %v", repo.deleted, err)
	}
}
