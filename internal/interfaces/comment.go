package interfaces

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// CommentController serves the comments' pages: a comment's link, the
// recent comments, a node's comments, posting, editing and deleting one.
type CommentController struct {
	*BaseController
	nodeRepo entity.NodesRepository
}

// NewCommentController creates a CommentController on deps.
func NewCommentController(deps BaseDeps) *CommentController {
	base := NewBaseController(deps)
	base.logger = deps.Logger.With("component", "comment.controller")
	return &CommentController{
		BaseController: base,
		nodeRepo:       deps.Nodes,
	}
}

// Redirect handles requests to /comment/{id} and redirects to the appropriate
// material page with the correct page number and comment anchor
func (c *CommentController) Redirect(w http.ResponseWriter, r *http.Request) {
	dbCtx, dbCancel := c.DBContext(r)
	defer dbCancel()

	commentID, ok := c.PathID(r)
	if !ok {
		c.logger.Warn("invalid or missing comment id", "id", r.PathValue("id"))
		c.RenderNotFound(w, r)
		return
	}

	comment, err := c.CommentsService().ByID(dbCtx, commentID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the comment", "comment_id", commentID)
		return
	}

	// A comment of a node that doesn't exist, or one the visitor may not
	// see (unpublished, in the trash, of a closed section), is a 404 like
	// a missing one: the redirect would tell its address.
	user, _ := c.GetCurrentUser(r)
	if comment.Node == nil || !c.authService.CanViewNode(dbCtx, comment.Node, user) {
		c.RenderNotFound(w, r)
		return
	}

	// One not published only for its author (waiting: after the
	// published ones, on the last page) and who administers comments.
	if comment.Status != entity.CommentPublished {
		mine := user != nil && user.ID == comment.CreatedBy && comment.Status == entity.CommentPending
		if !mine && !c.authService.HasPermission(dbCtx, user, service.PermAdministerComments) {
			c.RenderNotFound(w, r)
			return
		}
		published, err := c.CommentsService().CountByNodeID(dbCtx, comment.Nid, entity.CommentPublished, false)
		if err != nil {
			c.LogFailure(err, "counting the node's comments", "nid", comment.Nid)
		}
		last := max(1, (published+entity.CommentsPerPage-1)/entity.CommentsPerPage)
		http.Redirect(w, r, fmt.Sprintf("%s?page=%d#comment-%d", comment.Node.URL(), last, commentID), http.StatusFound)
		return
	}

	// Page the comment appears on (status 1 = published).
	pageNumber, err := c.CommentsService().GetCommentPageNumber(dbCtx, commentID, comment.Nid, 1)
	if err != nil {
		c.LogFailure(err, "error calculating comment page number",
			"comment_id", commentID,
			"nid", comment.Nid,
		)
		// If we can't calculate the page, redirect to the node without page param
		pageNumber = 1
	}

	http.Redirect(w, r, fmt.Sprintf("%s?page=%d#comment-%d", comment.Node.URL(), pageNumber, commentID), http.StatusFound)
}

// Recent renders GET /comments/recent - site-wide published comments with pagination.
func (c *CommentController) Recent(w http.ResponseWriter, r *http.Request) {
	dbCtx, dbCancel := c.DBContext(r)
	defer dbCancel()

	const perPage = 30
	status := entity.CommentPublished
	page := c.GetPageNumber(r)
	offset := (page - 1) * perPage

	comments, totalCount, err := c.CommentsService().ListRecentPublished(dbCtx, status, perPage, offset)
	if err != nil {
		c.LogFailure(err, "error loading recent comments")
		httputil.InternalServerError(w)
		return
	}

	pager := pagination.New(page, totalCount, perPage, r.URL.Path, r.URL.Query())
	pageTitle := i18n.T(r.Context(), "Recent comments")
	data := c.GetCommonData(r)
	data["title"] = pageTitle
	c.setPathTrail(data, r.URL.Path, pageTitle)
	data["pageSEO"] = PageSEOForPaginatedList(pageTitle, r.URL.Path, page, r.URL.Query())
	data["comments"] = comments
	data["pagination"] = pager
	data["isCommentsRecentPage"] = true
	data["showCommentAuthor"] = true
	data["truncateComments"] = true

	c.RenderOrFail(w, r, "comments/recent.html", data)
}

// commentsList - a page of the node's comments for the "comments-list"
// template (comments.html): its published comments, then the viewer's own
// waiting for approval (on the last page), the pager - for the comments
// API and for a content page that has them in its HTML.
func (c *BaseController) commentsList(ctx context.Context, r *http.Request, node *entity.Node, user *entity.User, page int) (map[string]any, error) {
	comments, totalCount, err := c.CommentsService().ByNodeIDPaginated(ctx, node.ID, 1, page)
	if err != nil {
		return nil, fmt.Errorf("loading the comments of node %d: %w", node.ID, err)
	}
	pager := pagination.NewWithAnchor(
		page,
		totalCount,
		entity.CommentsPerPage,
		"",
		r.URL.Query(),
		"comments",
	)
	// The viewer's own comments waiting for approval: after the
	// published ones, on the last page.
	var held entity.Comments
	if user != nil && pager.CurrentPage >= pager.TotalPages {
		pending := entity.CommentPending
		if held, err = c.CommentsService().Find(ctx, entity.CommentsFilter{Nid: &node.ID, CreatedBy: &user.ID, Status: &pending}); err != nil {
			service.LogFallback(c.logger, err, "loading the viewer's comments waiting for approval (none shown)", "node_id", node.ID)
		}
	}
	return map[string]any{
		"comments":     comments,
		"heldComments": held,
		"pagination":   pager,
		"currentUser":  user,
		"commentMode":  string(node.CommentMode),
		// canAdministerComments - edit/delete links on others' comments.
		"canAdministerComments": c.authService.HasPermission(r.Context(), user, service.PermAdministerComments),
	}, nil
}

// LoadComments handles AJAX requests to load comments for a node
func (c *CommentController) LoadComments(w http.ResponseWriter, r *http.Request) {
	dbCtx, dbCancel := c.DBContext(r)
	defer dbCancel()

	nodeIDStr := r.PathValue("nodeId")
	if nodeIDStr == "" {
		httputil.JSONError(w, i18n.T(r.Context(), "The content was not found."), http.StatusNotFound)
		return
	}

	nodeID, err := strconv.Atoi(nodeIDStr)
	if err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The content was not found."), http.StatusNotFound)
		return
	}

	node, err := c.nodeRepo.ByID(dbCtx, nodeID)
	if err != nil || node == nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The content was not found."), http.StatusNotFound)
		return
	}

	user, _ := c.GetCurrentUser(r)
	if !c.IsNodeVisible(dbCtx, node, user) {
		httputil.JSONError(w, i18n.T(r.Context(), "The content was not found."), http.StatusNotFound)
		return
	}
	if node.CommentMode == entity.CommentModeHidden {
		httputil.JSONError(w, i18n.T(r.Context(), "The content was not found."), http.StatusNotFound)
		return
	}

	page := c.GetPageNumber(r)
	data, err := c.commentsList(dbCtx, r, node, user, page)
	if err != nil {
		c.LogFailure(err, "error fetching comments",
			"node_id", nodeID,
			"page", page,
		)
		httputil.InternalServerError(w)
		return
	}

	var buf bytes.Buffer
	withRequestContext(r.Context(), data)
	err = c.templateService.RenderNamed(&buf, "comments-list", data, "comments.html", "pagination.html")
	if err != nil {
		c.LogFailure(err, "error rendering comments template",
			"node_id", nodeID,
			"page", page,
		)
		httputil.InternalServerError(w)
		return
	}

	pager := data["pagination"].(*pagination.Pagination)
	response := map[string]any{
		"html": buf.String(),
		"pagination": map[string]any{
			"currentPage": pager.CurrentPage,
			"totalPages":  pager.TotalPages,
			"hasMore":     pager.HasNext(),
			"hasPrev":     pager.HasPrevious(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// Create handles POST /comment/create.
func (c *CommentController) Create(w http.ResponseWriter, r *http.Request) {
	userID, loggedIn := c.authService.GetUserID(r)
	if !loggedIn {
		c.logger.Warn("unauthenticated user attempted to create comment")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		c.LogFailure(err, "error parsing form")
		c.RenderBadForm(w, r)
		return
	}

	// A bot's form (the bot trap), a new author without the CAPTCHA.
	if service.CheckBotTrap(r) != nil {
		c.logger.Info("comment refused by the bot trap", "user_id", userID)
		c.RenderBadRequest(w, r, i18n.T(r.Context(), "The form was sent too quickly, or looks sent by a program. Wait a few seconds and send it again."))
		return
	}
	if service.CommentCaptchaKey(r.Context(), &entity.User{ID: userID}) != "" {
		ok, err := captcha.Verify(r.Context(), entity.CaptchaFormComment, r.FormValue("g-recaptcha-response"), httputil.ClientIP(r))
		if err != nil {
			c.LogFailure(err, "recaptcha verify")
		}
		if !ok {
			c.RenderBadRequest(w, r, i18n.T(r.Context(), "Please confirm that you are not a robot."))
			return
		}
	}

	nidStr := r.FormValue("nid")
	nid, err := strconv.Atoi(nidStr)
	if err != nil {
		c.logger.Warn("invalid node id", "nid", nidStr, "error", err)
		c.RenderNotFound(w, r)
		return
	}

	node, err := c.nodeRepo.ByID(r.Context(), nid)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the content", "nid", nid)
		return
	}
	if !service.NodeTypeEnabled(string(node.Type)) {
		c.RenderNotFound(w, r)
		return
	}
	// Only a node the user may see (unpublished, on moderation - its
	// author's or a moderator's): a direct POST names any nid.
	user, _ := c.authService.GetCurrentUser(r.Context(), r)
	if !c.authService.CanViewNode(r.Context(), node, user) {
		c.RenderNotFound(w, r)
		return
	}

	var parentID *int
	if pid, err := strconv.Atoi(r.FormValue("parent_id")); err == nil && pid > 0 {
		parentID = &pid
	}

	dbCtx, dbCancel := c.DBContext(r)
	defer dbCancel()

	// The UI hides the form on a node not taking comments; Post refuses a
	// direct POST there too.
	createdComment, err := c.CommentsService().Post(dbCtx, *node, parentID, userID, r.FormValue("content"))
	switch {
	case errors.Is(err, service.ErrCommentEmpty):
		c.RenderBadRequest(w, r, i18n.T(r.Context(), "Enter the comment text."))
		return
	case errors.Is(err, service.ErrCommentsClosed):
		c.logger.Warn("comment create rejected: comments not open for this node", "nid", nid, "comment_mode", node.CommentMode, "user_id", userID)
		c.RenderBadRequest(w, r, i18n.T(r.Context(), "Comments are closed for this content."))
		return
	case errors.Is(err, service.ErrCommentParent):
		c.RenderBadRequest(w, r, i18n.T(r.Context(), "The comment you reply to isn't this content's."))
		return
	case errors.Is(err, service.ErrCommentRejected):
		c.logger.Info("comment refused by a check", "nid", nid, "user_id", userID, "reason", err.Error())
		c.RenderBadRequest(w, r, i18n.ErrorText(r.Context(), err))
		return
	}
	if err != nil {
		c.LogFailure(err, "error creating comment",
			"nid", nid,
			"user_id", userID,
		)
		httputil.InternalServerError(w)
		return
	}

	c.logger.Info("comment created",
		"comment_id", createdComment.ID,
		"nid", nid,
		"user_id", userID,
	)

	http.Redirect(w, r, fmt.Sprintf("/comment/%d", createdComment.ID), http.StatusFound)
}

// Edit handles GET/POST /comment/{id}/edit.
func (c *CommentController) Edit(w http.ResponseWriter, r *http.Request) {
	userID, loggedIn := c.authService.GetUserID(r)
	if !loggedIn {
		c.logger.Warn("unauthenticated user attempted to edit comment")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	dbCtx, dbCancel := c.DBContext(r)
	defer dbCancel()

	user, err := c.userService.ByID(dbCtx, userID)
	if err != nil {
		c.LogFailure(err, "error fetching user", "user_id", userID)
		httputil.InternalServerError(w)
		return
	}

	commentID, ok := c.PathID(r)
	if !ok {
		c.logger.Warn("invalid or missing comment id", "id", r.PathValue("id"))
		c.RenderNotFound(w, r)
		return
	}

	comment, err := c.CommentsService().ByID(dbCtx, commentID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the comment", "comment_id", commentID)
		return
	}

	// Check permissions: the author, or "administer comments"
	isAuthor := comment.CreatedBy == userID
	moderator := c.authService.HasPermission(r.Context(), user, service.PermAdministerComments)

	if !isAuthor && !moderator {
		c.logger.Warn("unauthorized edit attempt",
			"comment_id", commentID,
			"user_id", userID,
			"comment_author", comment.CreatedBy,
		)
		c.RenderForbidden(w, r)
		return
	}

	if r.Method == http.MethodGet {
		data := c.GetCommonData(r)
		data["comment"] = comment
		data["currentUser"] = user
		data["CSRFToken"] = csrf.EnsureToken(c.store, w, r)
		title := i18n.T(r.Context(), "Edit comment")
		data["title"] = title
		data["pageSEO"] = PageSEOForList(title, r.URL.Path)
		c.setCommentTrail(r.Context(), data, comment, title)

		if !c.RenderOrFail(w, r, "comment/edit.html", data) {
			return
		}
		return
	}

	if err := r.ParseForm(); err != nil {
		c.LogFailure(err, "error parsing form")
		c.RenderBadForm(w, r)
		return
	}

	// The author edits only what they may still comment on: a node they
	// see, taking comments (Revise), the text judged as a new one's.
	node, err := c.nodeRepo.ByID(dbCtx, comment.Nid)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the comment's node", "nid", comment.Nid)
		return
	}
	byAuthor := !moderator
	if byAuthor && !c.authService.CanViewNode(r.Context(), node, user) {
		c.RenderNotFound(w, r)
		return
	}
	_, err = c.CommentsService().Revise(dbCtx, *node, commentID, r.FormValue("content"), byAuthor)
	switch {
	case errors.Is(err, service.ErrCommentEmpty):
		c.RenderBadRequest(w, r, i18n.T(r.Context(), "Enter the comment text."))
		return
	case errors.Is(err, service.ErrCommentsClosed):
		c.RenderBadRequest(w, r, i18n.T(r.Context(), "Comments are closed for this content."))
		return
	case errors.Is(err, service.ErrCommentRejected):
		c.logger.Info("comment edit refused by a check", "comment_id", commentID, "user_id", userID, "reason", err.Error())
		c.RenderBadRequest(w, r, i18n.ErrorText(r.Context(), err))
		return
	}
	if err != nil {
		c.LogFailure(err, "error updating comment", "comment_id", commentID)
		httputil.InternalServerError(w)
		return
	}

	c.logger.Info("comment updated", "comment_id", commentID, "user_id", userID)

	http.Redirect(w, r, fmt.Sprintf("/comment/%d", commentID), http.StatusFound)
}

// DeleteConfirm handles GET /comment/{id}/delete - the confirmation page.
func (c *CommentController) DeleteConfirm(w http.ResponseWriter, r *http.Request) {
	userID, loggedIn := c.authService.GetUserID(r)
	if !loggedIn {
		c.logger.Warn("unauthenticated user attempted to delete comment")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	dbCtx, dbCancel := c.DBContext(r)
	defer dbCancel()

	user, err := c.userService.ByID(dbCtx, userID)
	if err != nil {
		c.LogFailure(err, "error fetching user", "user_id", userID)
		httputil.InternalServerError(w)
		return
	}

	// Deleting a comment - "administer comments".
	if !c.authService.HasPermission(r.Context(), user, service.PermAdministerComments) {
		c.logger.Warn("non-admin user attempted to delete comment", "user_id", userID)
		c.RenderForbidden(w, r)
		return
	}

	commentID, ok := c.PathID(r)
	if !ok {
		c.logger.Warn("invalid or missing comment id", "id", r.PathValue("id"))
		c.RenderNotFound(w, r)
		return
	}

	comment, err := c.CommentsService().ByID(dbCtx, commentID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the comment", "comment_id", commentID)
		return
	}

	replyCount, err := c.CommentsService().CountReplies(dbCtx, comment.ID)
	if err != nil {
		c.logger.Warn("error counting comment replies", "comment_id", commentID, "error", err)
	}

	data := c.GetCommonData(r)
	data["comment"] = comment
	data["currentUser"] = user
	data["replyCount"] = replyCount
	data["CSRFToken"] = csrf.EnsureToken(c.store, w, r)
	title := i18n.T(r.Context(), "Delete comment")
	data["title"] = title
	data["pageSEO"] = PageSEOForList(title, r.URL.Path)
	c.setCommentTrail(r.Context(), data, comment, title)

	c.RenderOrFail(w, r, "comment/delete.html", data)
}

// Delete handles POST /comment/{id}/delete.
func (c *CommentController) Delete(w http.ResponseWriter, r *http.Request) {
	userID, loggedIn := c.authService.GetUserID(r)
	if !loggedIn {
		c.logger.Warn("unauthenticated user attempted to delete comment")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}

	dbCtx, dbCancel := c.DBContext(r)
	defer dbCancel()

	user, err := c.userService.ByID(dbCtx, userID)
	if err != nil {
		c.LogFailure(err, "error fetching user", "user_id", userID)
		httputil.InternalServerError(w)
		return
	}

	// Deleting a comment - "administer comments".
	if !c.authService.HasPermission(r.Context(), user, service.PermAdministerComments) {
		c.logger.Warn("non-admin user attempted to delete comment", "user_id", userID)
		c.RenderForbidden(w, r)
		return
	}

	commentID, ok := c.PathID(r)
	if !ok {
		c.logger.Warn("invalid or missing comment id", "id", r.PathValue("id"))
		c.RenderNotFound(w, r)
		return
	}

	comment, err := c.CommentsService().ByID(dbCtx, commentID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the comment", "comment_id", commentID)
		return
	}

	back := "/"
	if comment.Node != nil && comment.Node.Slug != "" {
		back = comment.Node.URL()
	}

	err = c.CommentsService().Delete(dbCtx, commentID)
	if err != nil {
		c.LogFailure(err, "error deleting comment", "comment_id", commentID)
		httputil.InternalServerError(w)
		return
	}

	c.logger.Info("comment deleted", "comment_id", commentID, "user_id", userID)

	http.Redirect(w, r, back, http.StatusFound)
}

// setCommentTrail - a comment's page under its node (or under "Home"
// when the node isn't loaded).
func (c *CommentController) setCommentTrail(ctx context.Context, data map[string]any, comment *entity.Comment, title string) {
	if comment != nil && comment.Node != nil {
		c.setNodeSubpageTrail(ctx, data, *comment.Node, title)
		return
	}
	c.setChildTrail(data, nil, title)
}
