package subscriptions

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// tabURL - the viewer's Notifications tab.
const tabURL = "/me/" + manifest.TabPath

// perPage - the tab's notifications on a page.
const perPage = 25

// Controller - the module's pages: following content and terms, the
// Notifications tab, a notification's link, the e-mails' unsubscribe link.
type Controller struct {
	*modulekit.BaseController
	repo     *Repo
	describe describeDeps
	nodes    interface {
		ByID(ctx context.Context, nodeID int) (*modulekit.Node, error)
	}
	terms interface {
		TermByID(ctx context.Context, id int) (*modulekit.TaxonomyTerm, error)
	}
}

// NewController - the module's pages on base and repo; d names the
// notifications' items, nodes and terms load what is followed.
func NewController(base *modulekit.BaseController, repo *Repo, d describeDeps, nodes interface {
	ByID(ctx context.Context, nodeID int) (*modulekit.Node, error)
}, terms interface {
	TermByID(ctx context.Context, id int) (*modulekit.TaxonomyTerm, error)
}) *Controller {
	return &Controller{BaseController: base, repo: repo, describe: d, nodes: nodes, terms: terms}
}

// FollowState - the subscription button of a content item's comments or a
// term (comments_actions_subscriptions, term_actions_subscriptions).
type FollowState struct {
	SignedIn  bool
	Following bool
	// Action - where the button posts; Back - where to come back.
	Action string
	Back   string
}

// followState - userID's (ctx's) state of t, the form posting to action.
func (c *Controller) followState(ctx context.Context, t Target, action, back string) FollowState {
	st := FollowState{Action: action, Back: back}
	userID, ok := modulekit.ContextUserID(ctx)
	if !ok {
		return st
	}
	st.SignedIn = true
	on, err := c.repo.Subscribed(ctx, userID, t)
	modulekit.LogFallback(c.Logger(), err, "the subscription state (shown as not following)", "user_id", userID)
	st.Following = on
	return st
}

// NodePage - the module's part of a content page (AddNodePage): its
// comments' subscription button, on public content only (the others'
// comments tell nobody).
func (c *Controller) NodePage(ctx context.Context, node modulekit.Node) modulekit.NodePageParts {
	if !node.IsPublic() {
		return modulekit.NodePageParts{}
	}
	return modulekit.NodePageParts{Data: c.followState(ctx, Target{NodeID: node.ID},
		fmt.Sprintf("/subscriptions/follow/node/%d", node.ID), node.URL()+"#comments")}
}

// TermPage - the module's part of a term's page (AddTermPage): its follow
// button.
func (c *Controller) TermPage(ctx context.Context, term modulekit.TaxonomyTerm) any {
	return c.followState(ctx, Target{TermID: term.ID}, fmt.Sprintf("/subscriptions/follow/term/%d", term.ID), "/"+term.Slug)
}

// unreadCount - the viewer's unread notifications (0 - none, or they
// can't be counted).
func (c *Controller) unreadCount(ctx context.Context, viewer *modulekit.User) int {
	n, err := c.repo.UnreadCount(ctx, viewer.ID)
	if err != nil {
		modulekit.LogFallback(c.Logger(), err, "counting unread notifications (none shown)", "user_id", viewer.ID)
		return 0
	}
	return n
}

// PageData - every page's data (AddPageData): the signed-in user's unread
// notifications, data["subscriptionsUnread"] (the user menu's item) and
// on the user menu's toggle.
func (c *Controller) PageData(r *http.Request, data map[string]any) {
	user, _ := data["user"].(*modulekit.User)
	if user == nil {
		return
	}
	n := c.unreadCount(r.Context(), user)
	data["subscriptionsUnread"] = n
	modulekit.AddUserMenuCount(data, n)
}

// ProfileTab - the Notifications tab: the viewer's own only.
func (c *Controller) ProfileTab() modulekit.ProfileTab {
	return modulekit.ProfileTab{
		Module: manifest.Name,
		Path:   manifest.TabPath,
		Label:  i18n.N("Notifications"),
		Allowed: func(_ context.Context, viewer, _ *modulekit.User, isOwner, _ bool) bool {
			return viewer != nil && isOwner
		},
		Badge: c.unreadCount,
		Load:  c.loadTab,
	}
}

// SubscriptionView - a subscription as the tab lists it.
type SubscriptionView struct {
	Subscription
	Title string
	URL   string
	// Kind - the term's vocabulary, or empty for content.
	Kind string
}

// loadTab - a page of the user's notifications (the unread comments of a
// content item as one) and what they follow.
func (c *Controller) loadTab(ctx context.Context, _ http.ResponseWriter, r *http.Request, user *modulekit.User, data map[string]any) error {
	if seo, ok := data["pageSEO"].(modulekit.PageSEO); ok {
		seo.NoIndex = true
		data["pageSEO"] = seo
	}
	page := c.GetPageNumber(r)
	ns, total, err := c.repo.Notifications(ctx, user.ID, (page-1)*perPage, perPage)
	if err != nil {
		return err
	}
	items, err := describe(ctx, c.describe, ns, true)
	if err != nil {
		return err
	}
	data["subscriptionsItems"] = items
	data["pagination"] = pagination.New(page, total, perPage, tabURL, r.URL.Query())
	subs, err := c.subscriptionViews(ctx, user.ID)
	if err != nil {
		return err
	}
	data["subscriptionsList"] = subs
	return nil
}

// subscriptionViews - what userID follows, with names: content first.
func (c *Controller) subscriptionViews(ctx context.Context, userID int) ([]SubscriptionView, error) {
	subs, err := c.repo.UserSubscriptions(ctx, userID)
	if err != nil {
		return nil, err
	}
	var nodeIDs, termIDs []int
	for _, s := range subs {
		if s.NodeID > 0 {
			nodeIDs = append(nodeIDs, s.NodeID)
		} else {
			termIDs = append(termIDs, s.TermID)
		}
	}
	nodes, err := c.describe.nodes.ByIDs(ctx, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("the subscriptions' content: %w", err)
	}
	terms, err := c.describe.terms.TermsByIDs(ctx, termIDs)
	if err != nil {
		return nil, fmt.Errorf("the subscriptions' terms: %w", err)
	}
	var content, others []SubscriptionView
	for _, s := range subs {
		if s.NodeID > 0 {
			if n, ok := nodes[s.NodeID]; ok {
				content = append(content, SubscriptionView{Subscription: s, Title: n.Title, URL: n.URL()})
			}
		} else if t, ok := terms[s.TermID]; ok {
			others = append(others, SubscriptionView{Subscription: s, Title: t.Name, URL: "/" + t.Slug, Kind: t.Vocabulary.Name})
		}
	}
	return append(content, others...), nil
}

// back - where a form comes back to: its "back" field when it is a path
// of the site, else fallback.
func back(r *http.Request, fallback string) string {
	if p := modulekit.SafeRedirectPath(r.FormValue("back")); p != "" {
		return p
	}
	return fallback
}

// Follow handles POST /subscriptions/follow/{kind}/{id} (kind: node or term):
// action "unsubscribe" stops following, else follows (public content only:
// the tab shows the titles of what the user follows); back to the page.
func (c *Controller) Follow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := modulekit.ContextUserID(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	off := r.FormValue("action") == "unsubscribe"
	var t Target
	switch r.PathValue("kind") {
	case "node":
		node, err := c.nodes.ByID(ctx, id)
		if err != nil {
			c.RenderLoadFailure(w, r, err, "loading the content to follow", "nid", id)
			return
		}
		if !off && !node.IsPublic() {
			c.RenderNotFound(w, r)
			return
		}
		t.NodeID = id
	case "term":
		if _, err := c.terms.TermByID(ctx, id); err != nil {
			c.RenderLoadFailure(w, r, err, "loading the term to follow", "tid", id)
			return
		}
		t.TermID = id
	default:
		c.RenderNotFound(w, r)
		return
	}
	change := c.repo.Subscribe
	if off {
		change = c.repo.Unsubscribe
	}
	if err := change(ctx, userID, t); err != nil {
		c.LogFailure(err, "error changing a subscription", "user_id", userID)
	}
	http.Redirect(w, r, back(r, "/"), http.StatusSeeOther)
}

// Unfollow handles POST /subscriptions/delete/{id}: the tab's
// unsubscribe button.
func (c *Controller) Unfollow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := modulekit.ContextUserID(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	s, err := c.repo.SubscriptionByID(ctx, userID, id)
	if err != nil {
		c.LogFailure(err, "error loading a subscription", "id", id)
	} else if s != nil {
		if err := c.repo.Unsubscribe(ctx, userID, s.Target); err != nil {
			c.LogFailure(err, "error unsubscribing", "id", id)
		}
	}
	http.Redirect(w, r, tabURL, http.StatusSeeOther)
}

// Open handles GET /notifications/{id}: the notification (with the rest
// of its group) read, to what it tells of. A guest signs in first.
func (c *Controller) Open(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := modulekit.ContextUserID(r.Context())
	if !ok {
		http.Redirect(w, r, "/login?destination="+url.QueryEscape(r.URL.Path), http.StatusFound)
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	n, err := c.repo.Notification(ctx, userID, id)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading a notification", "id", id)
		return
	}
	if n == nil {
		c.RenderNotFound(w, r)
		return
	}
	if err := c.repo.MarkRead(ctx, *n); err != nil {
		c.LogFailure(err, "error marking a notification read", "id", id)
	}
	node, err := c.nodes.ByID(ctx, n.NodeID)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the content of a notification", "id", id)
		return
	}
	target := node.URL()
	if n.CommentID > 0 {
		target += fmt.Sprintf("#comment-%d", n.CommentID)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// ReadAll handles POST /notifications/read: every notification of the
// user read; back to the tab.
func (c *Controller) ReadAll(w http.ResponseWriter, r *http.Request) {
	userID, ok := modulekit.ContextUserID(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := c.repo.MarkAllRead(r.Context(), userID); err != nil {
		c.LogFailure(err, "error marking the notifications read", "user_id", userID)
	}
	http.Redirect(w, r, tabURL, http.StatusSeeOther)
}

// Unsubscribe handles GET and POST /subscriptions/unsubscribe/{token}: an
// e-mail's unsubscribe link, no sign-in needed - GET asks, POST does it
// (a mail client opening links mustn't unsubscribe anyone).
func (c *Controller) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, err := c.repo.SubscriptionByToken(ctx, r.PathValue("token"))
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading a subscription by its token")
		return
	}
	if s == nil {
		c.RenderNotFound(w, r)
		return
	}
	data := c.GetCommonData(r)
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "Unsubscribe"), NoIndex: true}
	if s.NodeID > 0 {
		if n, err := c.nodes.ByID(ctx, s.NodeID); err == nil {
			data["subscriptionTitle"], data["subscriptionURL"] = n.Title, n.URL()
		}
	} else if t, err := c.terms.TermByID(ctx, s.TermID); err == nil {
		data["subscriptionTitle"], data["subscriptionURL"], data["subscriptionTerm"] = t.Name, "/"+t.Slug, true
	}
	if r.Method == http.MethodPost {
		if err := c.repo.Unsubscribe(ctx, s.UserID, s.Target); err != nil {
			c.LogFailure(err, "error unsubscribing by a link")
			data["unsubscribeFailed"] = true
		} else {
			data["unsubscribed"] = true
		}
	}
	c.RenderOrFail(w, r, "subscriptions/unsubscribe.html", data)
}
