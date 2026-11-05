//go:build dbintegration

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestTranslationsCommand - "app translations" on a site with a catalog
// built by "app translations index" from a translations repository (a
// local one; a string that isn't the code's left out as stale): list -
// English (built in; Russian isn't, nor in this catalog), the catalog's
// German; add - German installed; update - up to date, then the catalog's
// new version; remove - gone, again - refused; without a catalog - English
// only.
func TestTranslationsCommand(t *testing.T) {
	repo := t.TempDir()
	dist := filepath.Join(t.TempDir(), "dist")
	index := filepath.Join(dist, "index.json")
	if err := os.MkdirAll(filepath.Join(repo, "de"), 0o755); err != nil {
		t.Fatal(err)
	}
	publish := func(s *site, version, pack string) string {
		t.Helper()
		for name, content := range map[string]string{"language.yaml": "version: " + version + "\ndraft: true\n", "engine.json": pack} {
			if err := os.WriteFile(filepath.Join(repo, "de", name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return s.app("translations", "index", "-src", repo, "-out", dist)
	}

	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSite(t, db, nil)
		if out := publish(s, "0.1.0", `{"strings": {"Forgot your password?": "Passwort vergessen?", "Create an account": "Konto erstellen", "Not a string of the site": "Nein"}}`); !strings.Contains(out, "de 0.1.0: 2 of ") || !strings.Contains(out, "1 stale left out (a draft)") {
			t.Errorf("index:\n%s", out)
		}
		lines := func(out string) map[string]string {
			m := map[string]string{}
			for _, l := range strings.Split(out, "\n") {
				if f := strings.Fields(l); len(f) > 0 {
					m[f[0]] = l
				}
			}
			return m
		}
		got := lines(s.app("translations", "list", "-catalog", index))
		if !strings.Contains(got["en"], "the source strings") || got["ru"] != "" ||
			!strings.Contains(got["de"], "Deutsch") || !strings.Contains(got["de"], "available") || !strings.Contains(got["de"], "a draft") {
			t.Errorf("list before:\n%v", got)
		}
		if out := s.app("translations", "add", "de", "-catalog", index); !strings.Contains(out, "de: installed, 2 strings\n") {
			t.Errorf("add:\n%s", out)
		}
		if got := lines(s.app("translations", "list", "-catalog", index)); !strings.Contains(got["de"], "installed") || !strings.Contains(got["de"], "0.1.0") {
			t.Errorf("list after add:\n%v", got)
		}
		if out := s.app("translations", "update", "de", "-catalog", index); !strings.Contains(out, "de: up to date (0.1.0)") {
			t.Errorf("update when up to date:\n%s", out)
		}
		publish(s, "0.2.0", `{"strings": {"Forgot your password?": "Passwort vergessen?", "Create an account": "Konto erstellen", "Log in": "Anmelden"}}`)
		if got := lines(s.app("translations", "list", "-catalog", index)); !strings.Contains(got["de"], "0.1.0 -> 0.2.0") {
			t.Errorf("list with an update:\n%v", got)
		}
		if out := s.app("translations", "update", "-catalog", index); !strings.Contains(out, "de: updated, 3 strings") {
			t.Errorf("update:\n%s", out)
		}
		if out := s.app("translations", "update", "-catalog", index); !strings.Contains(out, "the downloaded languages are up to date") {
			t.Errorf("update again:\n%s", out)
		}
		var n int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM interface_strings WHERE lang = 'de'`).Scan(&n); err != nil || n != 3 {
			t.Errorf("the German strings: %d (%v)", n, err)
		}
		if out := s.app("translations", "remove", "de"); !strings.Contains(out, "de: removed") {
			t.Errorf("remove:\n%s", out)
		}
		again := exec.Command(s.bin, "translations", "remove", "de")
		again.Dir, again.Env = repoRoot(t), s.env
		if out, err := again.CombinedOutput(); err == nil || !strings.Contains(string(out), "de isn't a downloaded language") {
			t.Errorf("remove again: %v\n%s", err, out)
		}
		if got := s.app("translations", "list"); !strings.Contains(got, "no translations catalog") || strings.Contains(got, "Deutsch") {
			t.Errorf("list without a catalog:\n%s", got)
		}
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM interface_packs`).Scan(&n); err != nil || n != 0 {
			t.Errorf("interface_packs: %d (%v)", n, err)
		}
	})
}
