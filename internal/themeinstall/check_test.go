package themeinstall

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// writeTheme writes files (path relative to dir → content) into dir.
func writeTheme(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// findingTexts - the findings as their String()s.
func findingTexts(findings []Finding) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = f.String()
	}
	return out
}

// sandManifest - a complete manifest of the theme "sand" (no repository).
const sandManifest = `name: Sand
description: A warm light theme.
author: Example
license: MIT
version: 1.0.0
min_version: 0.1.0
tags: [light]
screenshot: screenshot.jpg
favicon: /static/themes/sand/images/icon.svg
regions:
  - id: sidebar
    label: Sidebar
`

// TestCheckClean - a theme with its files, its strings translated and
// nothing from other servers: no findings; outside a repository of its
// own no release is compared.
func TestCheckClean(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kosatka-theme-sand")
	writeTheme(t, dir, map[string]string{
		"theme.yaml":                sandManifest,
		"README.md":                 "# Sand\n",
		"static/screenshot.jpg":     "jpg",
		"static/images/icon.svg":    "<svg/>",
		"static/css/sand.css":       ".a { background: url(../images/icon.svg); } .b { background: url(data:image/png;base64,AA); }",
		"static/js/sand.js":         "// see https://example.com/docs\nKosatka.t('Copied');\n",
		"templates/header.html":     `{{define "header"}}<a href="https://example.com">{{t "Edit"}}</a> {{plural .n "@count grain" "@count grains"}}{{themeJS "sand"}}<img src="/static/themes/sand/images/icon.svg">{{end}}`,
		"translations/ru.json":      `{"strings": {"Sand": "Песок", "Sidebar": "Боковая", "A warm light theme.": "Тёплая светлая тема.", "light": "светлая", "Copied": "Скопировано", "@count grain": ["@count песчинка", "@count песчинки", "@count песчинок"]}}`,
		"translations/en.json":      `{"strings": {}}`,
		"static/css/sand.min.css":   "@import url(https://cdn.example.com/x.css);",
		".git-ignored/whatever.css": "@import url(https://cdn.example.com/y.css);",
	})
	engine := []i18n.Source{{Source: "Edit"}}
	findings, err := Installer{}.Check(t.Context(), dir, engine)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings:\n%s", strings.Join(findingTexts(findings), "\n"))
	}
}

// TestCheckFindings - what Check reports: the manifest's gaps and the
// files it names, the theme's own files missing, resources from other
// servers, a catalog's missing, wrong and left-over strings.
func TestCheckFindings(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sand")
	writeTheme(t, dir, map[string]string{
		"theme.yaml":            "name: Sand\nscreenshot: shot.jpg\nstylesheets: [extra]\nregions:\n  - id: sidebar\n    label: Sidebar\n",
		"templates/header.html": `{{define "header"}}<link rel="preconnect" href="https://fonts.example.com">{{t "Hello @name"}}{{themeCSS "fonts"}}<img src="/static/themes/sand/images/gone.png"><img src="/static/themes/other/x.png">{{end}}`,
		"static/css/sand.css":   "@font-face { src: url(../fonts/gone.woff2); }\n@import url(\"//cdn.example.com/a.css\");",
		"static/js/sand.js":     "fetch('https://api.example.com/x');\nKosatka.formatPlural(n, '@count grain', '@count grains');",
		"translations/ru.json":  `{"strings": {"Sand": "Песок", "Hello @name": "Привет", "@count grain": "песчинки", "Unused": "Лишняя"}}`,
		"translations/xx.json":  `{"strings": `,
	})
	findings, err := Installer{}.Check(t.Context(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := findingTexts(findings)
	for _, want := range []string{
		"warning: theme.yaml: no description: the Appearance page and the themes catalog show it",
		"warning: theme.yaml: no version: the release, 1.2.0 for the tag v1.2.0",
		"problem: theme.yaml:2: screenshot shot.jpg: no static/shot.jpg",
		"problem: theme.yaml:3: stylesheet extra: no static/css/extra.css",
		"warning: no README.md: the themes catalog asks for one, in English, with the screenshot",
		"problem: static/css/sand.css:1: url(../fonts/gone.woff2): no static/fonts/gone.woff2",
		"problem: static/css/sand.css:2: loads //cdn.example.com/a.css from cdn.example.com: the themes catalog takes no third-party resources - ship the file with the theme",
		"problem: static/js/sand.js:1: loads https://api.example.com/x from api.example.com: the themes catalog takes no third-party resources - ship the file with the theme",
		"problem: templates/header.html:1: loads https://fonts.example.com from fonts.example.com: the themes catalog takes no third-party resources - ship the file with the theme",
		"problem: templates/header.html:1: themeCSS \"fonts\": no static/css/fonts.css",
		"problem: templates/header.html:1: /static/themes/sand/images/gone.png: no static/images/gone.png",
		"warning: templates/header.html:1: /static/themes/other/x.png: another theme's file (this one installs as sand)",
		"problem: theme.yaml:6: \"Sidebar\": not in translations/ru.json",
		"problem: translations/ru.json: \"Hello @name\": the translation drops the placeholder @name",
		"problem: translations/ru.json: \"@count grain\": a plural string, its 3 forms (one, few, many) needed",
		"warning: translations/ru.json: \"Unused\": the theme doesn't use it",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
	if !slices.ContainsFunc(got, func(s string) bool {
		return strings.HasPrefix(s, "problem: translations/xx.json: i18n: the xx catalog:")
	}) {
		t.Errorf("a broken catalog isn't reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestCheckRelease - the version against the repository's latest release
// tag: older - a problem, newer - not released yet, the same with commits
// after the tag - not raised.
func TestCheckRelease(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := filepath.Join(t.TempDir(), "kosatka-theme-sand")
	manifest := func(version string) map[string]string {
		return map[string]string{"theme.yaml": "name: Sand\nversion: " + version + "\nregions:\n  - id: sidebar\n    label: Sidebar\n"}
	}
	writeTheme(t, dir, manifest("1.0.0"))
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	release := func() string {
		t.Helper()
		findings, err := Installer{}.Check(t.Context(), dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range findings {
			if strings.Contains(f.Text, "version 1.") {
				return f.String()
			}
		}
		return ""
	}
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-qm", "first")
	if got, want := release(), "warning: theme.yaml:2: version 1.0.0: no release yet - tag it v1.0.0"; got != want {
		t.Errorf("no tags: %q, want %q", got, want)
	}
	git("tag", "v1.0.0")
	git("tag", "v1.1.0-rc1")
	if got := release(); got != "" {
		t.Errorf("released: %q, want nothing", got)
	}
	writeTheme(t, dir, map[string]string{"README.md": "# Sand\n"})
	if got, want := release(), "warning: theme.yaml:2: version 1.0.0 is the release v1.0.0, with changes after it: raise it for the next release"; got != want {
		t.Errorf("changed: %q, want %q", got, want)
	}
	writeTheme(t, dir, manifest("1.1.0"))
	if got, want := release(), "warning: theme.yaml:2: version 1.1.0 isn't released yet (the latest is v1.0.0): tag it v1.1.0"; got != want {
		t.Errorf("raised: %q, want %q", got, want)
	}
	git("add", "-A")
	git("commit", "-qm", "1.1.0")
	git("tag", "v1.1.0")
	git("tag", "v1.1.1")
	if got, want := release(), "problem: theme.yaml:2: version 1.1.0 is older than the latest release v1.1.1: installing it warns"; got != want {
		t.Errorf("behind: %q, want %q", got, want)
	}
}
