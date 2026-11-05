package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestJSKeysIn - the strings of the engine directory's scripts and of a
// site's own theme beside it, each once.
func TestJSKeysIn(t *testing.T) {
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	engine, site := t.TempDir(), t.TempDir()
	write(filepath.Join(engine, "web", "static", "js", "app.js"), `Kosatka.t('Engine string'); Kosatka.t('Both');`)
	write(filepath.Join(site, "themes", "mine", "static", "js", "mine.js"), `Kosatka.t('Theme string'); Kosatka.t('Both');`)
	keys := jsKeysIn(slog.New(slog.NewTextHandler(io.Discard, nil)), []string{engine, site})
	for _, want := range []string{"Engine string", "Theme string", "Both"} {
		if n := len(slices.DeleteFunc(slices.Clone(keys), func(k i18n.JSKey) bool { return k.Source != want })); n != 1 {
			t.Errorf("%q: %d times", want, n)
		}
	}
}
