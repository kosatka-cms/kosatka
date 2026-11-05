package privatemessages

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// Controller - the module's pages: a thread, the new message form, the
// actions on threads, and the profile's messages tab (ProfileTab).
type Controller struct {
	*modulekit.BaseController
	threads Service
	mailer  *Mailer
	// users - the recipient a new message's form starts with (?to=).
	users modulekit.UserRepository
}

// NewController - the module's controller.
func NewController(bc *modulekit.BaseController, threads Service, mailer *Mailer, users modulekit.UserRepository) *Controller {
	bc.SetLogger(bc.Logger().With("component", "private_messages.handler"))
	return &Controller{BaseController: bc, threads: threads, mailer: mailer, users: users}
}

// Messages - the old address of the private messages: they are the
// profile's messages tab (/me/messages).
func (c *Controller) Messages(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, base, http.StatusMovedPermanently)
}

// ThreadBulkArchive archives several threads (POST /me/messages/archive).
func (c *Controller) ThreadBulkArchive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, loggedIn := c.AuthService().GetUserID(r)
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}

	threadIDs := r.Form["thread_ids[]"]
	redirectTo := base + "?status=active"
	if len(threadIDs) == 0 {
		http.Redirect(w, r, redirectTo, http.StatusFound)
		return
	}

	for _, s := range threadIDs {
		threadID, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		if err := c.threads.Archive(ctx, threadID, userID); err != nil {
			c.LogFailure(err, "archiving the thread", "threadID", threadID)
		}
	}

	http.Redirect(w, r, redirectTo, http.StatusFound)
}

// ThreadBulkUnarchive unarchives several threads (POST /me/messages/unarchive).
func (c *Controller) ThreadBulkUnarchive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, loggedIn := c.AuthService().GetUserID(r)
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}

	threadIDs := r.Form["thread_ids[]"]
	if len(threadIDs) == 0 {
		http.Redirect(w, r, base+"?status=archived", http.StatusFound)
		return
	}

	for _, s := range threadIDs {
		threadID, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		if err := c.threads.Unarchive(ctx, threadID, userID); err != nil {
			c.LogFailure(err, "unarchiving the thread", "threadID", threadID)
		}
	}

	http.Redirect(w, r, base+"?status=archived", http.StatusFound)
}

// ThreadView shows a thread and its messages.
func (c *Controller) ThreadView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, loggedIn := c.AuthService().GetUserID(r)
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	threadID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}

	thread, err := c.threads.ThreadByID(ctx, threadID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the thread", "threadID", threadID)
		return
	}

	if !thread.HasMember(userID) {
		c.RenderForbidden(w, r)
		return
	}

	const perPage = 20

	page := c.GetPageNumber(r)

	// Total message count.
	totalCount, err := c.threads.MessageTotalCount(ctx, MessageFilter{ThreadID: threadID})
	if err != nil {
		c.LogFailure(err, "error getting messages count")
		httputil.InternalServerError(w)
		return
	}

	pager := pagination.New(page, totalCount, perPage, r.URL.Path, r.URL.Query())

	msgs, err := c.threads.MessageList(ctx,
		MessageFilter{ThreadID: threadID},
		Pagination{Limit: pager.ItemsPerPage, Offset: pager.Offset},
	)
	if err != nil {
		c.LogFailure(err, "error loading messages", "thread_id", threadID)
		httputil.InternalServerError(w)
		return
	}

	if err := c.threads.MarkRead(ctx, threadID, userID); err != nil {
		c.LogFailure(err, "marking the thread read", "threadID", threadID)
	}

	data := c.GetCommonData(r)
	data["thread"] = thread
	data["messagePage"] = msgs
	data["pagination"] = pager
	data["currentUserID"] = userID
	data["title"] = thread.Subject
	// The browser tab's title (it had none), not indexed (a private page).
	c.ApplyEditorPageSEOFromTitle(data, r)
	data["CSRFToken"] = csrf.EnsureToken(c.Store(), w, r)

	c.RenderOrFail(w, r, "private_messages/thread.html", data)
}

// ThreadCreate creates a new thread (POST /me/messages/new).
func (c *Controller) ThreadCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, loggedIn := c.AuthService().GetUserID(r)
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}

	subject := r.FormValue("subject")
	body := htmlsanitize.Sanitize(r.FormValue("body"))
	if subject == "" || body == "" {
		c.RenderBadRequest(w, r, i18n.T(ctx, "Enter the subject and the message."))
		return
	}

	// Recipients from the form: recipients[]=1&recipients[]=2.
	recipientStrs := r.Form["recipients[]"]
	if len(recipientStrs) == 0 {
		c.RenderBadRequest(w, r, i18n.T(ctx, "Choose at least one recipient."))
		return
	}

	var memberIDs []int
	for _, s := range recipientStrs {
		id, err := strconv.Atoi(s)
		if err != nil {
			c.RenderBadRequest(w, r, i18n.T(ctx, "The recipient was not found."))
			return
		}
		if !slices.Contains(memberIDs, id) {
			memberIDs = append(memberIDs, id)
		}
	}
	// Each recipient is mailed: a message isn't a mailing list.
	if len(memberIDs) > maxRecipients {
		c.RenderBadRequest(w, r, i18n.T(ctx, "A message goes to @count recipients at most.", "@count", strconv.Itoa(maxRecipients)))
		return
	}

	thread, err := c.threads.ThreadCreate(ctx, userID, subject, memberIDs, body)
	if errors.Is(err, ErrNoSuchRecipient) {
		c.RenderBadRequest(w, r, i18n.T(ctx, "The recipient was not found."))
		return
	}
	if err != nil {
		c.LogFailure(err, "error creating thread", "user_id", userID)
		httputil.InternalServerError(w)
		return
	}

	c.mailer.NotifyLater(thread.ID, userID, modulekit.MailBaseURL())

	http.Redirect(w, r, base+"/thread/"+strconv.Itoa(thread.ID), http.StatusFound)
}

// maxRecipients - the recipients a new message may have.
const maxRecipients = 20

// MessageReply replies in an existing thread (POST /me/messages/thread/{id}/reply).
func (c *Controller) MessageReply(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, loggedIn := c.AuthService().GetUserID(r)
	if !loggedIn {
		c.RenderForbidden(w, r)
		return
	}

	threadID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}

	body := htmlsanitize.Sanitize(r.FormValue("body"))
	if body == "" {
		c.RenderBadRequest(w, r, i18n.T(ctx, "Enter the message."))
		return
	}

	msg := Message{
		ThreadID: threadID,
		AuthorID: userID,
		Body:     body,
	}

	_, err = c.threads.MessageCreate(ctx, msg)
	if errors.Is(err, ErrNotThreadMember) {
		c.RenderForbidden(w, r)
		return
	}
	if err != nil {
		// No such thread - 404; another failure - 500.
		c.RenderLoadFailure(w, r, err, "error creating message", "thread_id", threadID)
		return
	}

	c.mailer.NotifyLater(threadID, userID, modulekit.MailBaseURL())

	http.Redirect(w, r, base+"/thread/"+strconv.Itoa(threadID), http.StatusFound)
}

// ThreadMarkRead marks a thread as read (POST /me/messages/thread/{id}/read).
func (c *Controller) ThreadMarkRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, loggedIn := c.AuthService().GetUserID(r)
	if !loggedIn {
		httputil.JSONError(w, i18n.T(ctx, "Sign in to continue."), http.StatusUnauthorized)
		return
	}

	threadID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.JSONError(w, i18n.T(ctx, "The conversation was not found."), http.StatusNotFound)
		return
	}

	if err := c.threads.MarkRead(ctx, threadID, userID); err != nil {
		c.LogFailure(err, "error marking read")
		httputil.InternalServerError(w)
		return
	}

	httputil.SendJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ThreadArchive archives a thread (POST /me/messages/thread/{id}/archive).
func (c *Controller) ThreadArchive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, loggedIn := c.AuthService().GetUserID(r)
	if !loggedIn {
		c.RenderForbidden(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}

	threadID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}

	if err := c.threads.Archive(ctx, threadID, userID); err != nil {
		c.LogFailure(err, "error archiving thread")
		httputil.InternalServerError(w)
		return
	}

	http.Redirect(w, r, base, http.StatusFound)
}

// ThreadNew - the new message form (GET /me/messages/new); ?to=<user ID>
// starts it with that recipient (the user_actions link next to a name).
func (c *Controller) ThreadNew(w http.ResponseWriter, r *http.Request) {
	viewerID, loggedIn := c.AuthService().GetUserID(r)
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	data := c.GetCommonData(r)
	if to, err := strconv.Atoi(r.URL.Query().Get("to")); err == nil && to > 0 && to != viewerID {
		if u, err := c.users.ByID(r.Context(), to); err == nil && u != nil && u.Status == modulekit.UserStatusActive {
			data["recipient"] = u
		}
	}
	data["title"] = i18n.T(r.Context(), "New message")
	c.ApplyEditorPageSEOFromTitle(data, r)
	data["CSRFToken"] = csrf.EnsureToken(c.Store(), w, r)

	c.RenderOrFail(w, r, "private_messages/new.html", data)
}

// ProfileTab - the profile's messages tab: the profile's owner and those
// who administer users open it; its badge - the viewer's unread threads.
func (c *Controller) ProfileTab() modulekit.ProfileTab {
	return modulekit.ProfileTab{
		Module: manifest.Name,
		Path:   manifest.TabPath,
		Label:  i18n.N("Messages"),
		Allowed: func(_ context.Context, viewer, _ *modulekit.User, isOwner, canAdminister bool) bool {
			return viewer != nil && (isOwner || canAdminister)
		},
		Badge: c.unreadCount,
		Load:  c.loadTab,
	}
}

// unreadCount - the viewer's unread threads (0 - none, or they can't be
// counted).
func (c *Controller) unreadCount(ctx context.Context, viewer *modulekit.User) int {
	n, err := c.threads.UnreadCount(ctx, viewer.ID)
	if err != nil {
		modulekit.LogFallback(c.Logger(), err, "counting unread threads (none shown)", "user_id", viewer.ID)
		return 0
	}
	return n
}

// PageData - every page's data (AddPageData): the signed-in user's
// unread threads, data["unreadCount"] (the module's templates) and on the
// user menu's toggle.
func (c *Controller) PageData(r *http.Request, data map[string]any) {
	user, _ := data["user"].(*modulekit.User)
	if user == nil {
		return
	}
	n := c.unreadCount(r.Context(), user)
	data["unreadCount"] = n
	modulekit.AddUserMenuCount(data, n)
}

// loadTab - the threads of the profile's user, active (default) or
// archived (?status=archived), with one participant (?participant=ID),
// a page of them. pmTabURL - the tab's own address (the filters stay on
// it); pmOwn - the viewer's own tab: writing, archiving and opening the
// threads are theirs only (an administrator of users sees the list).
func (c *Controller) loadTab(ctx context.Context, w http.ResponseWriter, r *http.Request, user *modulekit.User, data map[string]any) error {
	const perPage = 25
	viewer, _ := data["user"].(*modulekit.User)
	data["pmTabURL"] = r.URL.Path
	data["pmOwn"] = viewer != nil && viewer.ID == user.ID

	// A private page: not indexed.
	if seo, ok := data["pageSEO"].(modulekit.PageSEO); ok {
		seo.NoIndex = true
		data["pageSEO"] = seo
	}
	data["CSRFToken"] = csrf.EnsureToken(c.Store(), w, r)

	filter := ThreadFilter{UserID: user.ID}

	// Tabs: active (default) or archived.
	status := r.URL.Query().Get("status")
	if status != "archived" {
		status = "active"
	}
	archived := status == "archived"
	filter.Deleted = &archived
	data["filterStatus"] = status
	data["filterParticipant"] = r.URL.Query().Get("participant")
	data["filterParticipantName"] = ""

	// Filter by participant.
	if pid, err := strconv.Atoi(r.URL.Query().Get("participant")); err == nil && pid > 0 {
		filter.ParticipantID = pid
		if participant, err := c.UserService().ByID(ctx, pid); err == nil {
			data["filterParticipantName"] = participant.Username
		} else {
			modulekit.LogFallback(c.Logger(), err, "loading the participant of the filter (no name shown)", "user_id", pid)
		}
	}

	data["threads"] = &Threads{}
	data["pagination"] = pagination.New(1, 0, perPage, r.URL.Path, r.URL.Query())
	totalCount, err := c.threads.ThreadTotalCount(ctx, filter)
	if err != nil {
		return fmt.Errorf("counting threads (user %d): %w", user.ID, err)
	}
	pager := pagination.New(c.GetPageNumber(r), totalCount, perPage, r.URL.Path, r.URL.Query())
	data["pagination"] = pager

	filter.Limit = pager.ItemsPerPage
	filter.Offset = pager.Offset
	threads, err := c.threads.ThreadList(ctx, filter)
	if err != nil {
		return fmt.Errorf("listing threads (user %d): %w", user.ID, err)
	}
	// Mark unread threads.
	for i := range *threads {
		t := &(*threads)[i]
		for _, m := range t.Members {
			if m.UserID == user.ID && m.IsNew {
				t.IsUnread = true
				break
			}
		}
	}
	data["threads"] = threads
	return nil
}
