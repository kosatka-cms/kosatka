package entity

import (
	"context"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
)

// CommentsPerPage - how many comments a page of a node's comments shows.
var CommentsPerPage = 50

// CommentMode - per-node comment visibility: Open (comments shown, posting
// allowed), Closed (shown, posting disabled - enforced server-side too),
// Hidden (not rendered). ContentTypeDef.DefaultCommentMode is copied onto a
// new node at creation only; changing the type default later never touches
// existing nodes.
type CommentMode string

// The comment modes (CommentMode).
const (
	CommentModeOpen   CommentMode = "open"
	CommentModeClosed CommentMode = "closed"
	CommentModeHidden CommentMode = "hidden"
)

// OrDefault - the mode as stored on a node: "" (a form that doesn't carry
// the mode) is open, so nodes.comment_mode never gets an empty string.
func (m CommentMode) OrDefault() CommentMode {
	if m == "" {
		return CommentModeOpen
	}
	return m
}

// IsValidCommentMode reports whether s is one of the three CommentMode
// values (used to validate admin form input).
func IsValidCommentMode(s string) bool {
	switch CommentMode(s) {
	case CommentModeOpen, CommentModeClosed, CommentModeHidden:
		return true
	}
	return false
}

// Admin comments list sort keys (query param sort=)
const (
	AdminCommentsSortCreated = "created"
	AdminCommentsSortUpdated = "updated"
)

// AdminCommentsListFilter defines the status, sort and pagination of the
// admin comments list.
type AdminCommentsListFilter struct {
	Status int8   // Comment*: the list's tab
	SortBy string // AdminCommentsSort*; invalid/empty = created_at sort with Desc
	Desc   bool   // ascending when false (applies to default created_at sort too)
	Limit  int
	Offset int
}

// Comments - a list of comments.
type Comments []Comment

// A comment's Status: waiting for approval (shown to its author only),
// published, spam (shown to nobody; the admin's "Spam" tab).
const (
	CommentPending   int8 = 0
	CommentPublished int8 = 1
	CommentSpam      int8 = 2
)

// Why a comment waits for approval or is spam (Comment.HoldReason).
const (
	// CommentReasonNewAuthor - among its author's first comments
	// (CommentSettings.NewAuthorChecks).
	CommentReasonNewAuthor = "new_author"
	// CommentReasonMarked - marked as spam by hand.
	CommentReasonMarked = "marked"
	// CommentReasonLinks - more links than allowed: "links:<count>".
	CommentReasonLinks = "links"
	// CommentReasonStopWord - a stop word: "word:<the word>".
	CommentReasonStopWord = "word"
	// CommentReasonReported - visitors reported it (a module's
	// complaints): "reported:<how many>".
	CommentReasonReported = "reported"
)

// CommentReason - a reason with its detail ("links:3"); SplitCommentReason
// takes it apart.
func CommentReason(code, detail string) string { return code + ":" + detail }

// SplitCommentReason - reason's code and detail ("" - none).
func SplitCommentReason(reason string) (code, detail string) {
	code, detail, _ = strings.Cut(reason, ":")
	return code, detail
}

// Comment - a comment on a node (Nid): a root one, or a reply to ParentID;
// Node, Author, Parent and Children are set where loaded.
type Comment struct {
	ID        int
	Nid       int
	ParentID  *int
	CreatedBy int
	Subject   *string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Status    int8 // CommentPublished, CommentPending, CommentSpam
	// HoldReason - why it waits for approval or is spam (CommentReason*),
	// "" - none.
	HoldReason string
	Depth      int // Nesting depth (0 = root comment)
	Node       *Node
	Author     *User
	Parent     *Comment
	Children   Comments
}

// commentSummaryLength - Summary's cut of the text, in characters.
const commentSummaryLength = 60

// Summary - the comment in a list line: its subject, or the start of its
// text as plain text (cut at a word, with "…") when it has none, as new
// comments don't.
func (c Comment) Summary() string {
	if c.Subject != nil && strings.TrimSpace(*c.Subject) != "" {
		return *c.Subject
	}
	text := []rune(htmlsanitize.StripToText(c.Content))
	if len(text) <= commentSummaryLength {
		return string(text)
	}
	cut := string(text[:commentSummaryLength])
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:\u2014-") + "…" // \u2014: an em dash
}

// CommentsFilter - the comments Find and TotalCount select: a nil field
// matches any; Offset and Limit page them.
type CommentsFilter struct {
	Nid       *int
	ParentID  *int
	CreatedBy *int
	Status    *int8
	Offset    int
	Limit     int
}

// CommentsRepository - storage port for the comments.
type CommentsRepository interface {
	Find(ctx context.Context, filter CommentsFilter) (Comments, error)
	ListForAdmin(ctx context.Context, filter AdminCommentsListFilter) (Comments, int, error)
	ByID(ctx context.Context, id int) (*Comment, error)
	ByNodeID(ctx context.Context, nid int, status int8) (Comments, error)
	ByCreatedByPaginated(ctx context.Context, userID int, status int8, limit int, offset int) (Comments, error)
	ByNodeIDPaginated(ctx context.Context, nid int, status int8, limit int, offset int) (Comments, error)
	TotalCount(ctx context.Context, filter CommentsFilter) (int, error)
	CountByNodeID(ctx context.Context, nid int, status int8, rootOnly bool) (int, error)
	CountByNodeIDs(ctx context.Context, nids []int, status int8, rootOnly bool) (map[int]int, error)
	GetLatestComments(ctx context.Context, limit int, status int8, excludeTypes []string) (Comments, error)
	// ListRecentPublished returns published comments site-wide (including forum), except on nodes of excludeTypes, newest first, with total count.
	ListRecentPublished(ctx context.Context, status int8, excludeTypes []string, limit int, offset int) (Comments, int, error)
	GetCommentPath(ctx context.Context, commentID int) (string, error)
	CountCommentsWithPathLEQ(ctx context.Context, nid int, status int8, path string) (int, error)
	Create(ctx context.Context, c *Comment) (*Comment, error)
	Update(ctx context.Context, id int, c *Comment) (*Comment, error)
	// SetStatus sets the comment's status and the reason for it ("" -
	// none).
	SetStatus(ctx context.Context, id int, status int8, reason string) error
	// Delete removes the comment together with all its replies (the whole
	// subtree).
	Delete(ctx context.Context, id int) error
	// DescendantIDs returns the IDs of all replies to the comment, at any
	// depth.
	DescendantIDs(ctx context.Context, id int) ([]int, error)
	// NodeIDsByAuthor - the nodes the user has comments on (each once).
	NodeIDsByAuthor(ctx context.Context, userID int) ([]int, error)
	DeleteByNodeID(ctx context.Context, nodeID int) error
	// HasDuplicate - the author has a comment with this very text since
	// then (any status).
	HasDuplicate(ctx context.Context, authorID int, content string, since time.Time) (bool, error)
	// SpamBefore - the spam comments last changed (marked) before then.
	SpamBefore(ctx context.Context, before time.Time) ([]int, error)
}
