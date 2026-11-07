package service

import (
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/buildinfo"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestStatusReportPageRender - the status report's checks (errors first,
// HTML descriptions), the engine and the modules; the admin layout's bar
// links to it while checks warn.
func TestStatusReportPageRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("admin/reports/status.html", []string{"admin", "reports", "status.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("status-report-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{
		"requirements": []entity.Requirement{
			{Title: "Database updates", Value: "2 pending", Description: `Run <code>app migrate -apply</code>`, Severity: entity.RequirementError},
			{Title: "Private files", Value: "/opt/x/private", Severity: entity.RequirementOK, Module: "forms"},
		},
		"build":     buildinfo.Info{Commit: "0123456789abcdef", GoVersion: "go1.26"},
		"dbDriver":  "mysql",
		"dbVersion": "8.4.0",
		"modules":   []entity.StatusModule{{Name: "forum", Label: "Forum", Installed: true, Enabled: true, SchemaVersion: "20261015210625", SchemaStep: "baseline", SchemaUpdated: time.Date(2026, 11, 6, 21, 26, 0, 0, time.UTC)}, {Name: "forms", Label: "Forms"}},
	}
	var buf strings.Builder
	if err := set.ExecuteTemplate(&buf, "content", data); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`admin-pill--danger`, "Run <code>app migrate -apply</code>", `<span class="admin-table-sub">forms</span>`, `title="0123456789abcdef">01234567</code>`, "mysql 8.4.0", `<span title="20261015210625">06.11.2026 21:26</span>`, `<span class="admin-table-muted">· baseline</span>`, "Not installed"} {
		if !strings.Contains(out, want) {
			t.Errorf("status report lacks %q", want)
		}
	}

	files, err = svc.pageFilesToParse("admin/reports/status.html", []string{"admin", "reports", "status.html"})
	if err != nil {
		t.Fatal(err)
	}
	layout, err := svc.getOrParse("status-bar-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		data map[string]any
		want []string
		bar  bool
	}{
		{map[string]any{"statusErrors": 1, "statusWarnings": 2}, []string{"alert-danger", "1 error, 2 warnings", `href="/admin/reports/status"`}, true},
		{map[string]any{"statusWarnings": 1}, []string{"alert-warning", "1 warning"}, true},
		{map[string]any{}, nil, false},
	} {
		buf.Reset()
		if err := layout.ExecuteTemplate(&buf, "layout.html", c.data); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if got := strings.Contains(out, "admin-status-bar"); got != c.bar {
			t.Errorf("%v: bar %v", c.data, got)
		}
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				t.Errorf("%v: bar lacks %q", c.data, w)
			}
		}
	}
}
