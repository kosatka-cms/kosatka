package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTranslationsCommandUsage - an unknown command, a missing language or
// one too many prints the usage and exits 2, before the config is read.
func TestTranslationsCommandUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"frobnicate"}, {"add"}, {"remove"}, {"add", "de", "fr"}, {"update", "de", "fr"}, {"list", "de"}, {"add", "-nope", "de"}, {"index", "extra"}, {"index", "-nope"}} {
		var out, errOut bytes.Buffer
		if code := translationsCommand(args, &out, &errOut); code != 2 {
			t.Errorf("%v: exit %d, stderr %q", args, code, errOut.String())
		}
	}
	var out, errOut bytes.Buffer
	if translationsCommand(nil, &out, &errOut); !strings.Contains(errOut.String(), "app translations add <lang>") {
		t.Errorf("the usage: %q", errOut.String())
	}
}

// TestTranslationsIndex - "app translations index" on a repository with
// the code's strings from a file: the catalog written, a line per
// language; a -complete language missing a string, or a string that
// can't stand for its source - failed, nothing written.
func TestTranslationsIndex(t *testing.T) {
	repo, sources := t.TempDir(), filepath.Join(t.TempDir(), "strings.json")
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(sources, `[{"source": "Save"}, {"source": "Vote", "module": "polls"}]`)
	write(filepath.Join(repo, "de", "language.yaml"), "version: 0.1.0\n")
	write(filepath.Join(repo, "de", "engine.json"), `{"strings": {"Save": "Speichern"}}`)
	run := func(args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		code := translationsCommand(append([]string{"index", "-src", repo, "-sources", sources}, args...), &out, &errOut)
		return code, out.String(), errOut.String()
	}

	dist := filepath.Join(t.TempDir(), "dist")
	if code, out, errOut := run("-out", dist); code != 0 || !strings.Contains(out, "de 0.1.0: 1 of 2 strings") {
		t.Errorf("index: %d\n%s%s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dist, "de.json")); err != nil {
		t.Errorf("no de.json: %v", err)
	}

	dist = filepath.Join(t.TempDir(), "dist")
	if code, _, errOut := run("-out", dist, "-complete", "de"); code != 1 || !strings.Contains(errOut, "de must be complete; 1 strings missing") || !strings.Contains(errOut, `"Vote"`) {
		t.Errorf("-complete de: %d\n%s", code, errOut)
	}
	write(filepath.Join(repo, "de", "polls.json"), `{"strings": {"Vote": "<script>Abstimmen</script>"}}`)
	if code, _, errOut := run("-out", dist); code != 1 || !strings.Contains(errOut, `de/polls.json: "Vote"`) {
		t.Errorf("a string that can't stand: %d\n%s", code, errOut)
	}
	if _, err := os.Stat(dist); !os.IsNotExist(err) {
		t.Errorf("a failed index wrote %s: %v", dist, err)
	}
}
