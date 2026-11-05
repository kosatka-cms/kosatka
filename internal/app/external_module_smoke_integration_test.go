//go:build dbintegration

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

var (
	siteBinOnce sync.Once
	siteBin     string
	siteBinErr  error
)

// buildHelloSite builds the example site's own binary
// (examples/hello-site: the engine with the example module
// examples/hello, from outside the engine) once per test run.
func buildHelloSite(t *testing.T) string {
	t.Helper()
	siteBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "kosatka-hello-site")
		if err != nil {
			siteBinErr = err
			return
		}
		siteBin = filepath.Join(dir, "hello-site")
		build := exec.Command("go", "build", "-o", siteBin, ".")
		build.Dir = filepath.Join(repoRoot(t), "examples", "hello-site")
		if out, err := build.CombinedOutput(); err != nil {
			siteBinErr = fmt.Errorf("go build: %w\n%s", err, out)
		}
	})
	if siteBinErr != nil {
		t.Fatal(siteBinErr)
	}
	return siteBin
}

// TestSiteWithExternalModule - a site's own binary with a module from
// outside the engine (examples/hello-site, the module examples/hello):
// "app module list" names it with its package; a site installed with it
// (-modules hello) has its table, its page answers with its greetings
// and its stylesheet (the module ships no minified copy: its CSS is
// served for it), from the files built into the binary; "app assets
// export" writes the modules' static files, the hello's with a .min copy.
func TestSiteWithExternalModule(t *testing.T) {
	bin := buildHelloSite(t)
	out, err := exec.Command(bin, "module", "list").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "hello") || !strings.Contains(string(out), "example.com/kosatka-hello") || !strings.Contains(string(out), "polls") {
		t.Errorf("module list: %v\n%s", err, out)
	}

	export := t.TempDir()
	if out, err := exec.Command(bin, "assets", "export", export).CombinedOutput(); err != nil {
		t.Fatalf("assets export: %v\n%s", err, out)
	}
	for _, f := range []string{"modules/hello/hello.css", "modules/hello/hello.min.css", "modules/polls/polls.css"} {
		if _, err := os.Stat(filepath.Join(export, f)); err != nil {
			t.Errorf("assets export: no %s", f)
		}
	}

	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSiteBin(t, db, bin, func() {
			if _, err := db.DB.Exec(db.Dialect.Rebind("INSERT INTO hello_greetings (text, created_at) VALUES (?, CURRENT_TIMESTAMP)"), "Hi from the test"); err != nil {
				t.Fatalf("the module's table: %v", err)
			}
		}, "-modules", "hello")
		code, page := s.get("/hello")
		if code != 200 || !strings.Contains(page, "Hi from the test") || !strings.Contains(page, "/static/modules/hello/hello.min.css") {
			t.Errorf("GET /hello: %d, greeting %v, stylesheet %v", code, strings.Contains(page, "Hi from the test"), strings.Contains(page, "/static/modules/hello/hello.min.css"))
		}
		if code, css := s.get("/static/modules/hello/hello.min.css"); code != 200 || !strings.Contains(css, "hello-list") {
			t.Errorf("the module's stylesheet: %d %q", code, css)
		}
		s.signIn()
		if code, _ := s.get("/admin/config/feeds"); code != 404 {
			t.Errorf("an engine module not installed: /admin/config/feeds %d, want 404", code)
		}

		// The module's block, its setting and its data, and its line of the
		// status report: the SDK's, from outside the engine.
		if resp := s.post("/admin/config/blocks/plugin", url.Values{"csrf_token": {s.csrf("/admin/config/blocks")}, "plugin_id": {"hello_latest"}, "region": {"sidebar"}}); resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("place the module's block: %d", resp.StatusCode)
		}
		var id int
		if err := db.DB.QueryRow(`SELECT id FROM block_placements WHERE plugin_id = 'hello_latest'`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		edit := fmt.Sprintf("/admin/config/blocks/plugin/%d/edit", id)
		if _, page := s.get(edit); !strings.Contains(page, `name="setting_count"`) {
			t.Error("the block's placement page has no count setting")
		}
		if resp := s.post(edit, url.Values{"csrf_token": {s.csrf(edit)}, "region": {"sidebar"}, "visibility_scope": {"all"}, "setting_count": {"1"}}); strings.Contains(resp.Header.Get("Location"), "error") {
			t.Fatalf("the block's setting refused: %s", resp.Header.Get("Location"))
		}
		var home string
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if _, home = s.get("/"); strings.Contains(home, "hello-latest") {
				break
			}
		}
		if !strings.Contains(home, "hello-latest") || !strings.Contains(home, "Hi from the test") {
			t.Error("the module's block with its data is not on the front page")
		}
		if _, report := s.get("/admin/reports/status"); !strings.Contains(report, "Greetings") || strings.Contains(report, "Modules missing from the binary") {
			t.Error("the module's line is not in the status report, or it is missing from the binary there")
		}
		s.checkLog()

		// The site's binary won't drop the module from the site while the
		// database has it installed: the file stays as it was.
		tree := t.TempDir()
		for _, f := range []string{"go.mod", modulesFile} {
			b, err := os.ReadFile(filepath.Join(repoRoot(t), "examples", "hello-site", f))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(tree, f), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		remove := exec.Command(bin, "module", "remove", "example.com/kosatka-hello")
		remove.Dir, remove.Env = tree, s.env
		if out, err := remove.CombinedOutput(); err == nil || !strings.Contains(string(out), "hello is installed in the site's database") {
			t.Errorf("module remove of an installed module: %v\n%s", err, out)
		}
		if b, _ := os.ReadFile(filepath.Join(tree, modulesFile)); !strings.Contains(string(b), "example.com/kosatka-hello") {
			t.Errorf("a refused remove changed %s:\n%s", modulesFile, b)
		}

		// A binary without it (the engine's) on this database: the site
		// serves without the module's page and block; the startup log,
		// the status report and app migrate name it.
		s.bin = buildApp(t)
		s.serve()
		if code, _ := s.get("/hello"); code != 404 {
			t.Errorf("GET /hello without the module: %d, want 404", code)
		}
		if _, home := s.get("/"); strings.Contains(home, "hello-latest") {
			t.Error("the module's block is on the front page without the module")
		}
		admin := s.visitor()
		admin.signIn()
		if _, report := admin.get("/admin/reports/status"); !strings.Contains(report, "Modules missing from the binary") || !strings.Contains(report, "<td>hello<span") {
			t.Error("the status report doesn't name the module missing from the binary")
		}
		if out := s.app("migrate"); !strings.Contains(out, "hello: installed in the database, not in this binary") {
			t.Errorf("app migrate doesn't name the module missing from the binary:\n%s", out)
		}
		s.checkLog()
		if log := s.log.String(); !strings.Contains(log, "not in this binary") || !strings.Contains(log, "module=hello") {
			t.Errorf("the startup log doesn't name the module missing from the binary:\n%s", log)
		}
	})
}
