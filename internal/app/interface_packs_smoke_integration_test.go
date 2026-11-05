//go:build dbintegration

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestInterfacePacks - a site with a translations catalog (a local
// index.json, TRANSLATIONS_CATALOG_URL): the Languages tab offers its
// German; downloaded, German is a language of the site, chosen in Site
// information a guest sees the login page in it; the site's language
// can't be removed; English again, it is removed and the page is English.
func TestInterfacePacks(t *testing.T) {
	dir := t.TempDir()
	pack := `{"strings": {"Forgot your password?": "Passwort vergessen?", "Create an account": "Konto erstellen", "Not a string of the site": "Nein"}}`
	sum := sha256.Sum256([]byte(pack))
	index := fmt.Sprintf(`{"languages": [{"code": "de", "version": "0.1.0", "file": "de.json", "sha256": %q, "strings": 2, "draft": true}]}`, hex.EncodeToString(sum[:]))
	for name, content := range map[string]string{"de.json": pack, "index.json": index} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TRANSLATIONS_CATALOG_URL", filepath.Join(dir, "index.json"))

	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		t.Cleanup(func() { i18n.SetDefault(i18n.SourceLanguage) })
		s := serveSite(t, db)
		s.signIn()
		const page = "/admin/config/translations"
		if code, body := s.get(page); code != http.StatusOK || !strings.Contains(body, `<input type="hidden" name="lang" value="de">`) || !strings.Contains(body, "Deutsch") {
			t.Fatalf("%s: %d, no German to download", page, code)
		}
		resp := s.post(page+"/download", url.Values{"csrf_token": {s.csrf(page)}, "lang": {"de"}})
		if loc := resp.Header.Get("Location"); resp.StatusCode != http.StatusSeeOther || loc != page+"?done=downloaded&dropped=1&kept=2&lang=de" {
			t.Fatalf("downloading: %d → %s\n%s", resp.StatusCode, loc, s.log.String())
		}

		setLanguage := func(lang string) {
			t.Helper()
			const info = "/admin/config/system/site-information"
			if _, body := s.get(info); !strings.Contains(body, `<option value="de"`) {
				t.Fatal("Site information doesn't offer German")
			}
			var name, slogan, email, feed, tz string
			if err := db.DB.QueryRow(`SELECT site_name, slogan, site_email, front_feed_view, COALESCE(timezone, '') FROM site_information`).Scan(&name, &slogan, &email, &feed, &tz); err != nil {
				t.Fatal(err)
			}
			if tz == "" {
				tz = "UTC"
			}
			resp := s.postMultipart(info, url.Values{"csrf_token": {s.csrf(info)}, "site_name": {name}, "slogan": {slogan}, "site_email": {email},
				"front_feed_view": {feed}, "timezone": {tz}, "default_language": {lang}})
			if !strings.Contains(resp.Header.Get("Location"), "saved=1") {
				t.Fatalf("saving Site information: %s\n%s", resp.Header.Get("Location"), s.log.String())
			}
		}
		setLanguage("de")
		guest := s.visitor()
		if _, login := guest.get("/login"); !strings.Contains(login, "Passwort vergessen?") || !strings.Contains(login, "Konto erstellen") {
			t.Error("the login page isn't in German")
		}
		resp = s.post(page+"/delete", url.Values{"csrf_token": {s.csrf(page)}, "lang": {"de"}})
		if loc := resp.Header.Get("Location"); loc != page+"?error=in_use&lang=de" {
			t.Errorf("removing the site's language: %s", loc)
		}

		setLanguage("en")
		resp = s.post(page+"/delete", url.Values{"csrf_token": {s.csrf(page)}, "lang": {"de"}})
		if loc := resp.Header.Get("Location"); loc != page+"?done=deleted&lang=de" {
			t.Fatalf("removing: %s\n%s", loc, s.log.String())
		}
		if _, login := guest.get("/login"); !strings.Contains(login, "Forgot your password?") {
			t.Error("the login page isn't English again")
		}
		var n int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM interface_strings`).Scan(&n); err != nil || n != 0 {
			t.Errorf("interface_strings: %d rows (%v)", n, err)
		}
		s.checkLog()
	})
}
