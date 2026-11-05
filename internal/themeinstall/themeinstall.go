// Package themeinstall installs, updates and removes the themes kept in
// their own git repositories (app theme …, internal/app/theme.go): a
// theme's repository is its directory
// (theme.yaml at its root), its releases are its tags (v1.2.0). A theme
// is installed into themes/<name>/ at a tag, and themes/themes.lock
// (themes.Lock) records its source, tag, commit and files'
// checksum; the files are the site's to commit (as vendoring), the
// deploy carries them, Verify checks they are what the lock says.
package themeinstall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"golang.org/x/mod/sumdb/dirhash"

	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// nameRe - a theme's directory name.
var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// sourceRe - a repository git can clone: https://…, ssh (git@host:path,
// ssh://…) or file://… (a local one); never an option ("-…").
var sourceRe = regexp.MustCompile(`^(https://|ssh://|file://|[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:)[^\s]+$`)

// Installer - the themes of the project in paths.ThemesDir. Git - the git
// binary ("" - "git" from PATH); Log - where progress goes (nil - none).
type Installer struct {
	Git string
	Log io.Writer
}

func (in Installer) logf(format string, args ...any) {
	if in.Log != nil {
		_, _ = fmt.Fprintf(in.Log, format+"\n", args...)
	}
}

// git runs git with args (in dir, "" - the current one), its output.
func (in Installer) git(ctx context.Context, dir string, args ...string) (string, error) {
	bin := in.Git
	if bin == "" {
		bin = "git"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	// No prompts for credentials: a private repository needs them set up.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(errOut.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// Tags - source's release tags (semver, "v1.2.0"), the newest first;
// pre-releases (v1.2.0-rc1) after the releases.
func (in Installer) Tags(ctx context.Context, source string) ([]string, error) {
	if !sourceRe.MatchString(source) {
		return nil, fmt.Errorf("%q: not a repository address (https://…, git@host:path, ssh://…, file://…)", source)
	}
	out, err := in.git(ctx, "", "ls-remote", "--tags", "--refs", "--", source)
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, line := range strings.Split(out, "\n") {
		_, ref, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		tag := strings.TrimPrefix(ref, "refs/tags/")
		if semver.IsValid(tag) && semver.Canonical(tag) == tag {
			tags = append(tags, tag)
		}
	}
	sort.Slice(tags, func(i, j int) bool {
		pi, pj := semver.Prerelease(tags[i]) != "", semver.Prerelease(tags[j]) != ""
		if pi != pj {
			return pj
		}
		return semver.Compare(tags[i], tags[j]) > 0
	})
	return tags, nil
}

// resolve - version as asked ("" - the newest release), checked against
// source's tags.
func (in Installer) resolve(ctx context.Context, source, version string) (string, error) {
	tags, err := in.Tags(ctx, source)
	if err != nil {
		return "", err
	}
	if version == "" {
		for _, t := range tags {
			if semver.Prerelease(t) == "" {
				return t, nil
			}
		}
		return "", fmt.Errorf("%s has no release tags (v1.2.0)", source)
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	for _, t := range tags {
		if t == version {
			return t, nil
		}
	}
	return "", fmt.Errorf("%s has no tag %s", source, version)
}

// fetch clones source at tag into dir without its .git: the tag's commit
// and its date. A tag without theme.yaml at its root isn't a theme.
func (in Installer) fetch(ctx context.Context, source, tag, dir string) (string, time.Time, error) {
	in.logf("fetching %s %s", source, tag)
	if _, err := in.git(ctx, "", "-c", "advice.detachedHead=false", "clone", "--quiet", "--depth", "1", "--branch", tag, "--single-branch", "--", source, dir); err != nil {
		return "", time.Time{}, err
	}
	commit, err := in.git(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", time.Time{}, err
	}
	var date time.Time
	if out, err := in.git(ctx, dir, "log", "-1", "--format=%cI"); err == nil {
		date, _ = time.Parse(time.RFC3339, out)
	}
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		return "", time.Time{}, err
	}
	if _, err := os.Stat(filepath.Join(dir, themes.InfoFile)); err != nil {
		return "", time.Time{}, fmt.Errorf("%s %s: no %s at the repository's root - not a theme", source, tag, themes.InfoFile)
	}
	return commit, date, nil
}

// Add installs the theme in source at version ("" - its newest release)
// as themes/<name> (name "" - the repository's name, without a
// "kosatka-theme-" prefix). A theme directory already there is refused.
func (in Installer) Add(ctx context.Context, source, version, name string) (themes.LockEntry, error) {
	if name == "" {
		name = NameOf(source)
	}
	if !nameRe.MatchString(name) {
		return themes.LockEntry{}, fmt.Errorf("theme name %q: lowercase latin letters, digits, - and _ (set one with -name)", name)
	}
	if paths.ThemeDir(name) != "" {
		return themes.LockEntry{}, fmt.Errorf("themes/%s is already there (app theme update %s, or another -name)", name, name)
	}
	return in.install(ctx, name, source, version)
}

// Update moves installed theme name to version ("" - its source's newest
// release).
func (in Installer) Update(ctx context.Context, name, version string) (themes.LockEntry, error) {
	lock, err := themes.ReadLock()
	if err != nil {
		return themes.LockEntry{}, err
	}
	entry, ok := lock.Themes[name]
	if !ok {
		return themes.LockEntry{}, fmt.Errorf("%s: not installed from a repository (themes/themes.lock)", name)
	}
	return in.install(ctx, name, entry.Source, version)
}

// install fetches source at version into themes/<name> (replacing what is
// there only once the new copy is checked) and records it in the lock.
func (in Installer) install(ctx context.Context, name, source, version string) (themes.LockEntry, error) {
	tag, err := in.resolve(ctx, source, version)
	if err != nil {
		return themes.LockEntry{}, err
	}
	root, err := paths.ThemesDir()
	if err != nil {
		return themes.LockEntry{}, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return themes.LockEntry{}, err
	}
	// The copy is fetched beside the themes (the same file system, for a
	// rename), in a dot-directory no theme listing sees.
	stage, err := os.MkdirTemp(root, ".install-"+name+"-")
	if err != nil {
		return themes.LockEntry{}, err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	fetched := filepath.Join(stage, name)
	commit, _, err := in.fetch(ctx, source, tag, fetched)
	if err != nil {
		return themes.LockEntry{}, err
	}
	hash, err := Hash(fetched)
	if err != nil {
		return themes.LockEntry{}, err
	}

	// Swap it in: the current copy aside, the new one in its place, then
	// the manifest checked (a broken one puts the old copy back).
	dir := filepath.Join(root, name)
	old := filepath.Join(stage, "old")
	hadOld := false
	if _, err := os.Stat(dir); err == nil {
		if err := os.Rename(dir, old); err != nil {
			return themes.LockEntry{}, err
		}
		hadOld = true
	}
	if err := os.Rename(fetched, dir); err != nil {
		if hadOld {
			_ = os.Rename(old, dir)
		}
		return themes.LockEntry{}, err
	}
	info, err := themes.LoadInfoFor(name)
	if err != nil {
		_ = os.RemoveAll(dir)
		if hadOld {
			_ = os.Rename(old, dir)
		}
		return themes.LockEntry{}, fmt.Errorf("%s %s: %w", source, tag, err)
	}
	if !themes.Compatible(info.MinVersion) {
		in.logf("warning: %s needs Kosatka %s or newer (this is %s)", name, info.MinVersion, themes.EngineRelease())
	}
	if info.Version != "" && "v"+info.Version != tag {
		in.logf("warning: %s's theme.yaml says version %s, its tag is %s", name, info.Version, tag)
	}

	entry := themes.LockEntry{Source: source, Version: tag, Commit: commit, Hash: hash}
	if err := updateLock(func(l *themes.Lock) { l.Themes[name] = entry }); err != nil {
		return entry, err
	}
	in.logf("installed themes/%s %s (%s)", name, tag, shortCommit(commit))
	return entry, nil
}

// Remove deletes installed theme name and its lock entry. Its blocks'
// placements and settings stay in the database (harmless, as a theme's
// that is just not active); a site with it active falls back to the
// engine's templates.
func (in Installer) Remove(name string) error {
	lock, err := themes.ReadLock()
	if err != nil {
		return err
	}
	if _, ok := lock.Themes[name]; !ok {
		return fmt.Errorf("%s: not installed from a repository (themes/themes.lock)", name)
	}
	dir, err := installedDir(name)
	if err != nil {
		return err
	}
	if dir != "" {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := updateLock(func(l *themes.Lock) { delete(l.Themes, name) }); err != nil {
		return err
	}
	in.logf("removed themes/%s", name)
	return nil
}

// installedDir - theme name's directory among the site's own themes
// (paths.ThemesDir, where install puts it), "" when it isn't there: never
// a theme the engine ships of the same name (paths.ThemeDir finds those
// too).
func installedDir(name string) (string, error) {
	root, err := paths.ThemesDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, name)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "", nil
	}
	return dir, nil
}

// Problem - an installed theme that isn't what the lock says.
type Problem struct {
	Theme  string
	Reason string
}

// Verify checks each theme of the lock: its directory is there and its
// files' checksum is the lock's (no file changed, added or removed since
// it was installed; a theme is changed by a new release, not in place).
func Verify() ([]Problem, error) {
	lock, err := themes.ReadLock()
	if err != nil {
		return nil, err
	}
	var out []Problem
	names := make([]string, 0, len(lock.Themes))
	for name := range lock.Themes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := lock.Themes[name]
		dir, err := installedDir(name)
		if err != nil {
			return nil, err
		}
		if dir == "" {
			out = append(out, Problem{name, "its directory themes/" + name + " is missing"})
			continue
		}
		hash, err := Hash(dir)
		if err != nil {
			return nil, err
		}
		if hash != entry.Hash {
			out = append(out, Problem{name, "its files differ from " + entry.Version + " as installed (changed in place?)"})
		}
	}
	return out, nil
}

// Hash - the checksum of the theme files in dir (dirhash's "h1:…", as
// go.sum's): every file but the minified ones the build makes
// (*.min.css, *.min.js) and the system's (.DS_Store), by path.
func Hash(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".min.css") || strings.HasSuffix(name, ".min.js") || name == ".DS_Store" {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	return dirhash.Hash1(files, func(name string) (io.ReadCloser, error) {
		return os.Open(filepath.Join(dir, filepath.FromSlash(name)))
	})
}

// NameOf - a theme's name from its repository's address: the last part,
// without ".git" and a "kosatka-theme-" prefix ("…/kosatka-theme-sand.git"
// → "sand").
func NameOf(source string) string {
	s := strings.TrimSuffix(strings.TrimRight(source, "/"), ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimPrefix(strings.ToLower(s), "kosatka-theme-")
}

// updateLock rewrites themes/themes.lock with change applied (sorted, as
// json.Marshal does maps; one line per field, for a readable diff).
func updateLock(change func(*themes.Lock)) error {
	lock, err := themes.ReadLock()
	if err != nil {
		return err
	}
	change(&lock)
	p, err := paths.ThemesLockFile()
	if err != nil {
		return err
	}
	if len(lock.Themes) == 0 {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	b, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}

func shortCommit(c string) string {
	if len(c) > 8 {
		return c[:8]
	}
	return c
}
