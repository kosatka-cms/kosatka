package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// CommentDraft - a comment being posted, as its checks see it: its
// sanitized text.
type CommentDraft struct {
	Node     entity.Node
	ParentID *int
	AuthorID int
	Content  string
}

// CommentVerdict - a check's decision on a draft: zero - it passes;
// Hold - it waits for approval, Reason (entity.CommentReason*) says why;
// Reject - it is refused, the error (wrapping ErrCommentRejected) is shown
// to its author.
type CommentVerdict struct {
	Hold   bool
	Reason string
	Reject error
}

// CommentCheck - a check of new comments (CommentsService.AddCheck).
type CommentCheck func(ctx context.Context, d CommentDraft) (CommentVerdict, error)

// ErrCommentRejected - a check refused the comment (CommentVerdict.Reject
// wraps it).
var ErrCommentRejected = errors.New("the comment is refused")

// AddCheck - see CommentsService.AddCheck.
func (s *DefaultCommentsService) AddCheck(fn CommentCheck) { s.checks = append(s.checks, fn) }

// verdict - the checks' decision on d: the first refusing or holding
// it decides; none - it is published.
func (s *DefaultCommentsService) verdict(ctx context.Context, d CommentDraft) (CommentVerdict, error) {
	for _, check := range s.checks {
		v, err := check(ctx, d)
		if err != nil {
			return CommentVerdict{}, fmt.Errorf("checking a comment of user %d: %w", d.AuthorID, err)
		}
		if v.Reject != nil || v.Hold {
			return v, nil
		}
	}
	return CommentVerdict{}, nil
}

// Approve - see CommentsService.Approve.
func (s *DefaultCommentsService) Approve(ctx context.Context, id int) error {
	c, err := s.repository.ByID(ctx, id)
	if err != nil {
		return fmt.Errorf("approving comment %d: %w", id, err)
	}
	if c.Status == entity.CommentPublished {
		return nil
	}
	if err := s.repository.SetStatus(ctx, id, entity.CommentPublished, ""); err != nil {
		return err
	}
	s.NodeCommentsChanged(ctx, c.Nid)
	c.Status, c.HoldReason = entity.CommentPublished, ""
	s.tellPosted(ctx, *c)
	audit.Record(ctx, audit.Action{Type: audit.CommentApprove, Object: commentObject(ctx, *c)})
	return nil
}

// Hold - see CommentsService.Hold.
func (s *DefaultCommentsService) Hold(ctx context.Context, id int, reason string) error {
	c, err := s.repository.ByID(ctx, id)
	if err != nil {
		return fmt.Errorf("holding comment %d: %w", id, err)
	}
	if c.Status != entity.CommentPublished {
		return nil
	}
	if err := s.repository.SetStatus(ctx, id, entity.CommentPending, reason); err != nil {
		return err
	}
	s.NodeCommentsChanged(ctx, c.Nid)
	return nil
}

// MarkSpam - see CommentsService.MarkSpam.
func (s *DefaultCommentsService) MarkSpam(ctx context.Context, id int) error {
	c, err := s.repository.ByID(ctx, id)
	if err != nil {
		return fmt.Errorf("marking comment %d as spam: %w", id, err)
	}
	if c.Status == entity.CommentSpam {
		return nil
	}
	reason := c.HoldReason
	if reason == "" {
		reason = entity.CommentReasonMarked
	}
	if err := s.repository.SetStatus(ctx, id, entity.CommentSpam, reason); err != nil {
		return err
	}
	s.NodeCommentsChanged(ctx, c.Nid)
	audit.Record(ctx, audit.Action{Type: audit.CommentSpam, Object: commentObject(ctx, *c)})
	return nil
}

// NewAuthorCheck holds the first comments of an author (CommentSettings.
// NewAuthorChecks, counting their published ones) who may not comment
// without approval (PermSkipCommentApproval).
func NewAuthorCheck(settings entity.CommentSettingsRepository, comments entity.CommentsRepository, users interface {
	ByID(ctx context.Context, id int) (*entity.User, error)
}, auth interface {
	HasPermission(ctx context.Context, user *entity.User, permission string) bool
}) CommentCheck {
	return func(ctx context.Context, d CommentDraft) (CommentVerdict, error) {
		cfg, err := settings.Get(ctx)
		if err != nil || cfg.NewAuthorChecks == 0 {
			return CommentVerdict{}, err
		}
		author, err := users.ByID(ctx, d.AuthorID)
		if err != nil {
			return CommentVerdict{}, fmt.Errorf("loading the comment's author: %w", err)
		}
		if auth.HasPermission(ctx, author, PermSkipCommentApproval) {
			return CommentVerdict{}, nil
		}
		hold := CommentVerdict{Hold: true, Reason: entity.CommentReasonNewAuthor}
		if cfg.NewAuthorChecks < 0 {
			return hold, nil
		}
		published := entity.CommentPublished
		n, err := comments.TotalCount(ctx, entity.CommentsFilter{CreatedBy: &d.AuthorID, Status: &published})
		if err != nil {
			return CommentVerdict{}, fmt.Errorf("counting the author's comments: %w", err)
		}
		if n < cfg.NewAuthorChecks {
			return hold, nil
		}
		return CommentVerdict{}, nil
	}
}

// DuplicateCheck refuses the same text by the same author again within
// CommentSettings.DuplicateHours.
func DuplicateCheck(settings entity.CommentSettingsRepository, comments entity.CommentsRepository, now func() time.Time) CommentCheck {
	return func(ctx context.Context, d CommentDraft) (CommentVerdict, error) {
		cfg, err := settings.Get(ctx)
		if err != nil || cfg.DuplicateHours <= 0 {
			return CommentVerdict{}, err
		}
		dup, err := comments.HasDuplicate(ctx, d.AuthorID, d.Content, now().Add(-time.Duration(cfg.DuplicateHours)*time.Hour))
		if err != nil || !dup {
			return CommentVerdict{}, err
		}
		return CommentVerdict{Reject: rejected(i18n.Errorf("You have already posted this comment."))}, nil
	}
}

// StopWordsCheck holds or refuses (CommentSettings.StopWordsAction) a
// comment with a stop word in its text or a link's address, case aside.
func StopWordsCheck(settings entity.CommentSettingsRepository) CommentCheck {
	return func(ctx context.Context, d CommentDraft) (CommentVerdict, error) {
		cfg, err := settings.Get(ctx)
		if err != nil {
			return CommentVerdict{}, err
		}
		// Any run of spaces (a non-breaking one too) as one: a phrase
		// matches however it is spaced.
		text := strings.Join(strings.Fields(strings.ToLower(html.UnescapeString(d.Content))), " ")
		for _, w := range cfg.StopWordList() {
			if !strings.Contains(text, w) {
				continue
			}
			if cfg.StopWordsAction == entity.StopWordsReject {
				return CommentVerdict{Reject: rejected(i18n.Errorf("The comment has a word not allowed here."))}, nil
			}
			return CommentVerdict{Hold: true, Reason: entity.CommentReason(entity.CommentReasonStopWord, w)}, nil
		}
		return CommentVerdict{}, nil
	}
}

// linkPattern - a web address in a comment: a link's or one typed.
var linkPattern = regexp.MustCompile(`(?i)\b(?:https?://|www\.)[^\s"'<>]+`)

// CommentLinks - the different web addresses in a comment's HTML.
func CommentLinks(content string) int {
	seen := map[string]bool{}
	for _, m := range linkPattern.FindAllString(html.UnescapeString(content), -1) {
		seen[strings.TrimRight(strings.ToLower(m), ".,;:!?)/")] = true
	}
	return len(seen)
}

// LinksCheck holds a comment with more links than
// CommentSettings.LinksLimit.
func LinksCheck(settings entity.CommentSettingsRepository) CommentCheck {
	return func(ctx context.Context, d CommentDraft) (CommentVerdict, error) {
		cfg, err := settings.Get(ctx)
		if err != nil || cfg.LinksLimit < 0 {
			return CommentVerdict{}, err
		}
		if n := CommentLinks(d.Content); n > cfg.LinksLimit {
			return CommentVerdict{Hold: true, Reason: entity.CommentReason(entity.CommentReasonLinks, strconv.Itoa(n))}, nil
		}
		return CommentVerdict{}, nil
	}
}

// rejected - a check's refusal, message shown to the author
// (ErrCommentRejected).
func rejected(message error) error {
	return fmt.Errorf("%w: %w", ErrCommentRejected, message)
}

// SpamPurgeJob - the scheduler's job deleting spam kept longer than
// CommentSettings.SpamKeepDays (with its replies), hourly.
func SpamPurgeJob(settings entity.CommentSettingsRepository, comments entity.CommentsRepository, s CommentsService, now func() time.Time) Job {
	return Job{Name: "comment_spam_purge", Every: time.Hour, Run: func(ctx context.Context) error {
		cfg, err := settings.Get(ctx)
		if err != nil || cfg.SpamKeepDays <= 0 {
			return err
		}
		ids, err := comments.SpamBefore(ctx, now().AddDate(0, 0, -cfg.SpamKeepDays))
		if err != nil {
			return err
		}
		var errs []error
		for _, id := range ids {
			// A reply of spam deleted with it is gone already.
			if err := s.Delete(ctx, id); err != nil && !IsNotFound(err) {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}}
}
