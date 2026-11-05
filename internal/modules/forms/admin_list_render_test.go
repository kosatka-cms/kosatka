package forms

import (
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestFormsListRender - the forms list is an admin page (the admin's
// layout, not the site's): each form with its links, a delete button only
// for a form the user may delete; none - said so.
func TestFormsListRender(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	forms := []WebformListItem{
		{Webform: Webform{ID: 1, Node: modulekit.Node{ID: 7, Title: "Feedback", Slug: "feedback", Status: 1}}, WebformStats: WebformStats{Count: 2, Last: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)}},
		{Webform: Webform{ID: 2, Node: modulekit.Node{ID: 8, Title: "Survey", Slug: "survey"}}},
	}
	data := map[string]any{"title": "Forms", "forms": forms, "canDelete": map[int]bool{7: true}, "canCreate": true}
	out := testkit.RenderPageContent(t, "admin/forms.html", data)
	for _, want := range []string{
		`href="/node/7/form/analysis"`, `href="/node/7/form/submissions.csv"`,
		`href="/admin/forms/7/delete"`, `<span class="admin-pill">Unpublished</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("forms list lacks %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, `/admin/forms/8/delete`) {
		t.Error("a delete button for a form the user may not delete")
	}
	if actions := testkit.RenderPageBlock(t, "admin/forms.html", "page_actions", data); !strings.Contains(actions, `href="/create/form"`) {
		t.Errorf("no create button: %s", actions)
	}
	data["forms"] = []WebformListItem{}
	if out := testkit.RenderPageContent(t, "admin/forms.html", data); !strings.Contains(out, "No forms yet.") {
		t.Errorf("an empty list:\n%s", out)
	}
	page := testkit.RenderPageBlock(t, "admin/forms.html", "admin_header", map[string]any{"isAdmin": true, "pendingPeopleCount": 0})
	if !strings.Contains(page, `class="admin-shell"`) {
		t.Error("the forms list isn't in the admin shell")
	}
}

// TestFormsDeleteRender - the confirmation names the form and its
// submissions and posts to the delete address.
func TestFormsDeleteRender(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	data := map[string]any{"title": "Deleting a form", "question": "Delete the form \"Feedback\"?", "submissions": 3, "action": "/admin/forms/7/delete"}
	out := testkit.RenderPageContent(t, "admin/forms_delete.html", data)
	for _, want := range []string{`action="/admin/forms/7/delete"`, "Its 3 submissions go with it.", "Delete the form &#34;Feedback&#34;?", `href="/admin/forms"`} {
		if !strings.Contains(out, want) {
			t.Errorf("confirmation lacks %s:\n%s", want, out)
		}
	}
}
