package themeinstall

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// fixture - a project (web/templates with the engine's manifest) as the
// cwd, and a theme's git repository with releases v1.0.0 and v1.1.0, a
// pre-release v1.2.0-rc1 and a tag v2.0.0 that isn't a theme (no
// theme.yaml). Returns the repository's file:// address.
func fixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	project := filepath.Join(root, "project")
	write := func(p, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(project, "web", "templates", themes.InfoFile), "name: Base\nregions:\n  - id: sidebar\n    label: Sidebar\n")
	repo := filepath.Join(root, "kosatka-theme-sand")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	manifest := func(version string) string {
		return "name: Sand\nversion: " + version + "\nregions:\n  - id: sidebar\n    label: Sidebar\n"
	}
	write(filepath.Join(repo, "theme.yaml"), manifest("1.0.0"))
	write(filepath.Join(repo, "templates", "header.html"), `{{define "header"}}v1{{end}}`)
	write(filepath.Join(repo, "static", "css", "sand.css"), "body{}")
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-qm", "v1.0.0")
	git("tag", "v1.0.0")
	write(filepath.Join(repo, "theme.yaml"), manifest("1.1.0"))
	git("commit", "-qam", "v1.1.0")
	git("tag", "v1.1.0")
	write(filepath.Join(repo, "theme.yaml"), manifest("1.2.0"))
	git("commit", "-qam", "rc")
	git("tag", "v1.2.0-rc1")
	git("rm", "-q", "theme.yaml")
	git("commit", "-qm", "not a theme")
	git("tag", "v2.0.0-broken")
	git("tag", "not-a-version")
	t.Chdir(project)
	return "file://" + repo
}

func TestTags(t *testing.T) {
	source := fixture(t)
	tags, err := Installer{}.Tags(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tags, " "); got != "v1.1.0 v1.0.0 v2.0.0-broken v1.2.0-rc1" {
		t.Errorf("tags %q: the releases newest first, then the pre-releases; no other tags", got)
	}
	if _, err := (Installer{}).Tags(context.Background(), "--upload-pack=x"); err == nil {
		t.Error("an option as the repository")
	}
}

// TestInstallLifecycle - add (the newest release), verify, update to a
// version, a file changed in place, update back, remove.
func TestInstallLifecycle(t *testing.T) {
	source := fixture(t)
	ctx := context.Background()
	in := Installer{}

	entry, err := in.Add(ctx, source, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Version != "v1.1.0" || len(entry.Commit) != 40 || !strings.HasPrefix(entry.Hash, "h1:") {
		t.Errorf("installed %+v, want v1.1.0 with its commit and hash", entry)
	}
	if _, err := os.Stat("themes/sand/templates/header.html"); err != nil {
		t.Errorf("the theme's files: %v", err)
	}
	if _, err := os.Stat("themes/sand/.git"); err == nil {
		t.Error("the clone's .git kept")
	}
	lock, err := themes.ReadLock()
	if err != nil || lock.Themes["sand"] != entry {
		t.Errorf("the lock: %+v %v", lock, err)
	}
	themes, err := themes.Available()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, th := range themes {
		if th.Dir == "sand" {
			found = th.Source == source && th.Version == "1.1.0"
		}
	}
	if !found {
		t.Errorf("the Appearance page's themes: %+v, want sand from its source at 1.1.0", themes)
	}
	if _, err := in.Add(ctx, source, "", "sand"); err == nil {
		t.Error("a second add over the installed theme")
	}
	if problems, err := Verify(); err != nil || len(problems) != 0 {
		t.Errorf("verify after the install: %v %v", problems, err)
	}

	if entry, err = in.Update(ctx, "sand", "1.0.0"); err != nil || entry.Version != "v1.0.0" {
		t.Fatalf("update to 1.0.0: %+v %v", entry, err)
	}
	// The build's minified files don't count; a changed file does.
	if err := os.WriteFile("themes/sand/static/css/sand.min.css", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if problems, _ := Verify(); len(problems) != 0 {
		t.Errorf("a minified file counted: %v", problems)
	}
	if err := os.WriteFile("themes/sand/static/css/sand.css", []byte("body{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if problems, _ := Verify(); len(problems) != 1 || problems[0].Theme != "sand" {
		t.Errorf("a file changed in place: %v", problems)
	}

	// A tag that isn't a theme: refused, the installed copy kept.
	if _, err := in.Update(ctx, "sand", "v2.0.0-broken"); err == nil || !strings.Contains(err.Error(), "not a theme") {
		t.Errorf("update to a tag without a theme: %v", err)
	}
	if b, _ := os.ReadFile("themes/sand/static/css/sand.css"); string(b) != "body{color:red}" {
		t.Error("a refused update touched the installed copy")
	}
	if _, err := in.Update(ctx, "sand", "v9.9.9"); err == nil {
		t.Error("update to a missing tag")
	}

	if entry, err = in.Update(ctx, "sand", ""); err != nil || entry.Version != "v1.1.0" {
		t.Fatalf("update to the newest: %+v %v", entry, err)
	}
	if problems, _ := Verify(); len(problems) != 0 {
		t.Errorf("verify after the update: %v", problems)
	}

	if err := in.Remove("sand"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("themes/sand"); err == nil {
		t.Error("removed, the directory is still there")
	}
	if _, err := os.Stat("themes/themes.lock"); err == nil {
		t.Error("the last theme removed, the lock is still there")
	}
	if err := in.Remove("sand"); err == nil {
		t.Error("removing a theme not installed")
	}
}

// TestShippedThemesUntouched - a theme shipped with the project (not in
// the lock) is neither updated nor removed, nor added over; one in the
// lock whose directory is gone is a problem.
func TestShippedThemesUntouched(t *testing.T) {
	source := fixture(t)
	if err := os.MkdirAll("themes/own", 0o755); err != nil {
		t.Fatal(err)
	}
	in := Installer{}
	if _, err := in.Update(context.Background(), "own", ""); err == nil {
		t.Error("updating a shipped theme")
	}
	if err := in.Remove("own"); err == nil {
		t.Error("removing a shipped theme")
	}
	if _, err := in.Add(context.Background(), source, "", "own"); err == nil {
		t.Error("adding over a shipped theme")
	}
	if _, err := in.Add(context.Background(), source, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll("themes/sand"); err != nil {
		t.Fatal(err)
	}
	if problems, _ := Verify(); len(problems) != 1 || !strings.Contains(problems[0].Reason, "missing") {
		t.Errorf("an installed theme's directory gone: %v", problems)
	}
}

// TestRemoveKeepsEngineThemes - with an engine directory, removing a
// theme of the lock whose directory the site hasn't touches only the
// site's themes: the engine's theme of the same name stays.
func TestRemoveKeepsEngineThemes(t *testing.T) {
	source := fixture(t)
	if _, err := (Installer{}).Add(context.Background(), source, "", ""); err != nil {
		t.Fatal(err)
	}
	engine := t.TempDir()
	if err := os.MkdirAll(filepath.Join(engine, "web", "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	shipped := filepath.Join(engine, "themes", "sand")
	if err := os.MkdirAll(shipped, 0o755); err != nil {
		t.Fatal(err)
	}
	paths.SetEngineDir(engine)
	t.Cleanup(func() { paths.SetEngineDir("") })
	if err := os.RemoveAll("themes/sand"); err != nil {
		t.Fatal(err)
	}
	if problems, _ := Verify(); len(problems) != 1 || !strings.Contains(problems[0].Reason, "missing") {
		t.Errorf("the site's copy gone (the engine's is no stand-in): %v", problems)
	}
	if err := (Installer{}).Remove("sand"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shipped); err != nil {
		t.Errorf("the engine's theme is gone: %v", err)
	}
}

func TestNameOf(t *testing.T) {
	for source, want := range map[string]string{
		"https://github.com/example/kosatka-theme-sand":     "sand",
		"https://github.com/example/kosatka-theme-sand.git": "sand",
		"git@github.com:example/Night.git":                  "night",
		"file:///srv/themes/ocean/":                         "ocean",
	} {
		if got := NameOf(source); got != want {
			t.Errorf("NameOf(%q) = %q, want %q", source, got, want)
		}
	}
}
