package themeinstall

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/themes"
)

// The themes' catalog's builder (app theme index): from a list of the
// themes' repositories (themes.txt - one address a line, # comments), an
// index (themes.Catalog, index.json) of each one's newest release:
// its manifest's description, the release's commit and date, its
// screenshot's address and, on GitHub, its stars and last update. A
// repository that fails (no release, no theme at it, a broken manifest)
// is left out with its error - the others are built.

// RepoStats - a repository's stars and last update from its host.
type RepoStats struct {
	Stars   int
	Updated time.Time
}

// StatsFunc - source's stats; ok=false - its host isn't one the func
// knows.
type StatsFunc func(ctx context.Context, source string) (stats RepoStats, ok bool, err error)

// githubRepoRe - a GitHub repository's owner and name from its https or
// ssh address.
var githubRepoRe = regexp.MustCompile(`^(?:https://github\.com/|git@github\.com:|ssh://git@github\.com/)([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(?:\.git)?/?$`)

// githubRepo - source's owner and name on GitHub (ok=false - not there).
func githubRepo(source string) (owner, repo string, ok bool) {
	m := githubRepoRe.FindStringSubmatch(source)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// GitHubStats - stats from GitHub's API (api - its address, "" - the
// public one; token - "" for the anonymous limit, 60 requests an hour).
func GitHubStats(client *http.Client, api, token string) StatsFunc {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if api == "" {
		api = "https://api.github.com"
	}
	return func(ctx context.Context, source string) (RepoStats, bool, error) {
		owner, repo, ok := githubRepo(source)
		if !ok {
			return RepoStats{}, false, nil
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo), nil)
		if err != nil {
			return RepoStats{}, true, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			return RepoStats{}, true, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return RepoStats{}, true, fmt.Errorf("GitHub: %s", resp.Status)
		}
		var body struct {
			Stars    int       `json:"stargazers_count"`
			PushedAt time.Time `json:"pushed_at"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return RepoStats{}, true, err
		}
		return RepoStats{Stars: body.Stars, Updated: body.PushedAt}, true, nil
	}
}

// screenshotURL - the address of a release's screenshot: on GitHub its
// raw file at the tag; "" elsewhere.
func screenshotURL(source, tag, screenshot string) string {
	owner, repo, ok := githubRepo(source)
	if !ok || screenshot == "" {
		return ""
	}
	return "https://raw.githubusercontent.com/" + owner + "/" + repo + "/" + tag + "/" + path.Join("static", screenshot)
}

// ReadSources - the repositories listed in file (one a line; blank lines
// and # comments skipped), each once.
func ReadSources(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	return out, sc.Err()
}

// BuildIndex - the catalog of sources' newest releases (stats - nil for
// none); the repositories that failed, each with its error.
func (in Installer) BuildIndex(ctx context.Context, sources []string, stats StatsFunc) (themes.Catalog, []error) {
	catalog := themes.Catalog{Generated: time.Now().UTC().Truncate(time.Second)}
	var failed []error
	names := map[string]string{}
	for _, source := range sources {
		theme, err := in.indexOne(ctx, source, stats)
		if err != nil {
			failed = append(failed, fmt.Errorf("%s: %w", source, err))
			continue
		}
		if other, ok := names[theme.Name]; ok {
			failed = append(failed, fmt.Errorf("%s: its name %q is %s's too", source, theme.Name, other))
			continue
		}
		names[theme.Name] = source
		catalog.Themes = append(catalog.Themes, theme)
	}
	return catalog, failed
}

func (in Installer) indexOne(ctx context.Context, source string, stats StatsFunc) (themes.CatalogTheme, error) {
	tag, err := in.resolve(ctx, source, "")
	if err != nil {
		return themes.CatalogTheme{}, err
	}
	stage, err := os.MkdirTemp("", "kosatka-theme-index-")
	if err != nil {
		return themes.CatalogTheme{}, err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	dir := filepath.Join(stage, "theme")
	commit, released, err := in.fetch(ctx, source, tag, dir)
	if err != nil {
		return themes.CatalogTheme{}, err
	}
	info, err := themes.ReadManifestFile(filepath.Join(dir, themes.InfoFile))
	if err != nil {
		return themes.CatalogTheme{}, fmt.Errorf("%s: %w", tag, err)
	}
	theme := themes.CatalogTheme{
		Name: NameOf(source), Source: source, Version: tag, Commit: commit, Released: released,
		Title: info.Name, Description: info.Description, Author: info.Author, Homepage: info.Homepage,
		License: info.License, Tags: info.Tags, MinVersion: info.MinVersion, Base: info.Base,
		Screenshot: screenshotURL(source, tag, info.Screenshot),
	}
	if !nameRe.MatchString(theme.Name) {
		return themes.CatalogTheme{}, fmt.Errorf("its name %q isn't a theme directory's", theme.Name)
	}
	if stats != nil {
		if st, ok, err := stats(ctx, source); err != nil {
			in.logf("warning: %s: no stats: %v", source, err)
		} else if ok {
			theme.Stars, theme.Updated = st.Stars, st.Updated
		}
	}
	return theme, nil
}
