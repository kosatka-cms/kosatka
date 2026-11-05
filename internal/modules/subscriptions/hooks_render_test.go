package subscriptions

import (
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// useModule - the engine's templates with the module on (it is off by
// default) or off for the test.
func useModule(t *testing.T, on bool) {
	t.Helper()
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: on})
}

func wantAll(t *testing.T, what, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("%s has no %q:\n%s", what, want, out)
		}
	}
}

// TestUserMenuHooksRender - "Notifications" in the site's and the
// admin's user menu with the unread count, the site's one marked for the
// mobile menu; the module off - no item.
func TestUserMenuHooksRender(t *testing.T) {
	useModule(t, true)
	data := map[string]any{"user": &modulekit.User{ID: 1, Username: "ann"}, "subscriptionsUnread": 3, "userMenuCount": 3}
	wantAll(t, "the user menu", testkit.RenderPageBlock(t, "user/profile_page.html", "block_user_menu", data),
		`href="/me/notifications" class="dropdown-item dropdown-item-notifications" data-mobile-user-menu`, `<span class="badge bg-danger ms-1">3</span>`)
	wantAll(t, "the admin header", testkit.RenderPageBlock(t, "user/profile_page.html", "admin_header", data), `href="/me/notifications"`)

	useModule(t, false)
	for _, block := range []string{"block_user_menu", "admin_header"} {
		if out := testkit.RenderPageBlock(t, "user/profile_page.html", block, data); strings.Contains(out, "/me/notifications") {
			t.Errorf("the module off, %s has the notifications item", block)
		}
	}
}

// TestFollowButtonsRender - the comments' subscription button of a
// content page and the term page's "Follow": a form posting to the
// state's action, back to the page; once following, the state and the
// button undoing it; a guest is offered to log in and come back.
func TestFollowButtonsRender(t *testing.T) {
	useModule(t, true)
	node := func(s FollowState) map[string]any {
		return map[string]any{"moduleNode": map[string]any{manifest.Name: s}}
	}
	state := FollowState{SignedIn: true, Action: "/subscriptions/follow/node/7", Back: "/harbour#comments"}
	wantAll(t, "the comments' button", testkit.RenderPageBlock(t, "dynamic/page.html", "comments-placeholder", node(state)),
		`<form method="post" action="/subscriptions/follow/node/7"`, `name="back" value="/harbour#comments"`, "Subscribe to the comments")

	state.Following = true
	out := testkit.RenderPageBlock(t, "dynamic/page.html", "comments-placeholder", node(state))
	wantAll(t, "the comments' button, following", out, `name="action" value="unsubscribe"`, "You are subscribed", "Unsubscribe")

	out = testkit.RenderPageBlock(t, "dynamic/page.html", "comments-placeholder", node(FollowState{Back: "/harbour#comments"}))
	wantAll(t, "a guest's button", out, `href="/login?destination=%2Fharbour%23comments"`, "Log in to subscribe")
	if strings.Contains(out, "<form") {
		t.Error("a guest got the form")
	}

	term := map[string]any{"moduleTerm": map[string]any{manifest.Name: FollowState{SignedIn: true, Action: "/subscriptions/follow/term/3", Back: "/sailing"}}}
	wantAll(t, "the term's button", testkit.RenderPageBlock(t, "taxonomy/term.html", "term_actions_subscriptions", term),
		`action="/subscriptions/follow/term/3"`, "Follow")

	useModule(t, false)
	if out := testkit.RenderPageBlock(t, "dynamic/page.html", "comments-placeholder", node(state)); strings.Contains(out, "subscriptions-follow") {
		t.Error("the module off, the comments have its button")
	}
}

// TestProfileTabRender - the Notifications tab: the items (the unread
// marked, a group of comments counted with its authors, a reply quoting
// the comment) linking through the notification, "mark all as read" with
// unread ones, what the user follows with the buttons undoing it.
func TestProfileTabRender(t *testing.T) {
	useModule(t, true)
	at := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	data := map[string]any{
		"profileUser": &modulekit.User{ID: 1, Username: "ann", Slug: "users/ann"}, "isCurrentUser": true,
		"activeTab": "module", "activeModuleTab": manifest.Name,
		"profileModuleTabs":   []map[string]any{{"Label": "Notifications", "URL": "/me/notifications", "Badge": 2, "Active": true}},
		"subscriptionsUnread": 2,
		"subscriptionsItems": []Item{
			{Notification: Notification{ID: 9, Kind: KindComment, CreatedAt: at}, Count: 2, Actors: []string{"bob", "cat"}, NodeTitle: "Harbour"},
			{Notification: Notification{ID: 5, Kind: KindReply, CreatedAt: at, Read: true}, Count: 1, Actor: "bob", NodeTitle: "Harbour", Excerpt: "Fair winds"},
		},
		"subscriptionsList": []SubscriptionView{
			{Subscription: Subscription{ID: 4}, Title: "Harbour", URL: "/harbour"},
			{Subscription: Subscription{ID: 6}, Title: "Sailing", URL: "/sailing", Kind: "Tags"},
		},
		"pagination": pagination.New(1, 2, perPage, "/me/notifications", nil),
	}
	out := testkit.RenderPageContent(t, "user/profile_page.html", data)
	wantAll(t, "the tab", out,
		`<li class="subscriptions-item is-unread">`, `href="/notifications/9"`, "<strong>2 new comments</strong> on \"Harbour\"", "bob, cat",
		`<li class="subscriptions-item">`, "<strong>bob</strong> replied to your comment on \"Harbour\"", "\"Fair winds\"",
		`action="/notifications/read"`, `href="/harbour"`, `action="/subscriptions/delete/4"`, "· Tags", `action="/subscriptions/delete/6"`,
		`href="/me/edit#subscriptions-settings"`)

	data["subscriptionsItems"], data["subscriptionsList"], data["subscriptionsUnread"] = nil, nil, 0
	out = testkit.RenderPageContent(t, "user/profile_page.html", data)
	wantAll(t, "the empty tab", out, "No notifications yet.", "You follow nothing yet")
	if strings.Contains(out, `action="/notifications/read"`) {
		t.Error("nothing unread, but \"mark all as read\" is shown")
	}
}

// TestUnsubscribeRender - the unsubscribe link's page asks with a form
// posting back; done, it says so; a term's words differ.
func TestUnsubscribeRender(t *testing.T) {
	useModule(t, true)
	data := map[string]any{"subscriptionTitle": "Harbour", "subscriptionURL": "/harbour"}
	wantAll(t, "the question", testkit.RenderPageContent(t, "subscriptions/unsubscribe.html", data),
		"Unsubscribe from the comments of \"Harbour\"?", `<form method="post">`, `href="/harbour"`)
	data["unsubscribed"], data["subscriptionTerm"] = true, true
	out := testkit.RenderPageContent(t, "subscriptions/unsubscribe.html", data)
	wantAll(t, "the answer", out, "You no longer follow \"Harbour\".")
	if strings.Contains(out, "<form") {
		t.Error("done, the form is still shown")
	}
}

// TestSettingsSectionsRender - the profile form's notification settings
// (marked shown, the user's choices checked); the module off - none.
func TestSettingsSectionsRender(t *testing.T) {
	useModule(t, true)
	profile := map[string]any{
		"editUser": &modulekit.User{ID: 2, Username: "ann", Slug: "users/ann"}, "isSelf": true,
		"moduleSettings": map[string]any{manifest.Name: UserSettings{Mail: MailDaily, Replies: true}},
	}
	out := testkit.RenderPageContent(t, "user/edit_form.html", profile)
	wantAll(t, "the profile form", out, `name="subscriptions_user_settings" value="1"`,
		`name="subscriptions_mail" value="daily" checked`, `name="subscriptions_replies" value="1" checked`)
	if strings.Contains(out, `name="subscriptions_auto_own" value="1" checked`) {
		t.Error("an automatic subscription turned off is checked")
	}

	useModule(t, false)
	if out := testkit.RenderPageContent(t, "user/edit_form.html", profile); strings.Contains(out, "subscriptions_mail") {
		t.Error("the module off, the profile form has its settings")
	}
}
