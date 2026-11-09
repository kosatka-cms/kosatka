package themeinstall

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/kosatka-cms/kosatka/internal/app/demoset"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/i18nextract"
	"github.com/kosatka-cms/kosatka/internal/i18npack"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Finding - one thing Check found in a theme: a Problem (a site or the
// themes catalog would refuse it or show it wrong) or a warning. File and
// Line - where, File relative to the theme's directory ("" - the theme as
// a whole, Line 0 - the file as a whole).
type Finding struct {
	Problem bool
	File    string
	Line    int
	Text    string
}

func (f Finding) String() string {
	kind := "warning"
	if f.Problem {
		kind = "problem"
	}
	switch {
	case f.File == "":
		return kind + ": " + f.Text
	case f.Line == 0:
		return fmt.Sprintf("%s: %s: %s", kind, f.File, f.Text)
	}
	return fmt.Sprintf("%s: %s:%d: %s", kind, f.File, f.Line, f.Text)
}

// Check checks the theme in dir before a release (app theme check):
//   - the manifest (as installing checks it) and what the catalog asks of
//     it, the files it names (screenshot, stylesheets);
//   - the theme's files its templates, scripts and stylesheets refer to;
//   - resources loaded from other servers (CDNs, web fonts), which the
//     catalog doesn't accept;
//   - each language's catalog (translations/<lang>.json): the theme's
//     strings it lacks, translations that can't stand for their source,
//     strings the theme doesn't use - engine is the engine's and the
//     modules' strings (translations.Sources), translated by their own
//     catalogs, not the theme's;
//   - its own demo sets (demo/<lang>.yaml): each loads, in the theme's
//     regions;
//   - in a theme's own git repository, its version against the latest
//     release tag.
func (in Installer) Check(ctx context.Context, dir string, engine []i18n.Source) ([]Finding, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(abs, themes.InfoFile)); err != nil {
		return nil, fmt.Errorf("%s: not a theme (no %s)", dir, themes.InfoFile)
	}
	c := &checker{dir: abs, name: NameOf(filepath.Base(abs))}
	if !nameRe.MatchString(c.name) {
		c.problem("", 0, "the directory's name gives the theme the name %q: lowercase latin letters, digits, - and _", c.name)
	}
	info, manifestOK := c.manifest()
	if err := c.files(info); err != nil {
		return nil, err
	}
	if err := c.translations(engine); err != nil {
		return nil, err
	}
	if err := c.demo(info, manifestOK); err != nil {
		return nil, err
	}
	if manifestOK {
		c.release(ctx, in, info.Version)
	}
	return c.findings, nil
}

// checker - one Check: the theme's directory and name, what was found.
type checker struct {
	dir, name string
	findings  []Finding
}

func (c *checker) problem(file string, line int, format string, args ...any) {
	c.findings = append(c.findings, Finding{Problem: true, File: file, Line: line, Text: fmt.Sprintf(format, args...)})
}

func (c *checker) warn(file string, line int, format string, args ...any) {
	c.findings = append(c.findings, Finding{File: file, Line: line, Text: fmt.Sprintf(format, args...)})
}

// rel - path relative to the theme's directory, with slashes.
func (c *checker) rel(path string) string {
	if r, err := filepath.Rel(c.dir, path); err == nil {
		return filepath.ToSlash(r)
	}
	return path
}

// exists - whether the theme has the file at rel (slashes).
func (c *checker) exists(rel string) bool {
	fi, err := os.Stat(filepath.Join(c.dir, filepath.FromSlash(rel)))
	return err == nil && !fi.IsDir()
}

// manifestLine - the line of theme.yaml where top-level key is set (0 -
// none).
func (c *checker) manifestLine(key string) int {
	f, err := os.Open(filepath.Join(c.dir, themes.InfoFile))
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	for n := 1; s.Scan(); n++ {
		if strings.HasPrefix(s.Text(), key+":") {
			return n
		}
	}
	return 0
}

// manifest checks theme.yaml as installing does, then what the catalog
// and the Appearance page want of it and the files it names; false - it
// doesn't load.
func (c *checker) manifest() (entity.ThemeInfo, bool) {
	info, err := themes.ReadManifestFile(filepath.Join(c.dir, themes.InfoFile))
	if err != nil {
		c.problem(themes.InfoFile, 0, "%v", err)
		return info, false
	}
	for _, f := range []struct{ key, value string }{
		{"description", info.Description}, {"author", info.Author}, {"license", info.License},
		{"tags", strings.Join(info.Tags, ",")}, {"screenshot", info.Screenshot},
	} {
		if f.value == "" {
			c.warn(themes.InfoFile, 0, "no %s: the Appearance page and the themes catalog show it", f.key)
		}
	}
	if info.Version == "" {
		c.warn(themes.InfoFile, 0, "no version: the release, 1.2.0 for the tag v1.2.0")
	}
	if info.MinVersion == "" {
		c.warn(themes.InfoFile, 0, "no min_version: the oldest engine release the theme works with")
	} else if !themes.Compatible(info.MinVersion) {
		c.warn(themes.InfoFile, c.manifestLine("min_version"), "min_version %s: newer than this engine (%s)", info.MinVersion, themes.EngineRelease())
	}
	if info.Screenshot != "" && !c.exists("static/"+info.Screenshot) {
		c.problem(themes.InfoFile, c.manifestLine("screenshot"), "screenshot %s: no static/%s", info.Screenshot, info.Screenshot)
	}
	for _, name := range info.Stylesheets {
		if !c.exists("static/css/" + name + ".css") {
			c.problem(themes.InfoFile, c.manifestLine("stylesheets"), "stylesheet %s: no static/css/%s.css", name, name)
		}
	}
	if !c.exists("README.md") {
		c.warn("", 0, "no README.md: the themes catalog asks for one, in English, with the screenshot")
	}
	return info, true
}

var (
	// themeStatic - a URL of a theme's static file: the theme, the path.
	themeStatic = regexp.MustCompile(`/static/themes/([a-z0-9][a-z0-9_-]*)/([A-Za-z0-9._~/-]+)`)
	// themeAsset - themeCSS, themeCSSAsync or themeJS of a template: the
	// function, the file's name.
	themeAsset = regexp.MustCompile(`\b(themeCSS|themeCSSAsync|themeJS)\s+"([^"]+)"`)
	// cssURL - url(…) of a stylesheet.
	cssURL = regexp.MustCompile(`url\(\s*["']?([^"')\s]+)["']?\s*\)`)

	// remoteHTML - an element loading a resource from another server: a
	// script, a stylesheet or a preconnection, a picture, a frame…
	remoteHTML = regexp.MustCompile(`(?i)<(?:script|link|img|iframe|source|video|audio|embed|object)\b[^>]*?\s(?:src|href|data|srcset)\s*=\s*["']?((?:https?:)?//[^\s"'>]+)`)
	// remoteCSS - a stylesheet's @import or url() of another server.
	remoteCSS = regexp.MustCompile(`(?i)(?:@import\s+(?:url\(\s*)?|url\(\s*)["']?((?:https?:)?//[^\s"')]+)`)
	// remoteJS - a script fetching, importing or loading from another
	// server.
	remoteJS = regexp.MustCompile("(?:\\bfetch\\(|\\bimport\\(|\\bfrom|\\.src\\s*=)\\s*[\"'`]((?:https?:)?//[^\\s\"'`]+)")
)

// files checks what the theme's templates, stylesheets, scripts and
// manifest refer to: its own static files (a missing one), another
// theme's (a warning: not its base's), other servers (the catalog
// doesn't take a theme loading from them).
func (c *checker) files(info entity.ThemeInfo) error {
	return filepath.WalkDir(c.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if (path != c.dir && strings.HasPrefix(d.Name(), ".")) || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		rel := c.rel(path)
		ext := filepath.Ext(path)
		minified := strings.HasSuffix(path, ".min.css") || strings.HasSuffix(path, ".min.js")
		switch {
		case rel == themes.InfoFile, ext == ".html" && strings.HasPrefix(rel, "templates/"):
		case (ext == ".css" || ext == ".js") && strings.HasPrefix(rel, "static/") && !minified:
		default:
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			c.line(rel, i+1, ext, line, info)
		}
		return nil
	})
}

// line checks one line of file rel (see files).
func (c *checker) line(rel string, n int, ext, line string, info entity.ThemeInfo) {
	if ext == ".js" {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") {
			return
		}
	}
	for _, m := range themeStatic.FindAllStringSubmatch(line, -1) {
		theme, file := m[1], m[2]
		switch {
		case theme == c.name:
			if !c.exists("static/" + unminified(file)) {
				c.problem(rel, n, "/static/themes/%s/%s: no static/%s", theme, file, unminified(file))
			}
		case theme != info.Base:
			c.warn(rel, n, "/static/themes/%s/%s: another theme's file (this one installs as %s)", theme, file, c.name)
		}
	}
	if ext == ".html" {
		for _, m := range themeAsset.FindAllStringSubmatch(line, -1) {
			file := "css/" + strings.TrimSuffix(m[2], ".css") + ".css"
			if m[1] == "themeJS" {
				file = "js/" + strings.TrimSuffix(strings.TrimSuffix(m[2], ".js"), ".min") + ".js"
			}
			if info.Base == "" && !c.exists("static/"+file) {
				c.problem(rel, n, "%s %q: no static/%s", m[1], m[2], file)
			}
		}
	}
	if ext == ".css" {
		for _, m := range cssURL.FindAllStringSubmatch(line, -1) {
			ref := m[1]
			if strings.Contains(ref, ":") || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "#") || strings.Contains(ref, "var(") {
				continue
			}
			ref, _, _ = strings.Cut(ref, "?")
			ref, _, _ = strings.Cut(ref, "#")
			target := filepath.ToSlash(filepath.Join(filepath.Dir(rel), ref))
			if !c.exists(target) {
				c.problem(rel, n, "url(%s): no %s", m[1], target)
			}
		}
	}
	var remote []*regexp.Regexp
	switch ext {
	case ".html", ".yaml":
		remote = []*regexp.Regexp{remoteHTML, remoteCSS}
	case ".css":
		remote = []*regexp.Regexp{remoteCSS}
	case ".js":
		remote = []*regexp.Regexp{remoteHTML, remoteJS}
	}
	for _, re := range remote {
		for _, m := range re.FindAllStringSubmatch(line, -1) {
			c.problem(rel, n, "loads %s from %s: the themes catalog takes no third-party resources - ship the file with the theme", m[1], host(m[1]))
		}
	}
}

// unminified - file without ".min" (the site builds x.min.css from
// x.css).
func unminified(file string) string {
	for _, ext := range []string{".css", ".js"} {
		if strings.HasSuffix(file, ".min"+ext) {
			return strings.TrimSuffix(file, ".min"+ext) + ext
		}
	}
	return file
}

// host - the server of address (https://host/…, //host/…).
func host(address string) string {
	if strings.HasPrefix(address, "//") {
		address = "https:" + address
	}
	if u, err := url.Parse(address); err == nil && u.Host != "" {
		return u.Host
	}
	return address
}

// translations checks each language's catalog (translations/<lang>.json,
// the source language's left out) against the theme's strings.
func (c *checker) translations(engine []i18n.Source) error {
	msgs, err := i18nextract.ExtractTheme(c.dir, c.name)
	if err != nil {
		return err
	}
	key := func(ctx, source string) string { return ctx + "\x04" + source }
	engines := map[string]bool{}
	for _, s := range engine {
		if s.Theme == "" {
			engines[key(s.Context, s.Source)] = true
		}
	}
	// own - the theme's strings the engine doesn't translate, each once
	// (where it is first used).
	var own []i18nextract.Message
	used := map[string]bool{}
	for _, m := range msgs {
		k := key(m.Context, m.Source)
		if used[k] {
			continue
		}
		used[k] = true
		if !engines[k] {
			own = append(own, m)
		}
	}
	catalogs, err := filepath.Glob(filepath.Join(c.dir, "translations", "*.json"))
	if err != nil {
		return err
	}
	slices.Sort(catalogs)
	if len(catalogs) == 0 && len(own) > 0 {
		c.warn("translations/", 0, "no catalogs: the theme's %d strings show in English in every language", len(own))
	}
	for _, path := range catalogs {
		lang := strings.TrimSuffix(filepath.Base(path), ".json")
		if lang == i18n.SourceLanguage {
			continue
		}
		rel := c.rel(path)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries, err := i18n.ParseCatalog(lang, b)
		if err != nil {
			c.problem(rel, 0, "%v", err)
			continue
		}
		byKey := make(map[string]i18n.Entry, len(entries))
		for _, e := range entries {
			byKey[key(e.Context, e.Source)] = e
		}
		forms := len(i18n.PluralCategories(lang))
		for _, m := range own {
			e, ok := byKey[key(m.Context, m.Source)]
			switch {
			case !ok:
				c.problem(c.rel(m.File), m.Line, "%q: not in %s", m.Source, rel)
			case m.Plural != "" && len(e.Forms) == 0:
				c.problem(rel, 0, "%q: a plural string, its %d forms (%s) needed", m.Source, forms, strings.Join(i18n.PluralCategories(lang), ", "))
			default:
				if err := i18npack.CheckEntry(e, forms); err != nil {
					c.problem(rel, 0, "%q: %v", m.Source, err)
				}
			}
		}
		// An engine's or a module's string the catalog rewords is the
		// theme's choice; any other the theme doesn't use is left over.
		for _, e := range entries {
			if k := key(e.Context, e.Source); !used[k] && !engines[k] {
				c.warn(rel, 0, "%q: the theme doesn't use it", e.Source)
			}
		}
	}
	return nil
}

// demo checks the theme's own demo sets (demo/<lang>.yaml, app install
// -demo-set <theme>): each loads as the engine's sets do, its own
// pictures in demo/, its blocks and menus in the theme's regions (a subtheme's, without regions of its
// own, are its base's: not checked).
func (c *checker) demo(info entity.ThemeInfo, manifestOK bool) error {
	sets, err := filepath.Glob(filepath.Join(c.dir, "demo", "*.yaml"))
	if err != nil {
		return err
	}
	slices.Sort(sets)
	for _, path := range sets {
		rel := c.rel(path)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		set, err := demoset.Parse(b)
		if err != nil {
			c.problem(rel, 0, "%v", err)
			continue
		}
		// Its own files (an image or a file field's {file: …}): in demo/,
		// pictures images, documents no pages or scripts.
		for _, err := range set.CheckFiles(os.DirFS(filepath.Join(c.dir, "demo"))) {
			c.problem(rel, 0, "%v", err)
		}
		if !manifestOK || len(info.Regions) == 0 {
			continue
		}
		for _, r := range set.Regions() {
			if !slices.ContainsFunc(info.Regions, func(tr entity.ThemeRegion) bool { return string(tr.ID) == r }) {
				c.problem(rel, 0, "region %q: not one of the theme's", r)
			}
		}
	}
	return nil
}

// release compares version (theme.yaml) with the latest release tag of
// the theme's own git repository: older - a problem (installing warns),
// newer - not tagged yet, the same with commits or changes after the tag
// - not raised for the next release. A theme inside another repository
// (a site's) has no tags of its own: nothing to compare.
func (c *checker) release(ctx context.Context, in Installer, version string) {
	top, err := in.git(ctx, c.dir, "rev-parse", "--show-toplevel")
	if err != nil || !sameDir(top, c.dir) {
		return
	}
	out, err := in.git(ctx, c.dir, "tag", "--list", "v*")
	if err != nil {
		c.warn("", 0, "the release tags: %v", err)
		return
	}
	var tags []string
	for _, t := range strings.Fields(out) {
		if semver.IsValid(t) && semver.Prerelease(t) == "" && t == semver.Canonical(t) {
			tags = append(tags, t)
		}
	}
	if version == "" {
		return
	}
	line := c.manifestLine("version")
	if len(tags) == 0 {
		c.warn(themes.InfoFile, line, "version %s: no release yet - tag it v%s", version, version)
		return
	}
	semver.Sort(tags)
	latest := tags[len(tags)-1]
	switch cmp := semver.Compare("v"+version, latest); {
	case cmp < 0:
		c.problem(themes.InfoFile, line, "version %s is older than the latest release %s: installing it warns", version, latest)
	case cmp > 0:
		c.warn(themes.InfoFile, line, "version %s isn't released yet (the latest is %s): tag it v%s", version, latest, version)
	default:
		after, _ := in.git(ctx, c.dir, "rev-list", "--count", latest+"..HEAD")
		changes, _ := in.git(ctx, c.dir, "status", "--porcelain")
		if (after != "" && after != "0") || changes != "" {
			c.warn(themes.InfoFile, line, "version %s is the release %s, with changes after it: raise it for the next release", version, latest)
		}
	}
}

// sameDir - whether a and b are one directory (symbolic links resolved).
func sameDir(a, b string) bool {
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && ra == rb
}
