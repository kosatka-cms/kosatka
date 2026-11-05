//go:build dbintegration

package app

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestInterfaceStrings - Interface translations: the languages tab, a
// string found on the strings tab; its English rewritten there is what a
// guest sees at once; a translation dropping the source's HTML for its
// own is refused (the tab again, the string open with why, nothing
// changed); set back to the default - the code's text again.
func TestInterfaceStrings(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, false)
		s.signIn()
		const page, list = "/admin/config/translations", "/admin/config/translations/strings"
		if code, body := s.get(page); code != http.StatusOK || !strings.Contains(body, "English") {
			t.Fatalf("%s: %d", page, code)
		}
		code, body := s.get(list + "?lang=en&q=forgot+your")
		if code != http.StatusOK || !strings.Contains(body, `<input type="hidden" name="source" value="Forgot your password?">`) {
			t.Fatalf("%s: %d, no Forgot your password?", list, code)
		}

		form := func(text string) url.Values {
			return url.Values{"csrf_token": {s.csrf(list)}, "lang": {"en"}, "q": {"forgot your"}, "source": {"Forgot your password?"}, "text": {text}}
		}
		resp := s.post(list, form("Lost the password?"))
		if loc := resp.Header.Get("Location"); resp.StatusCode != http.StatusSeeOther || !strings.Contains(loc, "saved=1") || !strings.Contains(loc, "q=forgot+your") {
			t.Fatalf("saving: %d → %s\n%s", resp.StatusCode, loc, s.log.String())
		}
		guest := s.visitor()
		if _, login := guest.get("/login"); !strings.Contains(login, "Lost the password?") || strings.Contains(login, "Forgot your password?") {
			t.Error("the login page hasn't the site's text")
		}
		if _, body := s.get(list + "?lang=en&show=edited"); !strings.Contains(body, `<span class="admin-strings-text">Lost the password?</span>`) {
			t.Error("the edited strings haven't it")
		}

		refused, err := s.client.PostForm(s.base+list, form(`<b>Lost</b> the password?`))
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(refused.Body)
		_ = refused.Body.Close()
		if refused.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(b), "The translation can have only the tags of the English string, as they are: &lt;b&gt; isn&#39;t one.") ||
			!strings.Contains(string(b), `<details class="admin-strings-edit" open>`) {
			t.Errorf("a refused translation: %d", refused.StatusCode)
		}
		if _, login := guest.get("/login"); !strings.Contains(login, "Lost the password?") {
			t.Error("a refused translation changed the string")
		}

		resp = s.post(list+"/reset", url.Values{"csrf_token": {s.csrf(list)}, "lang": {"en"}, "source": {"Forgot your password?"}})
		if loc := resp.Header.Get("Location"); resp.StatusCode != http.StatusSeeOther || !strings.Contains(loc, "reset=1") {
			t.Fatalf("resetting: %d → %s", resp.StatusCode, loc)
		}
		if _, login := guest.get("/login"); !strings.Contains(login, "Forgot your password?") {
			t.Error("the login page hasn't the code's text back")
		}
		var n int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM interface_strings`).Scan(&n); err != nil || n != 0 {
			t.Errorf("interface_strings: %d rows (%v)", n, err)
		}
		s.checkLog()
	})
}
