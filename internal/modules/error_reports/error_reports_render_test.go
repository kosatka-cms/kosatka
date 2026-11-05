package errorreports

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/error_reports/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestHintBlock - the "error_report_hint" plugin block renders the
// module's own block_error_report_hint through the core's plugin_block.
func TestHintBlock(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	id := manifest.PluginBlockHint
	out := testkit.RenderPlacement(t, modulekit.BlockPlacement{ID: 1, PlacementType: modulekit.BlockPlacementPlugin, PluginID: &id, Region: "sidebar", VisibilityScope: modulekit.BlockVisibilityAll}, nil)
	if !strings.Contains(out, `class="error-report-hint`) {
		t.Errorf("hint block: %s", out)
	}
}

// TestReportFormAtPageEnd - the page layout ends with the module's report
// form (moduleHook "page_end") while the module is on, with the CAPTCHA
// key its page data gives; nothing while it is off.
func TestReportFormAtPageEnd(t *testing.T) {
	testkit.UseEngineRoot(t)
	render := func(t testing.TB) string {
		t.Helper()
		data := testkit.PageData()
		data["CSRFToken"], data["error_report_recaptcha_key"] = "tok", "site-key"
		return testkit.RenderPageBlock(t, "errors/not_found.html", "layout.html", data)
	}
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	if out := render(t); !strings.Contains(out, `id="error-report-modal"`) || !strings.Contains(out, `data-recaptcha="site-key"`) {
		t.Errorf("module on: no report form")
	}
	// A theme's own layout (the engine tests' fixture theme) with the same hook.
	t.Run("theme", func(t *testing.T) {
		testkit.UseTheme(t, filepath.Join(testkit.EngineRoot(), "testdata", "themes"), "fixture")
		if out := render(t); !strings.Contains(out, `id="error-report-modal"`) || !strings.Contains(out, `id="fixture-main"`) {
			t.Errorf("theme layout: no report form")
		}
	})
	testkit.WithModules(t, map[string]bool{manifest.Name: false})
	if out := render(t); strings.Contains(out, "error-report-modal") {
		t.Error("module off: the report form must not render")
	}
}

// TestAdminListTemplate - the admin list shows each report with its
// sender (a link, or none for a guest), page and selected text; the
// sender's text is plain text, escaped (anyone can send a report).
func TestAdminListTemplate(t *testing.T) {
	testkit.UseEngineRoot(t)
	uid := 3
	out := testkit.RenderPageContent(t, "admin/error_reports.html", map[string]any{"reports": ErrorReports{
		{ID: 7, UserID: &uid, Username: "ann", UserSlug: "users/ann", PageURL: "/news/a", SelectedText: "teh typo", Comment: "fix"},
		{ID: 6, PageURL: "/b<svg onload=alert(1)>", SelectedText: "<img src=x onerror=alert(1)>", Comment: "<script>alert(1)</script>"},
	}, "pagination": pagination.New(1, 2, adminPerPage, "/admin/error-reports", nil)})
	for _, want := range []string{`<a href="/users/ann">ann</a>`, `href="/news/a"`, "teh typo", "fix", "&lt;img src=x onerror=alert(1)&gt;", "&lt;script&gt;"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in %s", want, out)
		}
	}
	for _, bad := range []string{"<img src=x", "<script>", "<svg"} {
		if strings.Contains(out, bad) {
			t.Errorf("unescaped %q in %s", bad, out)
		}
	}
}

// TestAdminListStatus - the "New" tab is current by default, "All" with
// showAll; a new report has the "Fixed" button back to this page, a fixed
// one its date, dimmed, and "Back to the new ones" instead.
func TestAdminListStatus(t *testing.T) {
	testkit.UseEngineRoot(t)
	fixed := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	render := func(showAll bool, reports ErrorReports) string {
		return testkit.RenderPageContent(t, "admin/error_reports.html", map[string]any{"reports": reports, "showAll": showAll,
			"backURL": "/admin/error-reports?page=2", "pagination": pagination.New(1, len(reports), adminPerPage, adminListURL, nil)})
	}
	out := render(false, ErrorReports{{ID: 7, PageURL: "/a", SelectedText: "typo"}})
	for _, want := range []string{`class="nav-link active" href="/admin/error-reports" aria-current="page"`, `action="/admin/error-reports/7/resolve"`, `name="back" value="/admin/error-reports?page=2"`} {
		if !strings.Contains(out, want) {
			t.Errorf("new tab: no %q in %s", want, out)
		}
	}
	out = render(true, ErrorReports{{ID: 6, PageURL: "/b", SelectedText: "typo", ResolvedAt: &fixed}})
	if !strings.Contains(out, `class="nav-link active" href="/admin/error-reports?status=all"`) || !strings.Contains(out, `class="admin-row-muted"`) ||
		strings.Contains(out, "/resolve") || !strings.Contains(out, `action="/admin/error-reports/6/reopen"`) {
		t.Errorf("all tab, a fixed report: %s", out)
	}
	if out := render(false, nil); !strings.Contains(out, "No new reports") {
		t.Errorf("no new reports: %s", out)
	}
}

// TestTemplatesEmbedded - on a site the module's templates come from the
// binary (Assets, testkit.UseModuleAssets): its admin list renders from
// those, its source directory not read.
func TestTemplatesEmbedded(t *testing.T) {
	testkit.UseEngineRoot(t)
	testkit.UseModuleAssets(t, map[string]fs.FS{manifest.Name: Assets})
	if p, err := testkit.ResolveTemplate("admin/error_reports.html"); err != nil || !strings.HasPrefix(p, "embed:") {
		t.Fatalf("the module's template: %q %v, want the embedded one", p, err)
	}
	out := testkit.RenderPageContent(t, "admin/error_reports.html", map[string]any{"reports": ErrorReports{{ID: 7, PageURL: "/news/a", SelectedText: "teh typo"}},
		"pagination": pagination.New(1, 1, adminPerPage, "/admin/error-reports", nil)})
	if !strings.Contains(out, "teh typo") {
		t.Errorf("rendered from the embedded template: %s", out)
	}
}
