//go:build dbintegration

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestInstallDownloadedLanguage - "app install -language de" with German
// in the translations catalog (-translations, a local index.json): the
// language downloaded before the starter content, which is written in it
// (the content types' names), the site's language German, a guest's login
// page in it; a language neither built in nor in the catalog, or one not
// built in without a catalog - refused before anything is installed.
func TestInstallDownloadedLanguage(t *testing.T) {
	dir := t.TempDir()
	pack := `{"strings": {"Article": "Artikel", "Page": "Seite", "Forgot your password?": "Passwort vergessen?"}}`
	sum := sha256.Sum256([]byte(pack))
	index := filepath.Join(dir, "index.json")
	for name, content := range map[string]string{"de.json": pack,
		"index.json": fmt.Sprintf(`{"languages": [{"code": "de", "version": "0.1.0", "file": "de.json", "sha256": %q, "strings": 3}]}`, hex.EncodeToString(sum[:]))} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bin := buildApp(t)

	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		// Not built in, not in the catalog: refused, the database left empty.
		refused := exec.Command(bin, "install", "-site-name", "X", "-admin-name", "admin", "-admin-email", "admin@example.com",
			"-language", "fr", "-translations", index)
		refused.Dir, refused.Env = repoRoot(t), siteEnv(t, db, freePort(t))
		if out, err := refused.CombinedOutput(); err == nil || !strings.Contains(string(out), `unknown -language "fr": not built in`) {
			t.Errorf("-language fr: %v\n%s", err, out)
		}
		noCatalog := exec.Command(bin, "install", "-site-name", "X", "-admin-name", "admin", "-admin-email", "admin@example.com", "-language", "de")
		noCatalog.Dir, noCatalog.Env = repoRoot(t), siteEnv(t, db, freePort(t))
		if out, err := noCatalog.CombinedOutput(); err == nil || !strings.Contains(string(out), `-language "de" isn't built in (en): give the translations catalog`) {
			t.Errorf("-language de without a catalog: %v\n%s", err, out)
		}
		// The bookkeeping table is made by the check for an installed
		// site; no set is in it.
		var rows int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&rows); err != nil || rows != 0 {
			t.Errorf("a refused install installed %d migrations (%v)", rows, err)
		}

		s := installSiteBin(t, db, bin, nil, "-language", "de", "-translations", index)
		if !strings.Contains(s.installed, "translations: de installed (3 strings)") {
			t.Errorf("app install:\n%s", s.installed)
		}
		var lang, article, page string
		if err := db.DB.QueryRow(`SELECT default_language FROM site_information`).Scan(&lang); err != nil || lang != "de" {
			t.Errorf("the site's language: %q (%v)", lang, err)
		}
		if err := db.DB.QueryRow(`SELECT label FROM content_types WHERE machine_name = 'article'`).Scan(&article); err != nil || article != "Artikel" {
			t.Errorf("the article type's name: %q (%v)", article, err)
		}
		if err := db.DB.QueryRow(`SELECT label FROM content_types WHERE machine_name = 'page'`).Scan(&page); err != nil || page != "Seite" {
			t.Errorf("the page type's name: %q (%v)", page, err)
		}
		if _, login := s.visitor().get("/login"); !strings.Contains(login, "Passwort vergessen?") {
			t.Error("the login page isn't in German")
		}
		s.checkLog()
	})
}
