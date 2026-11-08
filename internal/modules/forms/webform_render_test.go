package forms

import (
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// testWebformValues mirrors the controller's values (First/Has).
type testWebformValues map[int][]string

func (v testWebformValues) First(id int) string {
	if len(v[id]) > 0 {
		return v[id][0]
	}
	return ""
}

func (v testWebformValues) Has(id int, option string) bool {
	for _, s := range v[id] {
		if s == option {
			return true
		}
	}
	return false
}

// TestWebformDetailTemplate - the form's elements post as element_<id>
// with what was entered and the errors; after sending the confirmation is
// shown instead; a closed form shows no form.
func TestWebformDetailTemplate(t *testing.T) {
	testkit.UseEngineRoot(t)
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/contact", Title: "Contact", Status: 1, CommentMode: modulekit.CommentModeHidden},
		Description: "Write to us", Elements: testWebformElements()}
	data := func(extra map[string]any) map[string]any {
		d := map[string]any{"webform": wf, "item": wf, "values": testWebformValues{}, "fieldErrors": map[int]string{}}
		for k, v := range extra {
			d[k] = v
		}
		return d
	}
	out := testkit.RenderPageContent(t, "webform/detail.html", data(map[string]any{
		"values":      testWebformValues{1: {"Ann"}, 4: {"Games"}},
		"fieldErrors": map[int]string{2: "Enter a valid e-mail address.", 0: "Confirm that you are not a robot."},
	}))
	for _, want := range []string{
		`action="/node/9/form"`, `name="element_1" value="Ann"`, `type="email" id="element_2"`,
		`type="radio" id="element_3_0" name="element_3" value="Ads"`,
		`name="element_4" value="Games" checked`, "Enter a valid e-mail address.", "Confirm that you are not a robot.", "Write to us",
		// What forms.js replaces with its answer, the error to focus.
		`<div class="webform-body" data-webform-body>`, `class="alert alert-danger webform-error" role="alert" tabindex="-1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("form: no %q", want)
		}
	}
	if strings.Contains(out, "comments") {
		t.Error("hidden comments must not render")
	}
	// The node's actions come from the engine's parts; the submissions
	// link through node_actions_forms.
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	out = testkit.RenderPageContent(t, "webform/detail.html", data(map[string]any{"canEdit": true, "canDelete": true, "canViewSubmissions": true, "submissionsCount": 4}))
	for _, want := range []string{`href="/node/9/edit"`, `href="/node/9/form/submissions"`, "Submissions (4)", `action="/node/9/unpublish"`, `href="/node/9/delete"`} {
		if !strings.Contains(out, want) {
			t.Errorf("form actions: no %q", want)
		}
	}
	if out := testkit.RenderPageContent(t, "webform/detail.html", data(nil)); strings.Contains(out, "node-actions") {
		t.Error("actions for a visitor")
	}
	if out := testkit.RenderPageContent(t, "webform/detail.html", data(map[string]any{"sent": true})); !strings.Contains(out, "Thank you, the form has been sent.") || strings.Contains(out, "<form") ||
		!strings.Contains(out, `class="alert alert-success webform-confirmation" role="status" tabindex="-1"`) {
		t.Errorf("sent: %s", out)
	}
	if out := testkit.RenderPageContent(t, "webform/detail.html", data(map[string]any{"unavailable": "The form is closed."})); !strings.Contains(out, "The form is closed.") || strings.Contains(out, "<form") {
		t.Errorf("closed: %s", out)
	}
	out = testkit.RenderPageContent(t, "webform/detail.html", data(map[string]any{"unavailable": "Log in to fill in the form.", "loginRequired": true}))
	if !strings.Contains(out, `href="/login?destination=%2fform%2fcontact"`) || strings.Contains(out, "<form") {
		t.Errorf("login required: %s", out)
	}

	consent := *wf
	consent.Elements = append(WebformElements{}, wf.Elements...)
	consent.Elements = append(consent.Elements, WebformElement{ID: 5, Type: WebformConsent, Label: "I agree", Required: true, Options: []string{"/privacy"}})
	out = testkit.RenderPageContent(t, "webform/detail.html", map[string]any{"webform": &consent, "item": &consent, "values": testWebformValues{5: {"1"}}, "fieldErrors": map[int]string{}})
	for _, want := range []string{`type="checkbox" id="element_5" name="element_5" value="1" required checked`, `<a href="/privacy" target="_blank" rel="noopener">privacy policy</a>`} {
		if !strings.Contains(out, want) {
			t.Errorf("consent: no %q", want)
		}
	}
}

// TestWebformEditorTemplate - every element is a row keyed by its ID with
// its fields; the new-row template has the placeholder key. A row's
// "More" is open when one of its settings is set or it has an error; the
// page attaches the module (its stylesheet).
func TestWebformEditorTemplate(t *testing.T) {
	testkit.UseEngineRoot(t)
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/contact"}, Elements: testWebformElements()}
	form := WebformFormFromEntity(wf, time.UTC)
	form.Elements[1].Prefill = "email"
	out := testkit.RenderPageContent(t, "node/edit_webform.html", map[string]any{
		"webform": wf, "editNode": wf, "form": form, "elementTypes": WebformElementTypes,
		"errors":              map[string]string{"element_3": "Enter the options, one per line."},
		"canEditNodeSettings": true,
	})
	if !strings.Contains(out, `name="created_by"`) || strings.Contains(out, "galleries_enabled") {
		t.Error("a form's node settings: authoring yes, galleries no")
	}
	if all, open := strings.Count(out, `<details class="webform-element-more"`), strings.Count(out, `<details class="webform-element-more" open>`); all != 5 || open != 2 {
		t.Errorf("More: %d, open %d; want 5 (4 rows and the template), 2 open (a setting, an error)", all, open)
	}
	for _, want := range []string{
		`<!--kosatka:attach forms-->`,
		`action="/node/9/edit"`, `name="element_row" value="1"`, `name="element_label_1" value="Name"`,
		`id="element_required_1" name="element_required_1" value="1" checked`, `<option value="radios" selected>One of the options</option>`,
		"Ads\nErrors", "Enter the options, one per line.", `name="element_row" value="__KEY__"`, `name="element_prefill_2" value="email"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("editor: no %q", want)
		}
	}
}

// TestWebformSubmissionsTemplate - a row per submission with its sender (a
// link, or "Guest") and a column per element (several values on lines);
// the CSV link and paging.
func TestWebformSubmissionsTemplate(t *testing.T) {
	testkit.UseEngineRoot(t)
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/contact", Title: "Contact"}, Elements: testWebformElements()}
	out := testkit.RenderPageContent(t, "webform/submissions.html", map[string]any{
		"webform": wf, "total": 51, "page": 1, "hasNext": true,
		"submissions": []WebformSubmission{
			{ID: 2, Username: "ann", UserSlug: "users/ann", RemoteIP: "10.0.0.1", Values: map[int][]string{1: {"Ann"}, 4: {"News", "Games"}}},
			{ID: 1, Values: map[int][]string{1: {"Bob"}}},
		},
	})
	for _, want := range []string{
		`href="/node/9/form/submissions.csv"`, "51 submissions", "<th>Sections</th>", `<a href="/users/ann">ann</a>`,
		"Guest", "News<br>Games", `href="?page=2"`, `<a href="/node/9/form/submissions/2">2</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in %s", want, out)
		}
	}
	if strings.Contains(out, "?page=0") {
		t.Error("no newer page on page 1")
	}
}

// TestWebformConditionAttributes - an element with a condition carries the
// element and value forms.js watches.
func TestWebformConditionAttributes(t *testing.T) {
	testkit.UseEngineRoot(t)
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/c", Status: 1, CommentMode: modulekit.CommentModeHidden}, Elements: WebformElements{
		{ID: 1, MachineName: "topic", Type: WebformRadios, Label: "Topic", Options: []string{"Ads"}},
		{ID: 2, MachineName: "budget", Type: WebformText, Label: "Budget", VisibleIfElement: "topic", VisibleIfValue: "Ads"},
	}}
	out := testkit.RenderPageContent(t, "webform/detail.html", map[string]any{"webform": wf, "item": wf, "values": testWebformValues{}, "fieldErrors": map[int]string{}})
	if !strings.Contains(out, `data-visible-if="element_1" data-visible-value="Ads"`) || strings.Count(out, "data-visible-if") != 1 {
		t.Errorf("attributes: %s", out)
	}
}

// TestWebformFileTemplates - a form with a file element posts multipart
// with an input limited to its types; the submissions page links the sent
// file by its original name.
func TestWebformFileTemplates(t *testing.T) {
	testkit.UseEngineRoot(t)
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/c", Status: 1, CommentMode: modulekit.CommentModeHidden}, Elements: WebformElements{
		{ID: 1, Type: WebformFile, Label: "CV", Options: []string{"pdf", "docx"}},
	}}
	out := testkit.RenderPageContent(t, "webform/detail.html", map[string]any{"webform": wf, "item": wf, "values": testWebformValues{}, "fieldErrors": map[int]string{}, "uploadLimit": 2 << 20})
	for _, want := range []string{`enctype="multipart/form-data"`, `type="file" id="element_1" name="element_1"`, `accept=".pdf, .docx"`} {
		if !strings.Contains(out, want) {
			t.Errorf("form: no %q in %s", want, out)
		}
	}
	out = testkit.RenderPageContent(t, "webform/submissions.html", map[string]any{
		"webform": wf, "total": 1, "page": 1,
		"submissions": []WebformSubmission{{ID: 5, Values: map[int][]string{1: {WebformFileValue("webform/9/1/ab.pdf", "my cv.pdf")}}}},
	})
	if !strings.Contains(out, `<a href="/node/9/form/file?sid=5&amp;eid=1&amp;d=0">my cv.pdf</a>`) || strings.Contains(out, "webform/9/1/ab.pdf") {
		t.Errorf("submissions: %s", out)
	}
}

// TestWebformAnalysisTemplate - a question's answered count, its options
// with counts and bars, a value no longer among the options marked.
func TestWebformAnalysisTemplate(t *testing.T) {
	testkit.UseEngineRoot(t)
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/c", Title: "Contact"}, Elements: testWebformElements()}
	out := testkit.RenderPageContent(t, "webform/analysis.html", map[string]any{"webform": wf, "analysis": WebformAnalysis{Total: 4, Elements: []WebformElementAnalysis{
		{Element: wf.Elements[0], Answered: 4, Percent: 100},
		{Element: wf.Elements[2], Answered: 2, Percent: 50, Options: []WebformOptionCount{{Value: "Ads", Count: 1, Percent: 50}, {Value: "Old", Count: 1, Percent: 50, Other: true}}},
	}}})
	for _, want := range []string{"4 submissions", "Answered: 4 (100%)", "<h2 class=\"h5\">Topic</h2>", `style="width: 50%"`, "1 · 50%", "no longer an option", `href="/node/9/form/submissions"`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in %s", want, out)
		}
	}
	if strings.Count(out, "no longer an option") != 1 {
		t.Error("only the old value is marked")
	}
}

// TestWebformPurgeForm - the purge form is shown only to those who may
// delete submissions.
func TestWebformPurgeForm(t *testing.T) {
	testkit.UseEngineRoot(t)
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/c"}, Elements: testWebformElements()}
	data := func(canDelete bool) map[string]any {
		return map[string]any{"webform": wf, "total": 1, "page": 1, "canDeleteSubmissions": canDelete,
			"submissions": []WebformSubmission{{ID: 1, Values: map[int][]string{}}}}
	}
	if out := testkit.RenderPageContent(t, "webform/submissions.html", data(true)); !strings.Contains(out, `action="/node/9/form/submissions/purge"`) || !strings.Contains(out, `href="/node/9/form/analysis"`) {
		t.Errorf("purge form / analysis link: %s", out)
	}
	if out := testkit.RenderPageContent(t, "webform/submissions.html", data(false)); strings.Contains(out, "/purge") {
		t.Error("no purge form without the permission")
	}
}

// TestTooLargeTemplate - the 413 page links back to the form it came from,
// else home.
func TestTooLargeTemplate(t *testing.T) {
	testkit.UseEngineRoot(t)
	if out := testkit.RenderPageContent(t, "errors/too_large.html", map[string]any{"back": "/form/contact"}); !strings.Contains(out, `<a href="/form/contact">Back to the form</a>`) {
		t.Errorf("back: %s", out)
	}
	if out := testkit.RenderPageContent(t, "errors/too_large.html", map[string]any{}); !strings.Contains(out, `<a href="/">`) {
		t.Errorf("home: %s", out)
	}
}

// TestWebformTypedInputs - number, date and phone elements get their
// input types, a number its limits.
func TestWebformTypedInputs(t *testing.T) {
	testkit.UseEngineRoot(t)
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/c", Status: 1, CommentMode: modulekit.CommentModeHidden}, Elements: WebformElements{
		{ID: 1, Type: WebformNumber, Label: "Age", Options: []string{"min=18"}},
		{ID: 2, Type: WebformDate, Label: "Day"},
		{ID: 3, Type: WebformPhone, Label: "Phone"},
	}}
	out := testkit.RenderPageContent(t, "webform/detail.html", map[string]any{"webform": wf, "item": wf, "values": testWebformValues{}, "fieldErrors": map[int]string{}})
	for _, want := range []string{`type="number" id="element_1" name="element_1" value="" class="form-control" step="any" min="18">`, `type="date" id="element_2"`, `type="tel" id="element_3"`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in %s", want, out)
		}
	}
}

// TestWebformSubmissionTemplate - every question with its answer (a dash
// when none, files as links), the sender, the neighbours and deleting.
func TestWebformSubmissionTemplate(t *testing.T) {
	testkit.UseEngineRoot(t)
	els := append(testWebformElements(), WebformElement{ID: 5, Type: WebformFile, Label: "CV"})
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/c", Title: "Contact"}, Elements: els}
	sub := &WebformSubmission{ID: 7, Username: "ann", UserSlug: "users/ann", RemoteIP: "10.0.0.1",
		Values: map[int][]string{1: {"Ann"}, 4: {"News", "Games"}, 5: {WebformFileValue("webform/9/5/ab.pdf", "cv.pdf")}}}
	render := func(newer, older int, canDelete bool) string {
		return testkit.RenderPageContent(t, "webform/submission.html", map[string]any{"webform": wf, "submission": sub, "newer": newer, "older": older, "canDeleteSubmissions": canDelete})
	}
	out := render(8, 0, true)
	for _, want := range []string{"<dt>Name</dt>", "Ann", "News<br>Games", `<a href="/users/ann">ann</a>`, "10.0.0.1",
		`<a href="/node/9/form/file?sid=7&amp;eid=5&amp;d=0">cv.pdf</a>`, `href="/node/9/form/submissions/8" rel="prev"`, `name="only" value="7"`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in %s", want, out)
		}
	}
	if strings.Count(out, `<span class="text-muted">-</span>`) != 2 || strings.Contains(out, `rel="next"`) {
		t.Errorf("unanswered dashes / no older link: %s", out)
	}
	if out := render(0, 6, false); strings.Contains(out, `name="only"`) || !strings.Contains(out, `href="/node/9/form/submissions/6" rel="next"`) {
		t.Errorf("without delete: %s", out)
	}
}

// TestWebformPrefillAndSource - an element filled from the address
// carries its parameter for forms.js, and the form carries the source
// field with its own path; the submission shows the page it came from.
func TestWebformPrefillAndSource(t *testing.T) {
	testkit.UseEngineRoot(t)
	wf := &Webform{Node: modulekit.Node{ID: 9, Slug: "form/order", Status: 1, CommentMode: modulekit.CommentModeHidden}, Elements: WebformElements{
		{ID: 1, MachineName: "product", Type: WebformText, Label: "Product", PrefillParam: "product"},
		{ID: 2, MachineName: "name", Type: WebformText, Label: "Name"},
	}}
	out := testkit.RenderPageContent(t, "webform/detail.html", map[string]any{"webform": wf, "item": wf, "values": testWebformValues{}, "fieldErrors": map[int]string{}})
	if !strings.Contains(out, `data-prefill="product"`) || strings.Count(out, "data-prefill=") != 1 {
		t.Errorf("prefill: %s", out)
	}
	if !strings.Contains(out, `name="webform_source" value="" data-webform-source data-form-path="/form/order"`) {
		t.Errorf("source field: %s", out)
	}
	sub := &WebformSubmission{ID: 3, SourceURL: "/catalog/tank?size=200", Values: map[int][]string{1: {"Tank"}}}
	page := testkit.RenderPageContent(t, "webform/submission.html", map[string]any{"webform": wf, "submission": sub})
	if !strings.Contains(page, `<a href="/catalog/tank?size=200">/catalog/tank?size=200</a>`) {
		t.Errorf("submission: %s", page)
	}
	list := testkit.RenderPageContent(t, "webform/submissions.html", map[string]any{"webform": wf, "total": 1, "page": 1, "submissions": []WebformSubmission{*sub}})
	if !strings.Contains(list, `href="/catalog/tank?size=200"`) {
		t.Errorf("submissions: %s", list)
	}
}
