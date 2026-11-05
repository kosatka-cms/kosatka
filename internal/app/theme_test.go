package app

import (
	"bytes"
	"strings"
	"testing"
)

// TestSplitVersion - "<repository or name>@<version>"; the "@" of an ssh
// address isn't a version's.
func TestSplitVersion(t *testing.T) {
	for arg, want := range map[string][2]string{
		"sand":        {"sand", ""},
		"sand@v1.2.0": {"sand", "v1.2.0"},
		"https://github.com/example/kosatka-theme-sand": {"https://github.com/example/kosatka-theme-sand", ""},
		"https://github.com/example/sand@1.2.0":         {"https://github.com/example/sand", "1.2.0"},
		"git@github.com:example/sand.git":               {"git@github.com:example/sand.git", ""},
		"git@github.com:example/sand.git@v2.0.0":        {"git@github.com:example/sand.git", "v2.0.0"},
	} {
		if source, version := splitVersion(arg); source != want[0] || version != want[1] {
			t.Errorf("splitVersion(%q) = %q, %q, want %q, %q", arg, source, version, want[0], want[1])
		}
	}
}

// TestThemeCommandUsage - an unknown or missing command prints the usage
// and exits 2.
func TestThemeCommandUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"frobnicate"}, {"add"}, {"update"}, {"remove", "a", "b"}} {
		var out, errOut bytes.Buffer
		if code := themeCommand(args, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "app theme add") {
			t.Errorf("%v: exit %d, stderr %q", args, code, errOut.String())
		}
	}
}
