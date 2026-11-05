package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// releasesFixture - <home>/releases/{r1,r2}, sites a and b on r1, the
// host running r2: its engine runs answer migrate -apply by migrate (a
// site → "ERR …" fails), write the dump "backup <file>" asks for, and
// record which engine each run used; every site answers /healthz.
func releasesFixture(t *testing.T, migrate map[string]string) (*sitesHost, string, *[]string) {
	t.Helper()
	home := t.TempDir()
	r1, r2 := filepath.Join(home, "releases", "r1"), filepath.Join(home, "releases", "r2")
	for _, d := range []string{r1, r2} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(home, "sites")
	for _, name := range []string{"a", "b"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, siteConfigFile), []byte("port: 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := setLink(dir, siteEngineLink, r1); err != nil {
			t.Fatal(err)
		}
	}
	var runs []string
	h := &sitesHost{engine: r2, release: r2, root: root, out: &bytes.Buffer{}, pause: func(time.Duration) {}}
	h.run = func(_ context.Context, s hostSite, engine string, args ...string) (string, error) {
		if engine == "" {
			engine = releaseName(s.linked(siteEngineLink))
		} else {
			engine = releaseName(engine)
		}
		runs = append(runs, s.Name+" "+engine+" "+strings.Join(args, " "))
		switch args[0] {
		case "config":
			return `{"port": 8101}`, nil
		case "backup":
			return "", os.WriteFile(args[1], []byte("dump"), 0o600)
		case "migrate":
			if rest, failed := strings.CutPrefix(migrate[s.Name], "ERR "); failed {
				return rest, errors.New("exit status 1")
			}
		}
		return "", nil
	}
	h.command = func(context.Context, string, ...string) (string, error) { return "", nil }
	h.status = func(context.Context, string) (int, error) { return 200, nil }
	return h, root, &runs
}

// TestSitesUpdateReleases - each site dumped (by this release), moved to
// it, migrated by it; the release it ran is kept as its previous one.
func TestSitesUpdateReleases(t *testing.T) {
	h, root, runs := releasesFixture(t, nil)
	if code := h.update(context.Background(), nil, updateOptions{restart: defaultRestart, wait: time.Second, backup: true, keep: 5}); code != 0 {
		t.Fatalf("update: %d\n%s", code, h.out)
	}
	for _, name := range []string{"a", "b"} {
		s := hostSite{Name: name, Dir: filepath.Join(root, name)}
		if got := releaseName(s.linked(siteEngineLink)); got != "r2" {
			t.Errorf("%s runs %s, want r2", name, got)
		}
		if got := releaseName(s.linked(sitePreviousLink)); got != "r1" {
			t.Errorf("%s's previous release %s, want r1", name, got)
		}
		if dump := newestBackup(s.Dir); !strings.HasSuffix(dump, "-r1.sql.gz") {
			t.Errorf("%s's dump %q", name, dump)
		}
	}
	joined := strings.Join(*runs, "\n")
	for _, want := range []string{"a r2 backup", "a r2 migrate -apply", "b r2 migrate -apply"} {
		if !strings.Contains(joined, want) {
			t.Errorf("runs lack %q:\n%s", want, joined)
		}
	}

	// Again: the sites run r2 already - migrated, no new dump.
	*runs = nil
	if code := h.update(context.Background(), nil, updateOptions{wait: time.Second, backup: true, keep: 5}); code != 0 {
		t.Fatalf("second update: %d", code)
	}
	if joined := strings.Join(*runs, "\n"); strings.Contains(joined, "backup") || !strings.Contains(joined, "a r2 migrate -apply") {
		t.Errorf("second update ran:\n%s", joined)
	}
}

// TestSitesUpdateReleaseFails - a site whose migrations fail goes back to
// the release it ran; the dump is named; the next site stays as it was.
func TestSitesUpdateReleaseFails(t *testing.T) {
	h, root, _ := releasesFixture(t, map[string]string{"a": "ERR migrate: duplicate column"})
	if code := h.update(context.Background(), nil, updateOptions{restart: defaultRestart, wait: time.Second, backup: true, keep: 5}); code != 1 {
		t.Fatalf("update: %d", code)
	}
	for _, name := range []string{"a", "b"} {
		s := hostSite{Name: name, Dir: filepath.Join(root, name)}
		if got := releaseName(s.linked(siteEngineLink)); got != "r1" {
			t.Errorf("%s runs %s after the failed update, want r1", name, got)
		}
	}
	out := h.out.(*bytes.Buffer).String()
	for _, want := range []string{"back to release r1", "stopped at a: migrations", "its database before the update: ", "not updated: b"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// TestSitesRollback - after an update, rollback brings each site back to
// its previous release and names its dump.
func TestSitesRollback(t *testing.T) {
	h, root, _ := releasesFixture(t, nil)
	if code := h.update(context.Background(), []string{"a"}, updateOptions{wait: time.Second, backup: true, keep: 5}); code != 0 {
		t.Fatalf("update: %d", code)
	}
	if code := h.rollback(context.Background(), nil, defaultRestart, time.Second); code != 0 {
		t.Fatalf("rollback: %d\n%s", code, h.out)
	}
	a := hostSite{Name: "a", Dir: filepath.Join(root, "a")}
	if got, prev := releaseName(a.linked(siteEngineLink)), releaseName(a.linked(sitePreviousLink)); got != "r1" || prev != "r2" {
		t.Errorf("a runs %s (previous %s), want r1 (r2)", got, prev)
	}
	out := h.out.(*bytes.Buffer).String()
	for _, want := range []string{"==> a\n  release: r2 -> r1", "before the update: ", "==> b: no previous release", "sites: 1 rolled back"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// TestPruneBackups - a site keeps its newest dumps.
func TestPruneBackups(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"20261001-000000-r1.sql.gz", "20261002-000000-r1.sql.gz", "20261003-000000-r2.sql.gz", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneBackups(dir, 2); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if strings.Join(left, " ") != "20261002-000000-r1.sql.gz 20261003-000000-r2.sql.gz notes.txt" {
		t.Errorf("left %q", left)
	}
}

// TestReleaseHome - a release's installation directory.
func TestReleaseHome(t *testing.T) {
	if home, ok := releaseHome("/opt/kosatka/releases/v0.2.0"); !ok || home != "/opt/kosatka" {
		t.Errorf("home %q, %v", home, ok)
	}
	if _, ok := releaseHome("/opt/kosatka"); ok {
		t.Error("a single engine directory taken for a release")
	}
}

// TestSitesAddRelease - a site added from a release links it, and its
// nginx site serves the engine's static files through that link.
func TestSitesAddRelease(t *testing.T) {
	h, root, _ := releasesFixture(t, nil)
	if err := h.add(context.Background(), testNewSite("news", 18741), "", time.Second, ""); err != nil {
		t.Fatal(err)
	}
	news := hostSite{Name: "news", Dir: filepath.Join(root, "news")}
	if got := releaseName(news.linked(siteEngineLink)); got != "r2" {
		t.Errorf("news runs %s, want r2", got)
	}
	if out := h.out.(*bytes.Buffer).String(); !strings.Contains(out, "root "+filepath.Join(news.Dir, siteEngineLink, "web")+";") {
		t.Errorf("the nginx site doesn't serve the static files through the engine link:\n%s", out)
	}
}
