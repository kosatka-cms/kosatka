package service

import (
	"html/template"
	"strings"
	"testing"
)

// TestAdminAlerts - admin_alerts shows the page's own message for a set
// flag and for the error code: danger for Errors, warning for Warnings,
// Fallback for another code, nothing without one.
func TestAdminAlerts(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	page := `{{template "admin_alerts" (dict "Ctx" .
		"Done" (dict "saved" "Saved")
		"Info" (dict "created" "Created")
		"Errors" (dict "dup" "Duplicate")
		"Warnings" (dict "empty" "Empty")
		"Fallback" "Failed")}}`
	render := func(data map[string]any) string {
		t.Helper()
		tmpl, err := template.New("alerts_test").Funcs(svc.buildFuncMap(nil)).ParseFiles("web/templates/admin/alerts.html")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tmpl.New("page").Parse(page); err != nil {
			t.Fatal(err)
		}
		var buf strings.Builder
		if err := tmpl.ExecuteTemplate(&buf, "page", data); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	for _, tc := range []struct {
		data map[string]any
		want []string
		not  []string
	}{
		{map[string]any{"saved": true}, []string{`alert-success" role="alert">Saved`}, []string{"alert-danger", "Created"}},
		{map[string]any{"created": "1"}, []string{`alert-info" role="alert">Created`}, nil},
		{map[string]any{"error": "dup"}, []string{`alert-danger" role="alert">Duplicate`}, []string{"Failed"}},
		{map[string]any{"error": "empty"}, []string{`alert-warning" role="alert">Empty`}, []string{"Failed"}},
		{map[string]any{"error": "other"}, []string{`alert-danger" role="alert">Failed`}, nil},
		{map[string]any{"error": ""}, nil, []string{"alert"}},
		{map[string]any{}, nil, []string{"alert"}},
	} {
		out := render(tc.data)
		for _, w := range tc.want {
			if !strings.Contains(out, w) {
				t.Errorf("%v: no %q in %s", tc.data, w, out)
			}
		}
		for _, n := range tc.not {
			if strings.Contains(out, n) {
				t.Errorf("%v: unexpected %q in %s", tc.data, n, out)
			}
		}
	}
}

// TestAdminConfirmDelete - a delete confirmation page (menus/link_delete)
// through admin_confirm_delete: its back link, question, own text, form
// action and button; Cancel goes back.
func TestAdminConfirmDelete(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	var buf strings.Builder
	data := map[string]any{"menu": map[string]any{"ID": 3, "Label": "Main"}, "link": map[string]any{"ID": 7, "Title": "Home"}}
	if err := svc.RenderNamed(&buf, "content", data, "admin/menus/link_delete.html", "admin/confirm_delete.html"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`<p><a href="/admin/config/menus/3/links">&larr; Links of menu "Main"</a></p>`,
		`<h5 class="alert-heading">Delete link "Home"?</h5>`,
		"they move up to its own level.",
		`<form method="post" action="/admin/config/menus/3/links/7/delete">`,
		`>Yes, delete link</button>`,
		`<a href="/admin/config/menus/3/links" class="btn btn-secondary">Cancel</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}
