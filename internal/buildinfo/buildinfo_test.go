package buildinfo

import (
	"runtime"
	"testing"
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
