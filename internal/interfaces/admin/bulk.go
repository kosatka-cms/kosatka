package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/textcut"
)

// parseBulkIDs parses ids from form (r.FormValue("ids") or r.Form["ids"])
func parseBulkIDs(r *http.Request) []int {
	var ids []int
	for _, v := range r.Form["ids"] {
		if id, err := strconv.Atoi(v); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// BulkConfirmContent shows confirmation page for bulk content actions
func (c *Controller) BulkConfirmContent(w http.ResponseWriter, r *http.Request) {
	c.bulkConfirmNodes(w, r, "/admin/content", "/api/admin/bulk/content")
}

// BulkConfirmModeration shows confirmation for bulk actions from the moderation queue (same API as content).
func (c *Controller) BulkConfirmModeration(w http.ResponseWriter, r *http.Request) {
	c.bulkConfirmNodes(w, r, "/admin/moderation", "/api/admin/bulk/moderation")
}

// bulkConfirmNodes - the confirmation of a bulk content action, posted
// from the list at back (and back there after), run by api.
func (c *Controller) bulkConfirmNodes(w http.ResponseWriter, r *http.Request, back, api string) {
	action, ok := bulkAction(w, r, back)
	if !ok {
		return
	}
	page := bulkConfirmPage{Action: action, API: api, Redirect: back, Title: bulkContentTitle}
	if action == "delete" {
		page.Notice = i18n.T(r.Context(), "The content of the content types goes to the trash, where it can be restored; the modules' content (forum topics, polls, forms) is deleted for good.")
	}
	c.bulkConfirm(w, r, page, func(ctx context.Context, ids []int) ([]bulkConfirmItem, error) {
		var items []bulkConfirmItem
		for _, id := range ids {
			node, err := c.nodeRepo.ByID(ctx, id)
			if err != nil {
				items = append(items, bulkConfirmItem{ID: id, Label: "?"})
				continue
			}
			items = append(items, bulkConfirmItem{ID: id, Label: node.Title})
		}
		return items, nil
	})
}

// BulkConfirmComment shows confirmation page for bulk comment actions
func (c *Controller) BulkConfirmComment(w http.ResponseWriter, r *http.Request) {
	action, ok := bulkAction(w, r, "/admin/content/comment")
	if !ok {
		return
	}
	if action != "approve" && action != "spam" {
		action = "delete"
	}
	back := service.SafeRedirectPath(r.FormValue("back"))
	if back == "" {
		back = commentsPath
	}
	page := bulkConfirmPage{Action: action, API: "/api/admin/bulk/comments", Redirect: back, Title: bulkCommentTitle}
	if action == "spam" {
		page.Alt = &bulkAltAction{
			Action: "spam_block",
			Label:  i18n.T(r.Context(), "and block their authors (not those who administer comments)"),
			Help:   i18n.T(r.Context(), "A blocked user can't sign in; their other comments stay as they are."),
		}
	}
	c.bulkConfirm(w, r, page, func(ctx context.Context, ids []int) ([]bulkConfirmItem, error) {
		var items []bulkConfirmItem
		for _, id := range ids {
			comment, err := c.commentsRepo.ByID(ctx, id)
			if err != nil {
				items = append(items, bulkConfirmItem{ID: id, Label: "?"})
				continue
			}
			label := textcut.Ellipsis(htmlsanitize.StripToText(comment.Content), 80)
			if comment.Subject != nil {
				label = *comment.Subject
			}
			items = append(items, bulkConfirmItem{ID: id, Label: label})
		}
		return items, nil
	})
}

// BulkConfirmPeople shows confirmation page for bulk people actions
func (c *Controller) BulkConfirmPeople(w http.ResponseWriter, r *http.Request) {
	action, ok := bulkAction(w, r, "/admin/people")
	if !ok {
		return
	}
	page := bulkConfirmPage{Action: action, API: "/api/admin/bulk/people", Redirect: "/admin/people", Title: bulkPeopleTitle}
	if action == peopleActionDeleteWithContent {
		page.Notice = i18n.T(r.Context(), "The accounts and everything these users created will be permanently deleted: content of all types (with its comments and everything attached to it), their comments, private messages, uploaded files and avatars. Site settings (content types, Views, vocabularies) are not affected.")
		page.Danger = true
	}
	c.bulkConfirm(w, r, page, func(ctx context.Context, ids []int) ([]bulkConfirmItem, error) {
		var items []bulkConfirmItem
		for _, id := range ids {
			user, err := c.userRepo.ByID(ctx, id)
			if err != nil {
				items = append(items, bulkConfirmItem{ID: id, Label: "?"})
				continue
			}
			label := user.Username
			if action == peopleActionDeleteWithContent {
				label += " - " + c.userContentSummaryText(ctx, id)
			}
			items = append(items, bulkConfirmItem{ID: id, Label: label})
		}
		return items, nil
	})
}

// bulkAction - the action a list's bulk form posted ("delete" when none);
// false - not a parsable POST, sent back to the list.
func bulkAction(w http.ResponseWriter, r *http.Request, list string) (string, bool) {
	if r.Method != http.MethodPost || r.ParseForm() != nil {
		http.Redirect(w, r, list, http.StatusFound)
		return "", false
	}
	if action := r.FormValue("action"); action != "" {
		return action, true
	}
	return "delete", true
}

// bulkContentTitle - the confirmation's question for n content items.
func bulkContentTitle(ctx context.Context, action string, n int) string {
	switch action {
	case "delete":
		return i18n.Plural(ctx, n, "Confirm deletion of @count content item", "Confirm deletion of @count content items")
	case "status_publish":
		return i18n.Plural(ctx, n, "Confirm publishing @count content item", "Confirm publishing @count content items")
	case "status_unpublish":
		return i18n.Plural(ctx, n, "Confirm unpublishing @count content item", "Confirm unpublishing @count content items")
	case "moderation_on":
		return i18n.Plural(ctx, n, "Confirm sending @count content item to moderation", "Confirm sending @count content items to moderation")
	case "moderation_off":
		return i18n.Plural(ctx, n, "Confirm removing @count content item from moderation", "Confirm removing @count content items from moderation")
	}
	return bulkOtherTitle(ctx, action, n)
}

// bulkCommentTitle - the confirmation's question for n comments.
func bulkCommentTitle(ctx context.Context, action string, n int) string {
	switch action {
	case "delete":
		return i18n.Plural(ctx, n, "Confirm deletion of @count comment", "Confirm deletion of @count comments")
	case "approve":
		return i18n.Plural(ctx, n, "Confirm approving @count comment", "Confirm approving @count comments")
	case "spam":
		return i18n.Plural(ctx, n, "Confirm marking @count comment as spam", "Confirm marking @count comments as spam")
	}
	return bulkOtherTitle(ctx, action, n)
}

// bulkPeopleTitle - the confirmation's question for n users.
func bulkPeopleTitle(ctx context.Context, action string, n int) string {
	switch action {
	case "delete":
		return i18n.Plural(ctx, n, "Confirm deletion of @count user (their content will remain without an author)", "Confirm deletion of @count users (their content will remain without an author)")
	case peopleActionDeleteWithContent:
		return i18n.Plural(ctx, n, "Confirm deletion of @count user along with all their content", "Confirm deletion of @count users along with all their content")
	case "block":
		return i18n.Plural(ctx, n, "Confirm blocking @count user", "Confirm blocking @count users")
	case "unblock":
		return i18n.Plural(ctx, n, "Confirm unblocking @count user", "Confirm unblocking @count users")
	}
	return bulkOtherTitle(ctx, action, n)
}

// bulkOtherTitle - the question for an action without its own.
func bulkOtherTitle(ctx context.Context, action string, n int) string {
	return i18n.T(ctx, "Confirm @action of @count items", "@action", action, "@count", n)
}

// peopleActionDeleteWithContent - the /admin/people bulk action that
// deletes the account and all its content (plain "delete" keeps the
// user's nodes, made authorless via nodes.created_by ON DELETE SET NULL).
const peopleActionDeleteWithContent = "delete_with_content"

// userContentSummaryText - what deleting this user with all content
// removes, for the bulk confirmation list.
func (c *Controller) userContentSummaryText(ctx context.Context, userID int) string {
	summary, err := c.userDeletionService.Summary(ctx, userID)
	if err != nil {
		c.LogFailure(err, "user content summary", "userID", userID)
		return i18n.T(ctx, "failed to count content")
	}
	labels := c.nodeTypeLabels(ctx)
	var parts []string
	for _, t := range summary.NodesByType {
		label := labels[t.Type]
		if label == "" {
			label = t.Type
		}
		parts = append(parts, fmt.Sprintf("%s: %d", label, t.Count))
	}
	if len(parts) == 0 {
		parts = append(parts, i18n.T(ctx, "no content"))
	}
	more := []string{i18n.T(ctx, "comments: @count", "@count", summary.Comments)}
	for _, m := range summary.Modules {
		more = append(more, i18n.T(ctx, m.Label, "@count", m.Count))
	}
	more = append(more, i18n.T(ctx, "files: @count", "@count", summary.Files))
	return strings.Join(parts, ", ") + "; " + strings.Join(more, ", ")
}

// invalidateContentCaches - after deleting content in bulk: the
// sidebar/View-block cache would otherwise keep showing deleted nodes
// and comments.
func (c *Controller) invalidateContentCaches() {
	if c.cacheService != nil {
		c.cacheService.ForceUpdate()
	}
}

type bulkConfirmItem struct {
	ID    int
	Label string
}

// bulkConfirmPage - a bulk action's confirmation page
// (admin/bulk_confirm.html, admin_bulk.js): Action goes to API item by
// item, then back to Redirect; Title - the question for the items'
// count; Notice - a warning above the list (Danger - the red one); Alt -
// an option that changes the action.
type bulkConfirmPage struct {
	Action, API, Redirect string
	Title                 func(ctx context.Context, action string, n int) string
	Notice                string
	Danger                bool
	Alt                   *bulkAltAction
}

// bulkAltAction - a checkbox of the confirmation page that, checked,
// applies Action instead (spam - spam and block the authors).
type bulkAltAction struct {
	Action, Label, Help string
}

func (c *Controller) bulkConfirm(
	w http.ResponseWriter,
	r *http.Request,
	page bulkConfirmPage,
	fetchItems func(ctx context.Context, ids []int) ([]bulkConfirmItem, error),
) {
	ids := parseBulkIDs(r)
	if len(ids) == 0 {
		http.Redirect(w, r, page.Redirect, http.StatusFound)
		return
	}

	items, err := fetchItems(r.Context(), ids)
	if err != nil {
		c.LogFailure(err, "bulk confirm fetch items")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["bulk"] = page
	data["bulkTitle"] = page.Title(r.Context(), page.Action, len(ids))
	data["items"] = items
	data["ids"] = ids

	c.RenderOrFail(w, r, "admin/bulk_confirm.html", data)
}

// bulkAPIRequest is the JSON body for bulk API
type bulkAPIRequest struct {
	Action string `json:"action"`
	IDs    []int  `json:"ids"`
}

// bulkAPIResponse is the JSON response for bulk API
type bulkAPIResponse struct {
	Processed int      `json:"processed"`
	Total     int      `json:"total"`
	Errors    []string `json:"errors,omitempty"`
	// Pending - the batch isn't finished yet (a long-running
	// per-item action, e.g. delete_with_content); the client repeats
	// the same request.
	Pending bool `json:"pending,omitempty"`
}

// BulkExecuteContent handles POST /api/admin/bulk/content
func (c *Controller) BulkExecuteContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.JSONError(w, i18n.T(r.Context(), "The request could not be read."), http.StatusBadRequest)
		return
	}
	var req bulkAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The request could not be read."), http.StatusBadRequest)
		return
	}
	if len(req.IDs) == 0 {
		httputil.JSONError(w, i18n.T(r.Context(), "The request could not be read."), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	user, _ := c.GetCurrentUser(r)
	switch req.Action {
	case "delete", "status_publish", "status_unpublish", "moderation_on", "moderation_off":
	default:
		httputil.JSONError(w, i18n.T(ctx, "The request could not be read."), http.StatusBadRequest)
		return
	}
	if !c.mayBulkContent(ctx, user, req.Action) {
		httputil.JSONError(w, i18n.T(ctx, "Access denied"), http.StatusForbidden)
		return
	}
	ids, denied := req.IDs, []string(nil)
	if req.Action == "delete" {
		var err error
		if ids, denied, err = c.deletableNodes(ctx, user, req.IDs); err != nil {
			c.LogFailure(err, "bulk delete: loading the nodes")
			httputil.JSONError(w, i18n.T(ctx, "The request could not be read."), http.StatusInternalServerError)
			return
		}
	}
	var processed int
	var errs []string
	switch req.Action {
	case "delete":
		processed, errs = c.adminBulkService.BulkDeleteNodes(ctx, user, ids)
	case "status_publish":
		processed, errs = c.adminBulkService.BulkUpdateNodeStatus(ctx, user, ids, 1)
	case "status_unpublish":
		processed, errs = c.adminBulkService.BulkUpdateNodeStatus(ctx, user, ids, 0)
	case "moderation_on":
		processed, errs = c.adminBulkService.BulkUpdateNodeModeration(ctx, user, ids, 1)
	case "moderation_off":
		processed, errs = c.adminBulkService.BulkUpdateNodeModeration(ctx, user, ids, 0)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bulkAPIResponse{Processed: processed, Total: len(req.IDs), Errors: append(denied, errs...)})
}

// mayBulkContent - whether user may apply the bulk content action at all,
// as the single actions check (access to the admin isn't enough):
// publishing takes "publish content" or "administer content" (the node
// form's publishing options), the moderation flag "administer content"
// or "moderation"; deleting is checked per node (deletableNodes).
func (c *Controller) mayBulkContent(ctx context.Context, user *entity.User, action string) bool {
	switch action {
	case "status_publish", "status_unpublish":
		return c.authService.CanPublishNode(ctx, user) || c.authService.HasPermission(ctx, user, service.PermAdministerContent)
	case "moderation_on", "moderation_off":
		return c.authService.HasPermission(ctx, user, service.PermAdministerContent) || c.authService.HasPermission(ctx, user, service.PermModerateContent)
	}
	return true
}

// deletableNodes - the nodes among ids user may delete ("delete any
// <type> content", as a node's own delete page), and one error line per
// node refused. An ID with no node is kept: the delete reports it.
func (c *Controller) deletableNodes(ctx context.Context, user *entity.User, ids []int) ([]int, []string, error) {
	nodes, err := c.nodeRepo.ByIDs(ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("the nodes to delete: %w", err)
	}
	var allowed []int
	var denied []string
	for _, id := range ids {
		if n, ok := nodes[id]; ok && !c.authService.CanDeleteNode(ctx, &n, user) {
			denied = append(denied, fmt.Sprintf("ID %d: %s", id, i18n.T(ctx, "Access denied")))
			continue
		}
		allowed = append(allowed, id)
	}
	return allowed, denied, nil
}

// BulkExecuteComments handles POST /api/admin/bulk/comments
func (c *Controller) BulkExecuteComments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.JSONError(w, i18n.T(r.Context(), "The request could not be read."), http.StatusBadRequest)
		return
	}
	var req bulkAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The request could not be read."), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	var do func(id int) error
	user, _ := c.GetCurrentUser(r)
	switch req.Action {
	case "delete":
		do = func(id int) error { return c.CommentsService().Delete(ctx, id) }
	case "approve":
		do = func(id int) error { return c.CommentsService().Approve(ctx, id) }
	case "spam":
		do = func(id int) error { return c.CommentsService().MarkSpam(ctx, id) }
	case "spam_block":
		do = func(id int) error { _, err := c.spamAndBlock(ctx, user, id); return err }
	}
	if do == nil || len(req.IDs) == 0 {
		httputil.JSONError(w, i18n.T(ctx, "The request could not be read."), http.StatusBadRequest)
		return
	}
	// Any comment, as on its own page: "administer comments".
	if !c.authService.HasPermission(ctx, user, service.PermAdministerComments) {
		httputil.JSONError(w, i18n.T(ctx, "Access denied"), http.StatusForbidden)
		return
	}
	processed := 0
	var errs []string
	for _, id := range req.IDs {
		if err := do(id); err != nil {
			c.LogFailure(err, "bulk comment action", "action", req.Action, "comment_id", id)
			errs = append(errs, strconv.Itoa(id)+": "+i18n.ErrorText(ctx, err))
		} else {
			processed++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bulkAPIResponse{Processed: processed, Total: len(req.IDs), Errors: errs})
}

// BulkExecutePeople handles POST /api/admin/bulk/people
func (c *Controller) BulkExecutePeople(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.JSONError(w, i18n.T(r.Context(), "The request could not be read."), http.StatusBadRequest)
		return
	}
	var req bulkAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.JSONError(w, i18n.T(r.Context(), "The request could not be read."), http.StatusBadRequest)
		return
	}
	if len(req.IDs) == 0 {
		httputil.JSONError(w, i18n.T(r.Context(), "The request could not be read."), http.StatusBadRequest)
		return
	}
	total := len(req.IDs)
	current, _ := c.GetCurrentUser(r)
	ids, errs := c.peopleBulkTargets(r.Context(), current, req.IDs, req.Action)
	var processed int
	var done []string
	switch req.Action {
	case peopleActionDeleteWithContent:
		// One user per request; while they still have content the response is
		// Pending and the client repeats the same request.
		for _, id := range ids {
			done, err := c.userDeletionService.DeleteWithContentStep(r.Context(), id, interfaces.StepBudget())
			if err != nil {
				c.LogFailure(err, "delete user with content", "userID", id)
				errs = append(errs, fmt.Sprintf("ID %d: %v", id, err))
				continue
			}
			if !done {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(bulkAPIResponse{Processed: processed, Total: total, Errors: errs, Pending: true})
				return
			}
			processed++
		}
		c.invalidateContentCaches()
	case "delete":
		processed, done = c.adminBulkService.BulkDeleteUsers(r.Context(), ids)
	case "block":
		processed, done = c.adminBulkService.BulkUpdateUserStatus(r.Context(), ids, entity.UserStatusBlocked)
	case "unblock":
		processed, done = c.adminBulkService.BulkUpdateUserStatus(r.Context(), ids, entity.UserStatusActive)
	default:
		httputil.JSONError(w, i18n.T(r.Context(), "The request could not be read."), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bulkAPIResponse{Processed: processed, Total: total, Errors: append(errs, done...)})
}

// peopleBulkTargets - of ids, the accounts current may act on, and why
// the others are left: not its own to block or delete, a full-access one
// only for a full-access editor (service.MayManageAccount).
func (c *Controller) peopleBulkTargets(ctx context.Context, current *entity.User, ids []int, action string) ([]int, []string) {
	var allowed []int
	var refused []string
	for _, id := range ids {
		if current != nil && id == current.ID && action != "unblock" {
			refused = append(refused, i18n.T(ctx, "you cannot block or delete your own account"))
			continue
		}
		target, err := c.userRepo.ByID(ctx, id)
		if err == nil && !service.MayManageAccount(ctx, c.authService, current, target) {
			refused = append(refused, fmt.Sprintf("ID %d: %s", id, i18n.T(ctx, "you may not change an account with this role")))
			continue
		}
		// A failed load is the action's to report.
		allowed = append(allowed, id)
	}
	return allowed, refused
}
