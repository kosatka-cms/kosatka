package complaints

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/modules/complaints/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
)

// noteMax - the longest note a visitor may add.
const noteMax = 1000

// Why a complaint isn't taken (Complain).
var (
	// ErrNotFound - no such item, or not one everyone sees.
	ErrNotFound = i18n.Errorf("there is nothing to complain about here")
	// ErrNotAllowed - the item's type takes no complaints, or it is the
	// visitor's own.
	ErrNotAllowed = i18n.Errorf("this can't be reported")
	// ErrReason - the reason isn't one offered, or "Other" without a note.
	ErrReason = i18n.Errorf("choose a reason; for \"Other\", say what is wrong")
	// ErrAction - a moderator's action that doesn't fit the item.
	ErrAction = errors.New("no such action for this item")
)

// Service - the complaints: taken from visitors, an item hidden after
// enough of them, decided by moderators.
type Service struct {
	repo     *Repo
	nodes    modulekit.NodesRepository
	comments modulekit.CommentsService
	deleter  modulekit.NodeDeleter
	site     modulekit.SiteInformationRepository
	users    modulekit.UserRepository
	mail     mailtemplate.Mailer
	logger   *slog.Logger
	baseURL  func() string
	now      func() time.Time
}

// ServiceDeps - what NewService needs.
type ServiceDeps struct {
	Repo     *Repo
	Nodes    modulekit.NodesRepository
	Comments modulekit.CommentsService
	Deleter  modulekit.NodeDeleter
	Site     modulekit.SiteInformationRepository
	Users    modulekit.UserRepository
	Mail     mailtemplate.Mailer
	Logger   *slog.Logger
	// BaseURL - the site's address for the e-mail's links (nil -
	// modulekit.MailBaseURL).
	BaseURL func() string
}

// NewService - the complaints' service.
func NewService(d ServiceDeps) *Service {
	if d.BaseURL == nil {
		d.BaseURL = modulekit.MailBaseURL
	}
	return &Service{repo: d.Repo, nodes: d.Nodes, comments: d.Comments, deleter: d.Deleter, site: d.Site,
		users: d.Users, mail: d.Mail, logger: d.Logger, baseURL: d.BaseURL, now: time.Now}
}

// Complain takes user's complaint about the object (KindNode or
// KindComment, its id) for reason (one of the settings' ReasonList; with
// OtherReason a note is needed): a second one of the same visitor changes
// nothing. A complaint new to the queue is mailed to the site; enough of
// them hide the object until a moderator decides. ErrNotFound,
// ErrNotAllowed, ErrReason - refused.
func (s *Service) Complain(ctx context.Context, user *modulekit.User, kind string, id int, reason, note string) error {
	settings, err := s.repo.Settings(ctx)
	if err != nil {
		return err
	}
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > noteMax {
		note = string([]rune(note)[:noteMax])
	}
	if !slices.Contains(settings.ReasonList(), reason) || (reason == OtherReason && note == "") {
		return ErrReason
	}
	object, err := s.object(ctx, settings, kind, id)
	if err != nil {
		return err
	}
	if object.AuthorID != 0 && object.AuthorID == user.ID {
		return ErrNotAllowed
	}
	target, added, fresh, err := s.repo.Add(ctx, object, user.ID, reason, note, s.now())
	if err != nil || !added {
		return err
	}
	if fresh {
		s.notify(ctx, target, reason, note)
	}
	return s.hideIfEnough(ctx, settings, target)
}

// object - what kind/id names, as a target: a public content item of a
// type taking complaints, or a published comment on a public item (when
// comments take them).
func (s *Service) object(ctx context.Context, settings Settings, kind string, id int) (Target, error) {
	switch kind {
	case KindNode:
		n, err := s.publicNode(ctx, id)
		if err != nil {
			return Target{}, err
		}
		if !settings.NodeTypeAllowed(string(n.Type)) {
			return Target{}, ErrNotAllowed
		}
		return Target{Kind: KindNode, ObjectID: n.ID, NodeID: n.ID, AuthorID: n.CreatedBy.ID}, nil
	case KindComment:
		if !settings.Comments {
			return Target{}, ErrNotAllowed
		}
		c, err := s.comments.ByID(ctx, id)
		if err != nil {
			if modulekit.IsNotFound(err) {
				return Target{}, ErrNotFound
			}
			return Target{}, fmt.Errorf("loading comment %d: %w", id, err)
		}
		if c.Status != modulekit.CommentPublished {
			return Target{}, ErrNotFound
		}
		if _, err := s.publicNode(ctx, c.Nid); err != nil {
			return Target{}, err
		}
		return Target{Kind: KindComment, ObjectID: c.ID, NodeID: c.Nid, AuthorID: c.CreatedBy}, nil
	}
	return Target{}, ErrNotFound
}

// publicNode - the content item id, if everyone sees it (ErrNotFound
// otherwise).
func (s *Service) publicNode(ctx context.Context, id int) (*modulekit.Node, error) {
	n, err := s.nodes.ByID(ctx, id)
	if err != nil {
		if modulekit.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("loading content %d: %w", id, err)
	}
	if !n.IsPublic() {
		return nil, ErrNotFound
	}
	return n, nil
}

// hideIfEnough hides t's object when the complaints since it was last
// resolved reach the settings' HideAfter (once: Hidden).
func (s *Service) hideIfEnough(ctx context.Context, settings Settings, t Target) error {
	if settings.HideAfter <= 0 || t.Hidden {
		return nil
	}
	var since time.Time
	if t.ResolvedAt != nil {
		since = *t.ResolvedAt
	}
	n, err := s.repo.CountSince(ctx, t.ID, since)
	if err != nil || n < settings.HideAfter {
		return err
	}
	switch t.Kind {
	case KindComment:
		err = s.comments.Hold(ctx, t.ObjectID, modulekit.CommentReason(modulekit.CommentReasonReported, strconv.Itoa(n)))
	case KindNode:
		err = s.setNodeStatus(ctx, t.ObjectID, nil, 0)
	}
	if err != nil {
		return fmt.Errorf("hiding %s %d after complaints: %w", t.Kind, t.ObjectID, err)
	}
	return s.repo.SetHidden(ctx, t.ID, true)
}

// Decide - a moderator's decision on the target: ResolutionKept (an
// object the complaints hid shown again), ResolutionSpam (a comment),
// ResolutionUnpublish (a content item unpublished, a comment back to
// waiting for approval), ResolutionDeleted (a comment deleted, a content
// item into the trash); the target leaves the queue.
// An object gone already is resolved as is. ErrAction - an action not
// for its kind.
func (s *Service) Decide(ctx context.Context, moderator *modulekit.User, targetID int, action string) error {
	t, err := s.repo.TargetByID(ctx, targetID)
	if err != nil {
		return err
	}
	switch {
	case action == ResolutionKept:
		if t.Hidden {
			err = s.restore(ctx, *t, moderator)
		}
	case action == ResolutionSpam && t.Kind == KindComment:
		err = s.comments.MarkSpam(ctx, t.ObjectID)
	case action == ResolutionUnpublish && t.Kind == KindNode:
		err = s.setNodeStatus(ctx, t.ObjectID, moderator, 0)
	case action == ResolutionUnpublish && t.Kind == KindComment:
		err = s.comments.Hold(ctx, t.ObjectID, modulekit.CommentReason(modulekit.CommentReasonReported, strconv.Itoa(t.Count)))
	case action == ResolutionDeleted && t.Kind == KindComment:
		err = s.comments.Delete(ctx, t.ObjectID)
	case action == ResolutionDeleted && t.Kind == KindNode:
		_, err = s.deleter.Remove(ctx, t.ObjectID, moderator)
	default:
		return ErrAction
	}
	if err != nil && !modulekit.IsNotFound(err) {
		return fmt.Errorf("deciding on %s %d (%s): %w", t.Kind, t.ObjectID, action, err)
	}
	return s.repo.Resolve(ctx, t.ID, action, moderator.ID, s.now())
}

// restore shows again an object the complaints hid.
func (s *Service) restore(ctx context.Context, t Target, moderator *modulekit.User) error {
	if t.Kind == KindComment {
		return s.comments.Approve(ctx, t.ObjectID)
	}
	return s.setNodeStatus(ctx, t.ObjectID, moderator, 1)
}

// setNodeStatus publishes (1) or unpublishes (0) a content item, recorded
// in its history as by (nil - the site's own doing).
func (s *Service) setNodeStatus(ctx context.Context, nodeID int, by *modulekit.User, status int) error {
	return modulekit.ChangeNode(ctx, nodeID, by, func(ctx context.Context) error {
		return s.nodes.UpdateStatus(ctx, nodeID, status)
	})
}

// CountNew - the targets waiting in the queue (the admin menu's number).
func (s *Service) CountNew(ctx context.Context) (int, error) { return s.repo.CountNew(ctx) }

// notify mails the site's address of a complaint new to the queue (the
// email MailNewComplaint); a failure is logged.
func (s *Service) notify(ctx context.Context, t Target, reason, note string) {
	if s.mail == nil || s.site == nil {
		return
	}
	site, err := s.site.Get(ctx)
	if err != nil || site == nil || strings.TrimSpace(site.SiteEmail) == "" {
		modulekit.LogFallback(s.logger, err, "the site's address for a complaint's e-mail (none sent)")
		return
	}
	base := strings.TrimSuffix(s.baseURL(), "/")
	if base == "" {
		return
	}
	lang := i18n.WithLang(ctx, i18n.Default())
	about, url := s.describe(lang, t)
	if note == "" {
		note = "-"
	}
	data := mailtemplate.TokenData{SiteName: site.SiteName, SiteSlogan: site.Slogan, SiteURLBrief: base, SiteLoginURL: base + "/login",
		Extra: map[string]string{
			"[complaint:about]":     about,
			"[complaint:reason]":    i18n.T(lang, reason),
			"[complaint:note]":      note,
			"[complaint:url]":       base + url,
			"[complaint:queue-url]": base + manifest.AdminQueuePath,
		}}
	modulekit.LogFallback(s.logger, s.mail.Send(ctx, MailNewComplaint, site.SiteEmail, data), "sending a complaint's e-mail", "target_id", t.ID)
}

// describe - what t is about in words (in ctx's language) and its page's
// path ("" - gone).
func (s *Service) describe(ctx context.Context, t Target) (about, url string) {
	n, err := s.nodes.ByID(ctx, t.NodeID)
	if err != nil {
		modulekit.LogFallback(s.logger, err, "the complained item's content", "node_id", t.NodeID)
		return i18n.T(ctx, "something gone"), ""
	}
	if t.Kind == KindNode {
		return "\"" + n.Title + "\"", n.URL()
	}
	author := i18n.T(ctx, "Guest")
	if t.AuthorID > 0 && s.users != nil {
		if u, err := s.users.ByID(ctx, t.AuthorID); err == nil && u != nil {
			author = u.Username
		}
	}
	return i18n.T(ctx, "a comment by @author on \"@title\"", "@author", author, "@title", n.Title),
		n.URL() + "#comment-" + strconv.Itoa(t.ObjectID)
}
