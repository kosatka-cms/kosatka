package privatemessages

import (
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// renderBlock - one define of the profile page's template set (the
// core's partials and the modules').
func renderBlock(t *testing.T, block string, data map[string]any) string {
	t.Helper()
	testkit.UseEngineRoot(t)
	return testkit.RenderPageBlock(t, "user/profile_page.html", block, data)
}

// TestUserMenuHooksRender - "Messages" in the site's and the admin's user
// menu (moduleHook "user_menu", "admin_user_menu") with the unread count,
// the site's one marked for the mobile menu; the toggle shows the user
// menu's number (userMenuCount). The module off - no item.
func TestUserMenuHooksRender(t *testing.T) {
	data := map[string]any{"user": &modulekit.User{ID: 1, Username: "ann"}, "unreadCount": 2, "userMenuCount": 2}
	menu := renderBlock(t, "block_user_menu", data)
	for _, want := range []string{`href="/me/messages" class="dropdown-item dropdown-item-messages" data-mobile-user-menu`, `<span class="badge bg-danger ms-1">2</span>`} {
		if !strings.Contains(menu, want) {
			t.Errorf("the user menu has no %q:\n%s", want, menu)
		}
	}
	if admin := renderBlock(t, "admin_header", data); !strings.Contains(admin, `href="/me/messages"`) || !strings.Contains(admin, `<span class="badge bg-danger ms-1">2</span>`) {
		t.Errorf("the admin header has no messages item or count:\n%s", admin)
	}

	withModule(t, false)
	for _, block := range []string{"block_user_menu", "admin_header"} {
		if out := renderBlock(t, block, map[string]any{"user": &modulekit.User{ID: 1, Username: "ann"}}); strings.Contains(out, "/me/messages") {
			t.Errorf("the module off, %s has the messages item", block)
		}
	}
}

// TestProfileTabRender - the profile page renders the active module tab
// with the module's "profile_tab_private_messages": the threads (the last
// message as plain text), the unread one marked, the archive form; another user's tab (an
// administrator of users) - the list on its own address, no thread links,
// archiving or writing.
func TestProfileTabRender(t *testing.T) {
	testkit.UseEngineRoot(t)
	data := func(tabURL string, own bool) map[string]any {
		return map[string]any{
			"profileUser": &modulekit.User{ID: 1, Username: "ann", Slug: "users/ann"}, "isCurrentUser": own,
			"activeTab": "module", "activeModuleTab": "private_messages",
			"profileModuleTabs": []map[string]any{{"Label": "Messages", "URL": tabURL, "Badge": 1, "Active": true}},
			"threads": &Threads{{ID: 5, Subject: "Hello there", IsUnread: true,
				LastMessage: &Message{Body: "<p>Tea &amp; cake</p>"}}},
			"filterStatus": "active", "pagination": pagination.New(1, 1, 25, tabURL, nil),
			"pmTabURL": tabURL, "pmOwn": own,
		}
	}
	out := testkit.RenderPageContent(t, "user/profile_page.html", data("/me/messages", true))
	for _, want := range []string{`href="/me/messages"`, `<span class="badge">1</span>`, `pm-container`, `href="/me/messages/thread/5"`, "Hello there", `action="/me/messages/archive"`, `href="/me/messages/new"`, "Tea &amp; cake"} {
		if !strings.Contains(out, want) {
			t.Errorf("the messages tab has no %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "&lt;p&gt;") {
		t.Error("the last message's preview shows its HTML tags")
	}

	out = testkit.RenderPageContent(t, "user/profile_page.html", data("/users/ann/messages", false))
	for _, want := range []string{`href="/users/ann/messages?status=archived`, `action="/users/ann/messages" class="pm-filters"`, "Hello there"} {
		if !strings.Contains(out, want) {
			t.Errorf("another user's tab has no %q", want)
		}
	}
	for _, unwanted := range []string{`/me/messages/thread/5`, `name="thread_ids[]"`, `href="/me/messages/new"`, `href="/me/messages?`} {
		if strings.Contains(out, unwanted) {
			t.Errorf("another user's tab has %q", unwanted)
		}
	}
}

// TestUserActionsAndRecipient - "Send private message" next to a user's
// name (its user_actions hook, the forum's topic author) opens the new
// message form with them chosen (?to=).
func TestUserActionsAndRecipient(t *testing.T) {
	testkit.UseEngineRoot(t)
	if out := testkit.RenderPageBlock(t, "user/profile_page.html", "user_actions_private_messages", &modulekit.User{ID: 7, Username: "bob"}); !strings.Contains(out, `href="/me/messages/new?to=7"`) {
		t.Errorf("no message link: %q", out)
	}

	out := testkit.RenderPageContent(t, "private_messages/new.html", map[string]any{"recipient": &modulekit.User{ID: 7, Username: "bob"}})
	if !strings.Contains(out, `<span class="user-autocomplete-tag">bob <button`) || !strings.Contains(out, `name="recipients[]" value="7"`) {
		t.Errorf("the form doesn't start with the recipient:\n%s", out)
	}
}

// TestMailDef - the new message e-mail on the Mail page: the module's,
// optional, its text kept by the module, a sample of each of its own
// placeholders.
func TestMailDef(t *testing.T) {
	def := NewSettingsRepo(nil).MailDef()
	if def.Key != MailNewMessage || def.Module != manifest.Name || !def.Optional || def.Load == nil || def.Save == nil {
		t.Errorf("the e-mail: %+v", def)
	}
	for _, p := range MailPlaceholders {
		if !slices.Contains(def.Placeholders, p) || def.Sample[p] == "" {
			t.Errorf("placeholder %s: listed %v, sample %q", p, slices.Contains(def.Placeholders, p), def.Sample[p])
		}
	}
}

// TestUserSettingsSectionRender - the profile form has the new message
// e-mail switch (moduleHook "user_settings"), marked shown; the module
// off - none.
func TestUserSettingsSectionRender(t *testing.T) {
	testkit.UseEngineRoot(t)
	data := map[string]any{
		"editUser":       &modulekit.User{ID: 2, Username: "ann", Slug: "users/ann"},
		"isSelf":         true,
		"moduleSettings": map[string]any{"private_messages": userSettingsForm{MailNewMessage: true}},
	}
	out := testkit.RenderPageContent(t, "user/edit_form.html", data)
	for _, want := range []string{`name="pm_user_settings" value="1"`, `name="pm_mail_new_message" value="1" checked`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in:\n%s", want, out)
		}
	}

	withModule(t, false)
	if out := testkit.RenderPageContent(t, "user/edit_form.html", data); strings.Contains(out, "pm_mail_new_message") {
		t.Error("the module off, the form has its switch")
	}
}
