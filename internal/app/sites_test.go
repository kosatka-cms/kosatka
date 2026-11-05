package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestParseEnvFile - an env file's lines as systemd reads them.
func TestParseEnvFile(t *testing.T) {
	got, err := parseEnvFile(strings.NewReader("# secrets\n\nDB_PASSWORD=s3cret\nexport SESSION_SECRET_KEY = \"a \\\"b\\\" c\"\nNAME='x y'\n"))
	want := []string{"DB_PASSWORD=s3cret", `SESSION_SECRET_KEY=a "b" c`, "NAME=x y"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("parsed %q, %v; want %q", got, err, want)
	}
	if _, err := parseEnvFile(strings.NewReader("OK=1\nnot a pair\n")); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("a bad line: %v", err)
	}
}

// sitesFixture - a sites root with sites a, b and c (c without a config:
// not a site), and a host whose engine runs answer from replies (site →
// command → output; an output starting with "ERR " fails), whose restarts
// are recorded and whose sites answer /healthz by health (port → status).
func sitesFixture(t *testing.T, replies map[string]map[string]string, health map[int]int) (*sitesHost, *bytes.Buffer, *[]string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if name != "c" {
			if err := os.WriteFile(filepath.Join(dir, siteConfigFile), []byte("port: 1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	var out bytes.Buffer
	var restarted []string
	h := &sitesHost{engine: root, root: root, out: &out, pause: func(time.Duration) {}}
	h.run = func(_ context.Context, s hostSite, _ string, args ...string) (string, error) {
		reply := replies[s.Name][strings.Join(args, " ")]
		if rest, failed := strings.CutPrefix(reply, "ERR "); failed {
			return rest, errors.New("exit status 1")
		}
		return reply, nil
	}
	h.command = func(_ context.Context, name string, args ...string) (string, error) {
		restarted = append(restarted, name+" "+strings.Join(args, " "))
		return "", nil
	}
	h.status = func(_ context.Context, url string) (int, error) {
		for port, code := range health {
			if strings.Contains(url, fmt.Sprintf(":%d/", port)) {
				return code, nil
			}
		}
		return 0, errors.New("connection refused")
	}
	return h, &out, &restarted
}

// siteReplies - a site on port with its migrate plan and apply outputs.
func siteReplies(port int, plan, apply string) map[string]string {
	return map[string]string{
		"config -json":   fmt.Sprintf(`{"port": %d, "public_base_url": "https://site%d.example.com"}`, port, port),
		"migrate":        plan,
		"migrate -apply": apply,
	}
}

// TestSitesUpdate - each site migrated, restarted and checked in turn.
func TestSitesUpdate(t *testing.T) {
	h, out, restarted := sitesFixture(t, map[string]map[string]string{
		"a": siteReplies(8101, "", "core: applied 2026_x\n"),
		"b": siteReplies(8102, "", "migrate: nothing to do\n"),
	}, map[int]int{8101: 200, 8102: 200})
	if code := h.update(context.Background(), nil, updateOptions{restart: "systemctl restart kosatka-site@{site}", wait: time.Second}); code != 0 {
		t.Fatalf("update: %d\n%s", code, out)
	}
	if want := []string{"systemctl restart kosatka-site@a", "systemctl restart kosatka-site@b"}; !reflect.DeepEqual(*restarted, want) {
		t.Errorf("restarted %q, want %q", *restarted, want)
	}
	for _, want := range []string{"==> a", "core: applied 2026_x", "healthy: http://127.0.0.1:8101/healthz", "==> b", "sites: 2 updated"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// TestSitesUpdateStops - a failed migration stops the update: the site
// isn't restarted, the ones after it aren't touched and are named.
func TestSitesUpdateStops(t *testing.T) {
	h, out, restarted := sitesFixture(t, map[string]map[string]string{
		"a": siteReplies(8101, "", "ERR migrate: duplicate column\n"),
		"b": siteReplies(8102, "", ""),
	}, map[int]int{8101: 200, 8102: 200})
	if code := h.update(context.Background(), nil, updateOptions{restart: defaultRestart, wait: time.Second}); code != 1 {
		t.Fatalf("update: %d, want 1", code)
	}
	if len(*restarted) != 0 {
		t.Errorf("restarted %q after a failed migration", *restarted)
	}
	for _, want := range []string{"duplicate column", "stopped at a: migrations", "not updated: b"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// TestSitesUpdateUnhealthy - a site not answering /healthz within the
// wait stops the update.
func TestSitesUpdateUnhealthy(t *testing.T) {
	h, out, _ := sitesFixture(t, map[string]map[string]string{
		"a": siteReplies(8101, "", ""),
		"b": siteReplies(8102, "", ""),
	}, map[int]int{8101: 503})
	if code := h.update(context.Background(), []string{"b", "a"}, updateOptions{}); code != 1 {
		t.Fatalf("update: %d, want 1", code)
	}
	for _, want := range []string{"stopped at b: not healthy", "connection refused", "not updated: a"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if _, err := h.sites([]string{"nope"}); err == nil {
		t.Error("an unknown site in -only isn't an error")
	}
}

// TestSitesList - a site per line: URL, pending migrations, health; a
// directory without site.yaml isn't a site.
func TestSitesList(t *testing.T) {
	h, out, _ := sitesFixture(t, map[string]map[string]string{
		"a": siteReplies(8101, "core: pending schema 2026_x\ncore: pending data 2026_y\n", ""),
		"b": siteReplies(8102, "migrate: nothing to do\n", ""),
	}, map[int]int{8101: 200})
	if code := h.list(context.Background()); code != 0 {
		t.Fatalf("list: %d", code)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines %q", lines)
	}
	for i, want := range [][]string{{"a", "https://site8101.example.com", "2 pending", "ok"}, {"b", "up to date", "down: connection refused"}} {
		for _, w := range want {
			if !strings.Contains(lines[i+1], w) {
				t.Errorf("line %q lacks %q", lines[i+1], w)
			}
		}
	}
}

// TestSiteEnviron - a site's environment: the basics, the engine and its
// config, then its env file; nothing else of this process.
func TestSiteEnviron(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, siteEnvFile), []byte("DB_NAME=blog\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_PASSWORD", "another site's")
	env, err := siteEnviron("/opt/kosatka", hostSite{Name: "blog", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(env, "\n")
	for _, want := range []string{"ENGINE_DIR=/opt/kosatka", "CONFIG_PATH=" + filepath.Join(dir, siteConfigFile), "DB_NAME=blog"} {
		if !strings.Contains(joined, want) {
			t.Errorf("env lacks %q: %q", want, env)
		}
	}
	if strings.Contains(joined, "DB_PASSWORD") {
		t.Error("this process's DB_PASSWORD leaked into the site's environment")
	}
}

// TestSiteCommand - a blank template is no step, not a panic.
func TestSiteCommand(t *testing.T) {
	if cmd := siteCommand("   ", "blog"); len(cmd) != 0 {
		t.Errorf("blank: %q", cmd)
	}
	if cmd := siteCommand("systemctl restart kosatka-site@{site}", "blog"); len(cmd) != 3 || cmd[2] != "kosatka-site@blog" {
		t.Errorf("%q", cmd)
	}
}
