package actionlog

import (
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/action_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestListPage - an entry shows who (a link to their actions) and from
// where, the action in its tone's colour, the object linked while it
// exists, what changed folded; an entry by nobody signed in, a deleted
// object without a link.
func TestListPage(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	at := time.Date(2026, 10, 25, 19, 40, 0, 0, time.UTC)
	rows := []row{
		{Entry: Entry{At: at, ActorID: 3, ActorName: "ann", IP: "203.0.113.5",
			Object:  modulekit.ActionObject{Kind: "role", ID: "editor", Label: "Editor", URL: "/admin/users/permissions"},
			Changes: []modulekit.ActionChange{{Field: "Delete any content", From: "no", To: "yes"}}},
			Action: "Permissions changed", Tone: modulekit.ActionToneAccess, Kind: "Role"},
		{Entry: Entry{At: at, Object: modulekit.ActionObject{Kind: "node", ID: "12", Label: "Old <post>"}}, Action: "Content deleted", Tone: modulekit.ActionToneDelete, Kind: "Article"},
		{Entry: Entry{At: at, Object: modulekit.ActionObject{Kind: "user", Label: "nobody"}, Changes: []modulekit.ActionChange{{Field: "Reason", To: "no such user"}}},
			Action: "Failed sign-in", Tone: modulekit.ActionToneFail, Kind: "User"},
	}
	out := testkit.RenderPageContent(t, "admin/action_log.html", map[string]any{"rows": rows, "total": 2, "keepDays": 90,
		"filter": Filter{}, "groups": groups(), "pagination": pagination.New(1, 2, perPage, manifest.AdminPath, nil)})
	for _, want := range []string{
		`href="/admin/reports/action-log?actor_id=3" class="fw-semibold">ann</a>`, "203.0.113.5",
		`action-log-badge action-log-badge--access">Permissions changed`, `<a href="/admin/users/permissions">Editor</a>`,
		"Role · ID editor", "<summary>What changed</summary>", `<td class="action-log-to">yes</td>`,
		`action-log-badge--delete">Content deleted`, "Old &lt;post&gt;", "Article · ID 12",
		`<option value="content">Content</option>`,
		`<span class="admin-table-sub">Reason: no such user</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if strings.Count(out, "<summary>") != 1 {
		t.Error("a single reason is a line, not a fold")
	}
	if strings.Contains(out, `<a href="">`) {
		t.Error("a deleted object must not be a link")
	}
}

// TestSettingsPage - how long entries are kept, the address switch, a
// switch per group as saved.
func TestSettingsPage(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	st := Settings{KeepDays: 180, GroupsOff: []string{"logins"}}
	var choices []groupChoice
	for _, g := range groups() {
		choices = append(choices, groupChoice{ActionGroup: g, On: !st.Off(g.Name)})
	}
	out := testkit.RenderPageContent(t, "admin/action_log_settings.html", map[string]any{"settings": st, "groups": choices, "keepChoices": keepChoices})
	for _, want := range []string{
		`<option value="180" selected>180 days</option>`, `<option value="0">Forever</option>`,
		`id="log_ip" name="log_ip" value="1" class="form-check-input">`,
		`id="group_content" name="groups" value="content" class="form-check-input" checked>`,
		`id="group_logins" name="groups" value="logins" class="form-check-input">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}

// TestHooks - the content form's "Log" row with the count and the
// content's own log; the users list's log icon only for those who may see
// the log.
func TestHooks(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	out := testkit.RenderNamed(t, "node_settings_action_log", map[string]any{"moduleSettings": map[string]any{"action_log": nodeLog{Count: 4, URL: ObjectLink("node", "12")}}}, "partials/action_log_hooks.html")
	if !strings.Contains(out, `href="/admin/reports/action-log?id=12&amp;kind=node"`) || !strings.Contains(out, "4 entries") {
		t.Errorf("node row: %s", out)
	}
	user := modulekit.User{ID: 5}
	if out := testkit.RenderNamed(t, "admin_people_actions_action_log", map[string]any{"User": user, "Ctx": map[string]any{"actionLogCanView": true}}, "partials/action_log_hooks.html", "admin/icons.html"); !strings.Contains(out, `href="/admin/reports/action-log?actor_id=5"`) || !strings.Contains(out, `class="admin-icon-btn"`) || !strings.Contains(out, "<svg") {
		t.Errorf("people icon: %s", out)
	}
	if out := testkit.RenderNamed(t, "admin_people_actions_action_log", map[string]any{"User": user, "Ctx": map[string]any{}}, "partials/action_log_hooks.html", "admin/icons.html"); strings.Contains(out, "<a") {
		t.Errorf("no permission, no button: %s", out)
	}
}
