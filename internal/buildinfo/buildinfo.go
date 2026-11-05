// Package buildinfo - what the running binary is: the engine's release,
// the commit and time it was built from (recorded by the Go toolchain from
// git, debug.ReadBuildInfo), the Go version.
package buildinfo

import (
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// Version - the engine's release, set at build time from the git tags:
// by the release workflow (the tag), the Dockerfile and
// scripts/build_linux_release.sh (git describe: "v1.2.0" on a tagged
// commit, "v1.2.0-3-g1a2b3c4d" three commits after it); empty - a plain
// go build, or no release tag yet.
var Version string

// TreeModified - "true"/"false": whether what goes to production had
// uncommitted changes, set by scripts/build_linux_release.sh
// (-ldflags "-X …/buildinfo.TreeModified=false"); over Go's vcs.modified,
// which any untracked file in the working copy sets. Empty - Go's.
var TreeModified string

// Info - the running binary.
type Info struct {
	Version   string
	Commit    string
	Time      time.Time
	Modified  bool
	GoVersion string
}

var (
	once sync.Once
	info Info
)

// Get - the running binary's info (read once).
func Get() Info {
	once.Do(func() {
		info = Info{Version: Version, GoVersion: runtime.Version()}
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Commit = s.Value
			case "vcs.time":
				info.Time, _ = time.Parse(time.RFC3339, s.Value)
			case "vcs.modified":
				info.Modified = s.Value == "true"
			}
		}
		if TreeModified != "" {
			info.Modified = TreeModified == "true"
		}
	})
	return info
}

// ShortCommit - the commit's first 8 characters ("" - unknown).
func (i Info) ShortCommit() string {
	if len(i.Commit) > 8 {
		return i.Commit[:8]
	}
	return i.Commit
}
