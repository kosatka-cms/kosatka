package themeinstall

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBuildIndex - a repository's newest release in the catalog (its
// manifest, commit, date and stats); a repository that isn't a theme is
// left out with its error, the others built.
func TestBuildIndex(t *testing.T) {
	source := fixture(t)
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	asked := 0
	stats := func(_ context.Context, s string) (RepoStats, bool, error) {
		asked++
		return RepoStats{Stars: 7, Updated: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}, true, nil
	}
	catalog, failed := Installer{}.BuildIndex(context.Background(), []string{source, "file://" + empty}, stats)
	if len(catalog.Themes) != 1 || len(failed) != 1 || !strings.Contains(failed[0].Error(), "empty") {
		t.Fatalf("themes %+v, failed %v: want the theme, the empty repository left out", catalog.Themes, failed)
	}
	th := catalog.Themes[0]
	if th.Name != "sand" || th.Version != "v1.1.0" || th.Title != "Sand" || len(th.Commit) != 40 || th.Released.IsZero() || th.Stars != 7 || asked != 1 {
		t.Errorf("the theme: %+v", th)
	}
	if th.Screenshot != "" {
		t.Errorf("a screenshot address off GitHub: %q", th.Screenshot)
	}
	if catalog.Generated.IsZero() {
		t.Error("no build time")
	}

	// The same name twice: the second left out.
	if _, failed := (Installer{}).BuildIndex(context.Background(), []string{source, source + "/"}, nil); len(failed) != 1 {
		t.Errorf("a name taken twice: %v", failed)
	}
}

func TestGitHubRepo(t *testing.T) {
	for source, want := range map[string]string{
		"https://github.com/example/kosatka-theme-sand":     "example/kosatka-theme-sand",
		"https://github.com/example/kosatka-theme-sand.git": "example/kosatka-theme-sand",
		"git@github.com:example/night.git":                  "example/night",
		"ssh://git@github.com/example/night":                "example/night",
		"https://gitlab.com/example/sand":                   "",
		"https://github.com/example":                        "",
	} {
		owner, repo, ok := githubRepo(source)
		if got := owner + "/" + repo; (ok && got != want) || (!ok && want != "") {
			t.Errorf("githubRepo(%q) = %q %v, want %q", source, got, ok, want)
		}
	}
	if got := screenshotURL("https://github.com/example/sand", "v1.2.0", "images/shot.jpg"); got != "https://raw.githubusercontent.com/example/sand/v1.2.0/static/images/shot.jpg" {
		t.Errorf("screenshotURL: %q", got)
	}
}

// TestGitHubStats - a repository's stars and last push from the API, the
// token sent when there is one; a host that isn't GitHub - not asked.
func TestGitHubStats(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/repos/example/sand" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"stargazers_count": 42, "pushed_at": "2026-10-20T10:00:00Z"}`))
	}))
	defer srv.Close()
	stats := GitHubStats(srv.Client(), srv.URL, "secret-token")
	st, ok, err := stats(context.Background(), "https://github.com/example/sand")
	if err != nil || !ok || st.Stars != 42 || st.Updated.Day() != 20 || auth != "Bearer secret-token" {
		t.Errorf("stats %+v %v %v, auth %q", st, ok, err, auth)
	}
	if _, ok, err := stats(context.Background(), "https://gitlab.com/example/sand"); ok || err != nil {
		t.Errorf("not GitHub: ok %v, %v", ok, err)
	}
	if _, _, err := stats(context.Background(), "https://github.com/example/missing"); err == nil {
		t.Error("a 404: no error")
	}
}

func TestReadSources(t *testing.T) {
	file := filepath.Join(t.TempDir(), "themes.txt")
	body := "# the themes\nhttps://github.com/example/sand\n\n  https://github.com/example/night  \nhttps://github.com/example/sand\n"
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSources(file)
	if err != nil || strings.Join(got, " ") != "https://github.com/example/sand https://github.com/example/night" {
		t.Errorf("sources %q %v", got, err)
	}
}
