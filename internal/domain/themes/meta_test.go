package themes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// TestCheckMeta - a manifest's description: releases as "1.2.0", an
// http(s) homepage, an SPDX license, lowercase tags, a screenshot in the
// theme's static directory.
func TestCheckMeta(t *testing.T) {
	ok := entity.ThemeInfo{Version: "1.2.0", MinVersion: "0.2.0", Homepage: "https://example.com/theme",
		License: "MIT OR Apache-2.0", Tags: []string{"dark", "two-columns"}, Screenshot: "images/screenshot.jpg"}
	if err := checkMeta(ok); err != nil {
		t.Fatalf("a valid description: %v", err)
	}
	for name, broken := range map[string]func(*entity.ThemeInfo){
		"a version with v":        func(i *entity.ThemeInfo) { i.Version = "v1.2.0" },
		"a short min_version":     func(i *entity.ThemeInfo) { i.MinVersion = "0.2" },
		"a javascript homepage":   func(i *entity.ThemeInfo) { i.Homepage = "javascript:alert(1)" },
		"a homepage without host": func(i *entity.ThemeInfo) { i.Homepage = "https://" },
		"a license with a quote":  func(i *entity.ThemeInfo) { i.License = `MIT"` },
		"a capital tag":           func(i *entity.ThemeInfo) { i.Tags = []string{"Dark"} },
		"too many tags": func(i *entity.ThemeInfo) {
			i.Tags = strings.Split(strings.Repeat("a,", maxTags+1), ",")[:maxTags+1]
		},
		"a screenshot up a dir": func(i *entity.ThemeInfo) { i.Screenshot = "../secret.png" },
		"a screenshot as svg":   func(i *entity.ThemeInfo) { i.Screenshot = "screenshot.svg" },
	} {
		info := ok
		info.Tags = append([]string(nil), ok.Tags...)
		broken(&info)
		if err := checkMeta(info); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// TestReleaseMeets - the release of a build (git describe) and whether it
// meets a theme's min_version; a developer's build meets every one.
func TestReleaseMeets(t *testing.T) {
	for version, want := range map[string]string{
		"v1.2.0":               "1.2.0",
		"v1.2.0-3-g1a2b3c4d":   "1.2.0",
		"v0.10.1-144-g0a8bc41": "0.10.1",
		"":                     "",
		"1a2b3c4d":             "",
	} {
		if got := releaseOf(version); got != want {
			t.Errorf("releaseOf(%q) = %q, want %q", version, got, want)
		}
	}
	for _, c := range []struct {
		release, min string
		want         bool
	}{
		{"1.2.0", "1.2.0", true},
		{"1.10.0", "1.9.3", true},
		{"0.1.0", "0.2.0", false},
		{"1.2.0", "2.0.0", false},
		{"", "9.9.9", true},
		{"0.1.0", "", true},
	} {
		if got := releaseMeets(c.release, c.min); got != c.want {
			t.Errorf("releaseMeets(%q, %q) = %v, want %v", c.release, c.min, got, c.want)
		}
	}
}

// TestThemeScreenshotsExist - every theme's screenshot (theme.yaml) is a
// file in its static directory, or its card would show a broken picture.
func TestThemeScreenshotsExist(t *testing.T) {
	zzChdirToProjectRoot(t)
	themes, err := Available()
	if err != nil {
		t.Fatal(err)
	}
	for _, th := range themes {
		if th.Screenshot == "" {
			continue
		}
		url := strings.SplitN(th.Screenshot, "?", 2)[0]
		file := filepath.Join("web", strings.TrimPrefix(url, "/"))
		if rel, ok := strings.CutPrefix(url, "/static/themes/"+th.Dir+"/"); ok && th.Dir != "" {
			file = filepath.Join(paths.ThemeStaticDir(th.Dir), rel)
		}
		if _, err := os.Stat(file); err != nil {
			t.Errorf("theme %q: its screenshot %s: %v", th.Dir, file, err)
		}
	}
}
