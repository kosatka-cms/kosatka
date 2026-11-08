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
// HTML descriptions), the engine (a site's own binary: the release's
// page, the site's build - only with its commit) and the modules (folded,
// with their count);
// the admin layout's bar links
// to it while checks warn.
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
	for _, want := range []string{`admin-pill--danger`, "Run <code>app migrate -apply</code>", `<span class="admin-table-sub">forms</span>`, `title="0123456789abcdef">01234567</code>`, "mysql 8.4.0", `<span title="20261015210625">06.11.2026 21:26</span>`, `<span class="admin-table-muted">· baseline</span>`, "Not installed", `<details class="admin-settings-group admin-settings-fold">`, `<span class="admin-settings-count">2</span>`} {
		if !strings.Contains(out, want) {
			t.Errorf("status report lacks %q", want)
		}
	}

	// A site's own binary on an engine release: the release's page for
	// its commit, no commit time; the site's module and commit.
	data["build"] = buildinfo.Info{Version: "v0.1.2", GoVersion: "go1.26",
		Site: buildinfo.Site{Module: "example.com/site", Commit: "fedcba9876543210", Time: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)}}
	buf.Reset()
	if err := set.ExecuteTemplate(&buf, "content", data); err != nil {
		t.Fatal(err)
	}
	out = buf.String()
	for _, want := range []string{`href="https://github.com/kosatka-cms/kosatka/releases/tag/v0.1.2"`, "<code>example.com/site</code>", `title="fedcba9876543210">fedcba98</code>`, "07.11.2026"} {
		if !strings.Contains(out, want) {
			t.Errorf("a site's binary: no %q", want)
		}
	}
	if strings.Count(out, "Commit time") != 1 {
		t.Error("a release's commit time shown (unknown)")
	}

	// A site's binary built outside a git checkout: no commit, no Site
	// section.
	data["build"] = buildinfo.Info{Version: "v0.1.2", GoVersion: "go1.26", Site: buildinfo.Site{Module: "example.com/site"}}
	buf.Reset()
	if err := set.ExecuteTemplate(&buf, "content", data); err != nil {
		t.Fatal(err)
	}
	if out = buf.String(); strings.Contains(out, "example.com/site") {
		t.Error("the Site section shown without the build's commit")
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
