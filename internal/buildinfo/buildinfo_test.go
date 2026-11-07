package buildinfo

import (
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	i := Get()
	if i.GoVersion != runtime.Version() {
		t.Errorf("Go version %q", i.GoVersion)
	}
	if (Info{Commit: "0123456789abcdef"}).ShortCommit() != "01234567" || (Info{Commit: "abc"}).ShortCommit() != "abc" {
		t.Error("ShortCommit")
	}
}

// vcs - the build settings Go records for a git checkout.
func vcs(commit, at string, modified bool) []debug.BuildSetting {
	m := "false"
	if modified {
		m = "true"
	}
	return []debug.BuildSetting{{Key: "vcs.revision", Value: commit}, {Key: "vcs.time", Value: at}, {Key: "vcs.modified", Value: m}}
}

// TestFromBuild - the engine's own binary: its commit Go recorded; a
// site's own binary: the site's module and commit, the engine's version
// from the dependency - a release's tag, or a commit's pseudo-version
// with the commit and its time; an engine replaced by a tree: only the
// build's version.
func TestFromBuild(t *testing.T) {
	engine := &debug.BuildInfo{Main: debug.Module{Path: EngineModule}, Settings: vcs("0123456789abcdef0123456789abcdef01234567", "2026-11-07T10:00:00Z", true)}
	i := fromBuild(engine, "v0.1.2", "false")
	if i.Version != "v0.1.2" || i.ShortCommit() != "01234567" || i.Time.IsZero() || i.Modified || i.Site.Module != "" {
		t.Errorf("the engine's binary: %+v", i)
	}

	site := func(engineVersion string, replace *debug.Module) *debug.BuildInfo {
		return &debug.BuildInfo{
			Main:     debug.Module{Path: "example.com/site"},
			Settings: vcs("fedcba9876543210fedcba9876543210fedcba98", "2026-11-07T12:00:00Z", false),
			Deps:     []*debug.Module{{Path: "example.com/other", Version: "v1.0.0"}, {Path: EngineModule, Version: engineVersion, Replace: replace}},
		}
	}
	i = fromBuild(site("v0.1.2", nil), "", "")
	if i.Version != "v0.1.2" || i.Commit != "" || i.ReleaseURL() != "https://github.com/kosatka-cms/kosatka/releases/tag/v0.1.2" {
		t.Errorf("a site on a release: %+v", i)
	}
	if i.Site.Module != "example.com/site" || i.Site.ShortCommit() != "fedcba98" || i.Site.Time.IsZero() {
		t.Errorf("the site's build: %+v", i.Site)
	}

	i = fromBuild(site("v0.1.3-0.20261107132919-a147fac63fe0", nil), "", "")
	want := time.Date(2026, 11, 7, 13, 29, 19, 0, time.UTC)
	if i.Version != "" || i.Commit != "a147fac63fe0" || !i.Time.Equal(want) || i.ReleaseURL() != "" {
		t.Errorf("a site on a commit: %+v", i)
	}

	i = fromBuild(site("v0.0.0", &debug.Module{Path: "../"}), "v0.1.2", "false")
	if i.Version != "v0.1.2" || i.Commit != "" || i.Site.Modified {
		t.Errorf("a site on an engine tree: %+v", i)
	}

	if (Info{Version: "v1.2.0-3-g1a2b3c4d"}).ReleaseURL() != "" || (Info{}).ReleaseURL() != "" {
		t.Error("ReleaseURL of a build past a tag or of none")
	}
}
