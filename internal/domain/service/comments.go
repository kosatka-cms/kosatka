package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/textcut"
)

// CommentsService - the comments on nodes: their lists and counts,
// posting and moderation, and the hooks of the modules that follow them.
type CommentsService interface {
	Find(ctx context.Context, filter entity.CommentsFilter) (entity.Comments, error)
	ByID(ctx context.Context, id int) (*entity.Comment, error)
	ByNodeID(ctx context.Context, nid int, status int8) (entity.Comments, error)
	ByNodeIDPaginated(ctx context.Context, nid int, status int8, page int) (entity.Comments, int, error)
	ByCreatedByPaginated(ctx context.Context, userID int, status int8, limit int, offset int) (entity.Comments, error)
	TotalCount(ctx context.Context, filter entity.CommentsFilter) (int, error)
	CountByNodeID(ctx context.Context, nid int, status int8, rootOnly bool) (int, error)
	CountByNodeIDs(ctx context.Context, nids []int, status int8, rootOnly bool) (map[int]int, error)
	GetLatest(ctx context.Context, limit int, status int8) (entity.Comments, error)
	ListRecentPublished(ctx context.Context, status int8, limit int, offset int) (entity.Comments, int, error)
	GetCommentPageNumber(ctx context.Context, commentID int, nid int, status int8) (int, error)
	Create(ctx context.Context, c *entity.Comment) (*entity.Comment, error)
	Update(ctx context.Context, id int, c *entity.Comment) (*entity.Comment, error)
	// Post adds a user's comment to the node as the comment form sends
	// it: the text sanitized (htmlsanitize.Sanitize), its start for the
	// subject; published, or waiting for approval when a check (AddCheck)
	// holds it. ErrCommentEmpty for a text with nothing left,
	// ErrCommentsClosed for a node not taking comments, ErrCommentParent
	// for a reply to another node's comment, ErrCommentRejected (wrapped)
	// for one a check refuses. Whether the author may see the node is the
	// caller's check.
	Post(ctx context.Context, node entity.Node, parentID *int, authorID int, content string) (*entity.Comment, error)
	// AddCheck adds a check of new comments (at startup): they run in the
	// order added, the first refusing or holding a comment decides.
	AddCheck(fn CommentCheck)
	// Approve publishes a comment waiting for approval or marked as spam,
	// telling of it (OnCommentPosted) as of one published at once.
	Approve(ctx context.Context, id int) error
	// MarkSpam marks the comment as spam: shown to nobody.
	MarkSpam(ctx context.Context, id int) error
	// Hold takes a published comment back to waiting for approval for
	// reason (entity.CommentReason*): shown to nobody but its author until
	// approved (Approve).
	Hold(ctx context.Context, id int, reason string) error
	// Revise replaces a comment's text (sanitized) and its subject;
	// ErrCommentEmpty for a text with nothing left. byAuthor - its author
	// revises it, not a moderator: then, as for a new comment (Post),
	// node must take comments (ErrCommentsClosed) and the checks judge
	// the new text (a refusal; held - it waits for approval again).
	Revise(ctx context.Context, node entity.Node, id int, content string, byAuthor bool) (*entity.Comment, error)
	// Delete removes the comment together with all its replies.
	Delete(ctx context.Context, id int) error
	// CountReplies returns how many replies (at any depth) the comment has.
	CountReplies(ctx context.Context, id int) (int, error)
	DeleteByNodeID(ctx context.Context, nodeID int) error
	BuildCommentTree(comments entity.Comments) entity.Comments
	// OnNodeCommentsChanged registers module's fn, called after a comment
	// of a node is created, updated or deleted while the module is
	// enabled: a module keeping its own counts of a node's comments
	// updates them. Its error is logged; the comment's change stands.
	OnNodeCommentsChanged(module string, fn func(ctx context.Context, nodeID int) error)
	// NodeCommentsChanged runs those hooks for the node: comments changed
	// another way (a user's, deleted with the account by cascade).
	NodeCommentsChanged(ctx context.Context, nodeID int)
	// NodeIDsCommentedBy - the nodes the user has comments on.
	NodeIDsCommentedBy(ctx context.Context, userID int) ([]int, error)
	// OnCommentPosted registers module's fn, called after a published
	// comment is created (a reply too: its ParentID) while the module is
	// enabled: a module telling of it (the subscriptions). Its error is
	// logged; the comment stands.
	OnCommentPosted(module string, fn func(ctx context.Context, c entity.Comment) error)
}

// DefaultCommentsService - the CommentsService on the comments repository.
type DefaultCommentsService struct {
	repository entity.CommentsRepository
	logger     *slog.Logger
	changed    []func(ctx context.Context, nodeID int) error
	posted     []func(ctx context.Context, c entity.Comment) error
	checks     []CommentCheck
}

// OnNodeCommentsChanged - see CommentsService.OnNodeCommentsChanged.
func (s *DefaultCommentsService) OnNodeCommentsChanged(module string, fn func(ctx context.Context, nodeID int) error) {
	s.changed = append(s.changed, func(ctx context.Context, nodeID int) error {
		if !ModuleEnabled(module) {
			return nil
		}
		return fn(ctx, nodeID)
	})
}

// OnCommentPosted - see CommentsService.OnCommentPosted.
func (s *DefaultCommentsService) OnCommentPosted(module string, fn func(ctx context.Context, c entity.Comment) error) {
	s.posted = append(s.posted, func(ctx context.Context, c entity.Comment) error {
		if !ModuleEnabled(module) {
			return nil
		}
		return fn(ctx, c)
	})
}

// NodeIDsCommentedBy - see CommentsService.NodeIDsCommentedBy.
func (s *DefaultCommentsService) NodeIDsCommentedBy(ctx context.Context, userID int) ([]int, error) {
	ids, err := s.repository.NodeIDsByAuthor(ctx, userID)
	return entity.Wrapped(ids, err, "the nodes user %v commented on", userID)
}

// NodeCommentsChanged runs the OnNodeCommentsChanged hooks for the node.
func (s *DefaultCommentsService) NodeCommentsChanged(ctx context.Context, nodeID int) {
	for _, fn := range s.changed {
		LogFallback(s.logger, fn(ctx, nodeID), "error updating after a comment change", "nid", nodeID)
	}
}

// NewCommentsService - the comments service on r (the comments) and
// baseLogger.
func NewCommentsService(r entity.CommentsRepository, baseLogger *slog.Logger) CommentsService {
	return &DefaultCommentsService{
		repository: r,
		logger:     baseLogger.With("component", "comments.service"),
	}
}

// Find - the comments matching filter.
func (s *DefaultCommentsService) Find(ctx context.Context, filter entity.CommentsFilter) (entity.Comments, error) {
	comments, err := s.repository.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("finding comments: %w", err)
	}

	return comments, nil
}

// ByID - the comment id, with its node and author.
func (s *DefaultCommentsService) ByID(ctx context.Context, id int) (*entity.Comment, error) {
	comment, err := s.repository.ByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting comment (id %v): %w", id, err)
	}

	return comment, nil
}

// ByCreatedByPaginated - a page of user userID's comments with status,
// newest first.
func (s *DefaultCommentsService) ByCreatedByPaginated(ctx context.Context, userID int, status int8, limit int, offset int) (entity.Comments, error) {
	comments, err := s.repository.ByCreatedByPaginated(ctx, userID, status, limit, offset)
	return entity.Wrapped(comments, err, "getting comments by user (user_id %v)", userID)
}

// ByNodeID - the node's comments with status, as a tree (BuildCommentTree).
func (s *DefaultCommentsService) ByNodeID(ctx context.Context, nid int, status int8) (entity.Comments, error) {
	comments, err := s.repository.ByNodeID(ctx, nid, status)
	if err != nil {
		return nil, fmt.Errorf("getting comments by node id (nid %v, status %v): %w", nid, status, err)
	}

	tree := s.BuildCommentTree(comments)

	return tree, nil
}

// ByNodeIDPaginated - one page (page, from 1) of the node's comments with
// status in thread order, entity.CommentsPerPage a page, and how many
// there are in all.
func (s *DefaultCommentsService) ByNodeIDPaginated(ctx context.Context, nid int, status int8, page int) (entity.Comments, int, error) {
	// Materialized path pagination: single query ORDER BY path LIMIT/OFFSET
	commentsPerPage := entity.CommentsPerPage
	offset := (page - 1) * commentsPerPage

	totalAllComments, err := s.repository.CountByNodeID(ctx, nid, status, false)
	if err != nil {
		return nil, 0, fmt.Errorf("counting comments (nid %v): %w", nid, err)
	}

	if totalAllComments == 0 {
		return entity.Comments{}, 0, nil
	}

	comments, err := s.repository.ByNodeIDPaginated(ctx, nid, status, commentsPerPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("getting paginated comments (nid %v): %w", nid, err)
	}

	return comments, totalAllComments, nil
}

// TotalCount - how many comments match filter.
func (s *DefaultCommentsService) TotalCount(ctx context.Context, filter entity.CommentsFilter) (int, error) {
	total, err := s.repository.TotalCount(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("comments total count: %w", err)
	}

	return total, nil
}

// CountByNodeID - how many comments with status the node has; rootOnly -
// only those that aren't replies.
func (s *DefaultCommentsService) CountByNodeID(ctx context.Context, nid int, status int8, rootOnly bool) (int, error) {
	count, err := s.repository.CountByNodeID(ctx, nid, status, rootOnly)
	if err != nil {
		return 0, fmt.Errorf("counting comments by node id (nid %v, status %v, rootOnly %v): %w", nid, status, rootOnly, err)
	}

	return count, nil
}

// CountByNodeIDs - CountByNodeID of each of the nodes nids, by node ID.
func (s *DefaultCommentsService) CountByNodeIDs(ctx context.Context, nids []int, status int8, rootOnly bool) (map[int]int, error) {
	counts, err := s.repository.CountByNodeIDs(ctx, nids, status, rootOnly)
	if err != nil {
		return map[int]int{}, fmt.Errorf("counting comments by node ids (nids %v, status %v): %w", nids, status, err)
	}
	return counts, nil
}

// Create saves comment c as given (a reply only to a comment on the same
// node, ErrCommentParent) and runs the node's comment hooks; a published
// one is told of (OnCommentPosted).
func (s *DefaultCommentsService) Create(ctx context.Context, c *entity.Comment) (*entity.Comment, error) {
	if c.Content == "" {
		return nil, fmt.Errorf("comment content cannot be empty")
	}

	if c.Nid == 0 {
		return nil, fmt.Errorf("node id is required")
	}

	if c.CreatedBy == 0 {
		return nil, fmt.Errorf("author id is required")
	}

	// The parent comment, if given, must exist and be on the same node.
	if c.ParentID != nil && *c.ParentID > 0 {
		parent, err := s.repository.ByID(ctx, *c.ParentID)
		if err != nil {
			return nil, fmt.Errorf("the parent comment (id %v): %w", *c.ParentID, err)
		}
		if parent.Nid != c.Nid {
			return nil, fmt.Errorf("the parent comment (id %v) is on node %v, not %v: %w", *c.ParentID, parent.Nid, c.Nid, ErrCommentParent)
		}
	}

	comment, err := s.repository.Create(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("creating comment (nid %v, author %v): %w", c.Nid, c.CreatedBy, err)
	}
	s.NodeCommentsChanged(ctx, c.Nid)
	if comment.Status == entity.CommentPublished {
		s.tellPosted(ctx, *comment)
	}

	return comment, nil
}

// tellPosted runs the OnCommentPosted hooks for the published comment c.
func (s *DefaultCommentsService) tellPosted(ctx context.Context, c entity.Comment) {
	for _, fn := range s.posted {
		LogFallback(s.logger, fn(ctx, c), "error telling of a posted comment", "comment_id", c.ID, "nid", c.Nid)
	}
}

// What a comment can't be posted with.
var (
	// ErrCommentParent - the comment replied to is on another node (Create).
	ErrCommentParent = errors.New("the parent comment is on another node")
	// ErrCommentEmpty - nothing but spaces is left of the comment's text
	// once sanitized.
	ErrCommentEmpty = errors.New("the comment is empty")
	// ErrCommentsClosed - the node takes no new comments (its comments
	// are closed or hidden).
	ErrCommentsClosed = errors.New("comments are closed for the node")
)

// commentSubjectLength - the runes of a comment's text its subject keeps.
const commentSubjectLength = 30

// commentSubject - the start of a comment's text, as plain text, for its
// subject.
func commentSubject(html string) string {
	return textcut.Ellipsis(htmlsanitize.StripToText(html), commentSubjectLength)
}

// Post - see CommentsService.
func (s *DefaultCommentsService) Post(ctx context.Context, node entity.Node, parentID *int, authorID int, content string) (*entity.Comment, error) {
	content = strings.TrimSpace(htmlsanitize.Sanitize(content))
	if content == "" {
		return nil, ErrCommentEmpty
	}
	if node.CommentMode == entity.CommentModeClosed || node.CommentMode == entity.CommentModeHidden {
		return nil, ErrCommentsClosed
	}
	v, err := s.verdict(ctx, CommentDraft{Node: node, ParentID: parentID, AuthorID: authorID, Content: content})
	if err != nil {
		return nil, err
	}
	if v.Reject != nil {
		return nil, v.Reject
	}
	status := entity.CommentPublished
	if v.Hold {
		status = entity.CommentPending
	}
	subject := commentSubject(content)
	return s.Create(ctx, &entity.Comment{
		Nid:        node.ID,
		ParentID:   parentID,
		CreatedBy:  authorID,
		Subject:    &subject,
		Content:    content,
		Status:     status,
		HoldReason: v.Reason,
	})
}

// Revise - see CommentsService.
func (s *DefaultCommentsService) Revise(ctx context.Context, node entity.Node, id int, content string, byAuthor bool) (*entity.Comment, error) {
	content = strings.TrimSpace(htmlsanitize.Sanitize(content))
	if content == "" {
		return nil, ErrCommentEmpty
	}
	var hold CommentVerdict
	if byAuthor {
		if node.CommentMode == entity.CommentModeClosed || node.CommentMode == entity.CommentModeHidden {
			return nil, ErrCommentsClosed
		}
		existing, err := s.repository.ByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("revising comment (id %v): %w", id, err)
		}
		v, err := s.verdict(ctx, CommentDraft{Node: node, ParentID: existing.ParentID, AuthorID: existing.CreatedBy, Content: content})
		if err != nil {
			return nil, err
		}
		if v.Reject != nil {
			return nil, v.Reject
		}
		if v.Hold && existing.Status == entity.CommentPublished {
			hold = v
		}
	}
	subject := commentSubject(content)
	comment, err := s.Update(ctx, id, &entity.Comment{Subject: &subject, Content: content})
	if err != nil || !hold.Hold {
		return comment, err
	}
	if err := s.repository.SetStatus(ctx, id, entity.CommentPending, hold.Reason); err != nil {
		return nil, fmt.Errorf("holding revised comment (id %v): %w", id, err)
	}
	comment.Status, comment.HoldReason = entity.CommentPending, hold.Reason
	s.NodeCommentsChanged(ctx, node.ID)
	return comment, nil
}

// Update saves the changes c to comment id and runs its node's comment
// hooks.
func (s *DefaultCommentsService) Update(ctx context.Context, id int, c *entity.Comment) (*entity.Comment, error) {
	existing, err := s.repository.ByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("updating comment (id %v): %w", id, err)
	}

	comment, err := s.repository.Update(ctx, id, c)
	if err != nil {
		return nil, fmt.Errorf("updating comment (id %v): %w", id, err)
	}
	s.NodeCommentsChanged(ctx, existing.Nid)

	return comment, nil
}

// Delete - see CommentsService.Delete.
func (s *DefaultCommentsService) Delete(ctx context.Context, id int) error {
	existing, err := s.repository.ByID(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting comment (id %v): %w", id, err)
	}

	err = s.repository.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting comment (id %v): %w", id, err)
	}
	s.NodeCommentsChanged(ctx, existing.Nid)
	audit.Record(ctx, audit.Action{Type: audit.CommentDelete, Object: commentObject(ctx, *existing)})

	return nil
}

// commentObject - the comment c as an action log's object: its author,
// the content it was on and the start of its text.
func commentObject(ctx context.Context, c entity.Comment) audit.Object {
	author, title := i18n.T(ctx, "Guest"), ""
	if c.Author != nil && c.Author.Username != "" {
		author = c.Author.Username
	}
	if c.Node != nil {
		title = c.Node.Title
	}
	return audit.Object{Kind: audit.KindComment, ID: strconv.Itoa(c.ID),
		Label: i18n.T(ctx, "@author on \"@title\": @text", "@author", author, "@title", title, "@text", PlainExcerpt(c.Content, 80))}
}

// CountReplies - see CommentsService.CountReplies.
func (s *DefaultCommentsService) CountReplies(ctx context.Context, id int) (int, error) {
	ids, err := s.repository.DescendantIDs(ctx, id)
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

// DeleteByNodeID deletes every comment of the node.
func (s *DefaultCommentsService) DeleteByNodeID(ctx context.Context, nodeID int) error {
	if err := s.repository.DeleteByNodeID(ctx, nodeID); err != nil {
		return fmt.Errorf("deleting the comments of node %v: %w", nodeID, err)
	}

	return nil
}

// BuildCommentTree builds a comment tree from a flat list.
func (s *DefaultCommentsService) BuildCommentTree(comments entity.Comments) entity.Comments {
	if len(comments) == 0 {
		return entity.Comments{}
	}

	commentMap := make(map[int]entity.Comment)
	for i := range comments {
		c := comments[i]
		c.Children = entity.Comments{}
		commentMap[c.ID] = c
	}

	childrenMap := make(map[int][]int)
	for i := range comments {
		c := comments[i]
		if c.ParentID != nil && *c.ParentID != 0 {
			childrenMap[*c.ParentID] = append(childrenMap[*c.ParentID], c.ID)
		}
	}

	var buildSubtree func(id int) entity.Comment
	buildSubtree = func(id int) entity.Comment {
		c := commentMap[id]
		c.Children = entity.Comments{}

		if childIDs, exists := childrenMap[id]; exists {
			for _, childID := range childIDs {
				c.Children = append(c.Children, buildSubtree(childID))
			}
		}

		return c
	}

	// Roots are comments with no parent (nil or 0).
	var rootComments entity.Comments
	for i := range comments {
		c := comments[i]
		if c.ParentID == nil || *c.ParentID == 0 {
			rootComments = append(rootComments, buildSubtree(c.ID))
		}
	}

	return rootComments
}

// ListRecentPublished returns paginated published comments across all nodes (including forum), newest first; comments on node types of disabled modules are left out.
func (s *DefaultCommentsService) ListRecentPublished(ctx context.Context, status int8, limit int, offset int) (entity.Comments, int, error) {
	comments, total, err := s.repository.ListRecentPublished(ctx, status, DisabledNodeTypes(), limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing recent published comments (limit %v, offset %v): %w", limit, offset, err)
	}
	return comments, total, nil
}

// GetLatest returns the latest comments from all content types but the
// disabled modules'.
func (s *DefaultCommentsService) GetLatest(ctx context.Context, limit int, status int8) (entity.Comments, error) {
	comments, err := s.repository.GetLatestComments(ctx, limit, status, DisabledNodeTypes())
	if err != nil {
		return nil, fmt.Errorf("getting latest comments (limit %v): %w", limit, err)
	}

	return comments, nil
}

// GetCommentPageNumber calculates which page a specific comment appears on
// when comments are displayed in threaded order (materialized path) with pagination.
func (s *DefaultCommentsService) GetCommentPageNumber(ctx context.Context, commentID int, nid int, status int8) (int, error) {
	path, err := s.repository.GetCommentPath(ctx, commentID)
	if err != nil {
		return 0, fmt.Errorf("getting comment path for page calculation (nid %v, commentID %v): %w", nid, commentID, err)
	}

	position, err := s.repository.CountCommentsWithPathLEQ(ctx, nid, status, path)
	if err != nil {
		return 0, err
	}

	// Position is 1-based count; convert to 0-indexed for page calc
	position0 := position - 1
	if position0 < 0 {
		position0 = 0
	}
	pageNumber := (position0 / entity.CommentsPerPage) + 1

	return pageNumber, nil
}

// deleteUserWithComments deletes a user's row with del - their comments
// go with it by cascade - then runs the comment hooks of the nodes they
// had comments on (a forum topic recounts its replies). comments nil -
// del only.
func deleteUserWithComments(ctx context.Context, comments CommentsService, logger *slog.Logger, userID int, del func() error) error {
	var nodes []int
	if comments != nil {
		ids, err := comments.NodeIDsCommentedBy(ctx, userID)
		LogFallback(logger, err, "the nodes a deleted user commented on", "user_id", userID)
		nodes = ids
	}
	if err := del(); err != nil {
		return err
	}
	for _, nid := range nodes {
		comments.NodeCommentsChanged(ctx, nid)
	}
	return nil
}
