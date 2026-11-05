package app

import (
	"bytes"
	"strings"
	"testing"
)

// TestDispatch - a command runs instead of the server; no arguments or
// only the server's flags serve; an unknown command is an error with the
// usage, never the server.
func TestDispatch(t *testing.T) {
	var errOut bytes.Buffer
	if _, ok := dispatch(nil, &errOut); ok {
		t.Error("no arguments: a command, want the server")
	}
	if _, ok := dispatch([]string{"-config", "x.yaml"}, &errOut); ok {
		t.Error("the server's flag: a command, want the server")
	}
	// The config before a command: the command runs, its own flags its.
	commands["probe"] = func(args []string) int { return len(args) }
	t.Cleanup(func() { delete(commands, "probe") })
	for _, args := range [][]string{{"-config", "x.yaml", "probe", "-apply"}, {"-config=x.yaml", "probe", "-apply"}} {
		if code, ok := dispatch(args, &errOut); !ok || code != 1 {
			t.Errorf("%v: %d %v, want the command with its one flag", args, code, ok)
		}
	}
	errOut.Reset()
	if code, ok := dispatch([]string{"frobnicate"}, &errOut); !ok || code != 2 || !strings.Contains(errOut.String(), `unknown command "frobnicate"`) || !strings.Contains(errOut.String(), "app migrate") {
		t.Errorf("an unknown command: %d %v %q", code, ok, errOut.String())
	}
	errOut.Reset()
	if code, ok := dispatch([]string{"help"}, &errOut); !ok || code != 0 || !strings.Contains(errOut.String(), "app theme") {
		t.Errorf("help: %d %v %q", code, ok, errOut.String())
	}
	if code, ok := dispatch([]string{"theme"}, &errOut); !ok || code != 2 {
		t.Errorf("theme without its command: %d %v", code, ok)
	}
}
