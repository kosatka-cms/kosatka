// Package buildinfo - what the running binary is: the engine's release,
// the commit and time it was built from (recorded by the Go toolchain from
// git, debug.ReadBuildInfo), the Go version; for a site's own binary (its
// main calls kosatka.Main) - the engine as its dependency and the site's
// module.
package buildinfo

import (
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// EngineModule - the engine's module path.
const EngineModule = "github.com/kosatka-cms/kosatka"

// Version - the engine's release, set at build time from the git tags:
// by the release workflow (the tag), the Dockerfile and
// scripts/build_linux_release.sh (git describe: "v1.2.0" on a tagged
// commit, "v1.2.0-3-g1a2b3c4d" three commits after it); empty - a plain
// go build, or no release tag yet (a site's own binary then takes the
// engine's version from its dependency).
var Version string

// TreeModified - "true"/"false": whether what goes to production had
// uncommitted changes, set by scripts/build_linux_release.sh
// (-ldflags "-X …/buildinfo.TreeModified=false"); over Go's vcs.modified,
// which any untracked file in the working copy sets. Empty - Go's.
var TreeModified string

// Info - the running binary.
type Info struct {
	// Version - the engine's release; Commit, Time - the engine's commit
	// and its time ("", zero - unknown: a release taken as a dependency
	// has its tag only, see ReleaseURL).
	Version   string
	Commit    string
	Time      time.Time
	Modified  bool
	GoVersion string
	// Site - the site's own binary the engine is built into (its module
	// and the commit it was built from); a zero Site - the engine's own.
	Site Site
}

// Site - a site's own binary: its main module (kosatka.Main with the
// modules from outside the engine) and the git commit Go recorded for it.
type Site struct {
	Module   string
	Commit   string
	Time     time.Time
	Modified bool
}

var (
	once sync.Once
	info Info
)

// Get - the running binary's info (read once).
func Get() Info {
	once.Do(func() {
		bi, _ := debug.ReadBuildInfo()
		info = fromBuild(bi, Version, TreeModified)
	})
	return info
}

// fromBuild - the info of a binary built as bi (nil - unknown) with the
// version and treeModified its build set.
func fromBuild(bi *debug.BuildInfo, version, treeModified string) Info {
	i := Info{Version: version, GoVersion: runtime.Version()}
	if bi == nil {
		return i
	}
	// Go records the main module's commit: the engine's own binary's, or
	// a site's binary's.
	var commit string
	var at time.Time
	var modified bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.time":
			at, _ = time.Parse(time.RFC3339, s.Value)
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if treeModified != "" {
		modified = treeModified == "true"
	}
	if bi.Main.Path == "" || bi.Main.Path == EngineModule {
		i.Commit, i.Time, i.Modified = commit, at, modified
		return i
	}
	i.Site = Site{Module: bi.Main.Path, Commit: commit, Time: at, Modified: modified}
	for _, d := range bi.Deps {
		if d.Path != EngineModule {
			continue
		}
		// Replaced by a directory (an engine tree): its version says
		// nothing, the build's own Version does.
		if d.Replace != nil {
			break
		}
		if module.IsPseudoVersion(d.Version) {
			// A commit, not a release: the version holds the commit
			// and its time.
			i.Commit, _ = module.PseudoVersionRev(d.Version)
			i.Time, _ = module.PseudoVersionTime(d.Version)
		} else if i.Version == "" {
			i.Version = d.Version
		}
		break
	}
	return i
}

// ShortCommit - the commit's first 8 characters ("" - unknown).
func (i Info) ShortCommit() string {
	return short(i.Commit)
}

// ShortCommit - the site's commit's first 8 characters ("" - unknown).
func (s Site) ShortCommit() string {
	return short(s.Commit)
}

func short(commit string) string {
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
}

// ReleaseURL - the page of the engine's release Version ("" - Version
// isn't a release: empty, or a git describe past a tag).
func (i Info) ReleaseURL() string {
	if !semver.IsValid(i.Version) || semver.Prerelease(i.Version) != "" || semver.Build(i.Version) != "" {
		return ""
	}
	return "https://" + EngineModule + "/releases/tag/" + i.Version
}
