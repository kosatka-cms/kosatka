package complaints

import (
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/complaints/manifest"
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

func wantNone(t *testing.T, what, out string, bad ...string) {
	t.Helper()
	for _, b := range bad {
		if strings.Contains(out, b) {
			t.Errorf("%s has %q:\n%s", what, b, out)
		}
	}
}

// TestCommentLinkRender - "Report" under someone else's comment for a
// signed-in visitor (the core's comment template, moduleHook
// "comment_links"; in the comments' list rendered alone too), with what
// the window says it is; none under their own comment, one waiting for
// approval, for a guest, or with the module off.
func TestCommentLinkRender(t *testing.T) {
	useModule(t, true)
	comment := &modulekit.Comment{ID: 12, CreatedBy: 3, Content: "<p>Cheap <b>boats</b> &amp; sails</p>", Author: &modulekit.User{ID: 3, Username: "ann", Slug: "users/ann"}}
	render := func(data map[string]any) string {
		data["Comment"], data["CommentMode"] = comment, "open"
		return testkit.RenderPageBlock(t, "dynamic/page.html", "comment", data)
	}
	bob := &modulekit.User{ID: 4, Username: "bob"}
	wantAll(t, "another's comment", render(map[string]any{"currentUser": bob}),
		`<li class="comment-complain">`, `data-complain="comment" data-complain-id="12"`, `data-complain-about="ann: &quot;Cheap boats &amp; sails&quot;"`, ">Report</a>")
	for name, data := range map[string]map[string]any{
		"own comment": {"currentUser": &modulekit.User{ID: 3}},
		"held":        {"currentUser": bob, "Held": true},
		"guest":       {},
	} {
		wantNone(t, name, render(data), "data-complain")
	}
	// The comments' list as /api/comments renders it alone.
	list := testkit.RenderNamed(t, "comments-list", map[string]any{"comments": []*modulekit.Comment{comment}, "currentUser": bob, "commentMode": "open", "pagination": pagination.New(1, 1, 50, "/harbour", nil)},
		"comments.html", "pagination.html")
	wantAll(t, "the comments' list", list, `data-complain="comment" data-complain-id="12"`)
	useModule(t, false)
	wantNone(t, "the module off", render(map[string]any{"currentUser": bob}), "data-complain")
}

// TestNodeLinkRender - "Report this content" under a content item the
// module's page part names (Controller.NodePage), the title escaped.
func TestNodeLinkRender(t *testing.T) {
	useModule(t, true)
	data := map[string]any{"moduleNode": map[string]any{manifest.Name: nodeLink{ID: 7, Title: `Harbour <img src=x>`}}}
	out := testkit.RenderNamed(t, "node_links_complaints", data, "partials/complaints_hooks.html")
	wantAll(t, "the content's link", out, `data-complain="node" data-complain-id="7"`, `data-complain-about="&quot;Harbour &lt;img src=x&gt;&quot;"`, "Report this content")
	wantNone(t, "the content's link", out, "<img")
	if out := testkit.RenderNamed(t, "node_links_complaints", map[string]any{}, "partials/complaints_hooks.html"); strings.Contains(out, "data-complain") {
		t.Errorf("no page part, a link: %s", out)
	}
}

// TestDialogAtPageEnd - the page ends with the complaint's window (a
// <template>: the reasons, "Other" named, whether comments take
// complaints) for a visitor who may report (complaintsReasons); nothing
// without, nor with the module off.
func TestDialogAtPageEnd(t *testing.T) {
	useModule(t, true)
	render := func(reasons []string) string {
		data := testkit.PageData()
		data["CSRFToken"] = "tok"
		if reasons != nil {
			data["complaintsReasons"], data["complaintsOther"], data["complaintsComments"] = reasons, OtherReason, true
		}
		return testkit.RenderPageBlock(t, "errors/not_found.html", "layout.html", data)
	}
	out := render(Settings{Reasons: "Off topic"}.ReasonList())
	wantAll(t, "the page", out, `<template id="complaint-dialog" data-comments="1" data-other="Other"`,
		`name="reason" value="Off topic"> Off topic`, `name="reason" value="Other"> Other`, `name="note"`)
	wantNone(t, "no reasons", render(nil), "complaint-dialog")
	useModule(t, false)
	wantNone(t, "the module off", render([]string{"Spam"}), "complaint-dialog")
}

// queuePage - the queue's page of rows (Controller.Queue).
func queuePage(t *testing.T, tab string, rows []queueRow) string {
	t.Helper()
	return testkit.RenderPageContent(t, "admin/complaints.html", map[string]any{"rows": rows, "tab": tab,
		"pagination": pagination.New(1, len(rows), queuePerPage, manifest.AdminQueuePath, nil)})
}

// TestQueueRender - a comment's row (its excerpt and author escaped:
// visitors wrote them) with Spam, Unpublish, Delete (asked first), Block
// the author, Keep; a content item hidden by the complaints - its type,
// the mark, Unpublish (it stays so), Delete and "Show again" for Keep;
// the reasons with their counts, the notes; a row whose item is gone
// only Keep; the resolved tab - what was done and by whom, no buttons.
func TestQueueRender(t *testing.T) {
	useModule(t, true)
	at := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	rows := []queueRow{
		{Target: Target{ID: 5, Kind: KindComment, ObjectID: 12, AuthorID: 3, Status: StatusNew, Count: 4, LastAt: at,
			Reasons: []ReasonCount{{Reason: "Spam or advertising", Count: 3}, {Reason: "Insults", Count: 1}},
			Notes:   []Note{{Text: "<script>alert(1)</script>", At: at}}},
			Title: "Harbour", URL: "/harbour#comment-12", Excerpt: "<img src=x onerror=alert(1)>", AuthorName: "ann", AuthorURL: "/users/ann"},
		{Target: Target{ID: 6, Kind: KindNode, ObjectID: 7, AuthorID: 4, Status: StatusNew, Hidden: true, Count: 2, LastAt: at},
			Title: "Sails", URL: "/sails", TypeLabel: "Article", AuthorName: "bob", AuthorURL: "/users/bob"},
		{Target: Target{ID: 8, Kind: KindComment, ObjectID: 13, Status: StatusNew, Count: 1, LastAt: at}, Gone: true},
	}
	out := queuePage(t, StatusNew, rows)
	wantAll(t, "the queue", out,
		`class="nav-link active" href="/admin/content/complaints" aria-current="page"`,
		`<a href="/users/ann">ann</a>`, "\"&lt;img src=x onerror=alert(1)&gt;\"", "&lt;script&gt;", "Spam or advertising · 3", "Insults · 1",
		`action="/admin/content/complaints/5/spam"`, `action="/admin/content/complaints/5/unpublished"`,
		`action="/admin/content/complaints/5/deleted" data-confirm=`, `href="/user/3/edit"`, `action="/admin/content/complaints/5/kept"`,
		"Content · Article", "Hidden by the complaints", `action="/admin/content/complaints/6/unpublished"`, `action="/admin/content/complaints/6/deleted"`,
		"Show again", "Gone already", `action="/admin/content/complaints/8/kept"`)
	wantNone(t, "the queue", out, "<img src=x", "<script>", "/6/spam", "/8/deleted", "/8/unpublished")

	resolvedAt := at.Add(time.Hour)
	out = queuePage(t, StatusResolved, []queueRow{{Target: Target{ID: 6, Kind: KindNode, Status: StatusResolved, Resolution: ResolutionKept, ResolvedAt: &resolvedAt, Count: 2, LastAt: at},
		Title: "Sails", URL: "/sails", Decision: "Kept", ResolvedByName: "mod"}})
	wantAll(t, "the resolved tab", out, `href="/admin/content/complaints?tab=resolved" aria-current="page"`, `admin-pill--ok">Kept</span>`, "mod · ")
	wantNone(t, "the resolved tab", out, "<form method=\"post\" action=\"/admin/content/complaints/6")
	if out := queuePage(t, StatusNew, nil); !strings.Contains(out, "No new complaints") {
		t.Errorf("an empty queue: %s", out)
	}
}

// TestSettingsRender - the settings' form: comments and the content
// types chosen checked, the reasons with the defaults as the
// placeholder, the hiding choices, the link to the e-mail's text.
func TestSettingsRender(t *testing.T) {
	useModule(t, true)
	labels := map[string]string{"article": "Article", "page": "Page"}
	out := testkit.RenderPageContent(t, "admin/complaints_settings.html", map[string]any{
		"settings": Settings{Comments: true, NodeTypes: []string{"article"}, Reasons: "Off topic", HideAfter: 5},
		"types":    modulekit.NodeTypesByLabel(labels), "typeLabels": labels, "hideAfterChoices": hideAfterChoices,
		"reasonsPlaceholder": "Spam\nInsults", "mailURL": modulekit.MailTemplatePath(MailNewComplaint)})
	wantAll(t, "the settings", out, `name="comments" value="1" checked`, `name="node_types" value="article" checked> Article`,
		`name="node_types" value="page"> Page`, "placeholder=\"Spam\nInsults\">Off topic</textarea>", `<option value="5" selected>after 5 complaints</option>`,
		`<option value="0">don't hide</option>`, `href="/admin/config/mail/templates/complaints.new_complaint"`)
}

// TestTemplatesEmbedded - on a site the module's templates come from the
// binary (Assets, testkit.UseModuleAssets).
func TestTemplatesEmbedded(t *testing.T) {
	useModule(t, true)
	testkit.UseModuleAssets(t, map[string]fs.FS{manifest.Name: Assets})
	if p, err := testkit.ResolveTemplate("admin/complaints.html"); err != nil || !strings.HasPrefix(p, "embed:") {
		t.Fatalf("the module's template: %q %v, want the embedded one", p, err)
	}
	if out := queuePage(t, StatusNew, nil); !strings.Contains(out, "No new complaints") {
		t.Errorf("rendered from the embedded template: %s", out)
	}
}
