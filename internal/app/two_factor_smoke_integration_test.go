//go:build dbintegration

package app

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/twofactor"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// authAddr - the sign-in forms' posts come each from an address of its
// own: their rate limit (by address) would refuse a test's many.
var authAddr int

// postRead posts form and returns the answer's code, Location and body.
func (s *site) postRead(path string, form url.Values) (int, string, string) {
	s.t.Helper()
	req, err := http.NewRequest(http.MethodPost, s.base+path, strings.NewReader(form.Encode()))
	if err != nil {
		s.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authAddr++
	req.Header.Set("X-Forwarded-For", "198.51.100."+strconv.Itoa(authAddr%250+1))
	resp, err := s.client.Do(req)
	if err != nil {
		s.t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Location"), string(b)
}

// enableTwoFactor turns two-factor sign-in on for the signed-in
// administrator (the key on the page, a code from it): the recovery
// codes shown once.
func enableTwoFactor(t *testing.T, s *site) [][]string {
	t.Helper()
	code, page := s.get("/me/two-factor")
	key := regexp.MustCompile(`<code class="two-factor-key">([A-Z2-7 ]+)</code>`).FindStringSubmatch(page)
	if code != http.StatusOK || key == nil || !strings.Contains(page, "<svg") {
		t.Fatalf("the setup page: %d\n%s", code, page)
	}
	secret := strings.ReplaceAll(key[1], " ", "")
	status, _, body := s.postRead("/me/two-factor/enable", url.Values{"csrf_token": {s.csrf("/me/two-factor")}, "code": {twofactor.Code(secret, twofactor.Step(time.Now()))}})
	codes := regexp.MustCompile(`<li><code>([a-z0-9]{4}-[a-z0-9]{4})</code></li>`).FindAllStringSubmatch(body, -1)
	if status != http.StatusOK || len(codes) != twofactor.RecoveryCodes {
		t.Fatalf("turning on: %d, %d codes\n%s", status, len(codes), body)
	}
	return codes
}

// twoFactorPasswordStep signs the administrator out and in again with the
// password: the sign-in waits for the code.
func twoFactorPasswordStep(t *testing.T, s *site) {
	t.Helper()
	_ = s.post("/logout", url.Values{"csrf_token": {s.csrf("/me")}})
	c, where, _ := s.postRead("/login", url.Values{"login": {"admin"}, "pass": {s.password}, "csrf_token": {s.csrf("/login")}})
	if c != http.StatusFound || !strings.HasPrefix(where, "/login/code") {
		t.Fatalf("the password alone: %d → %q", c, where)
	}
	if c, _ := s.get("/admin"); c == http.StatusOK {
		t.Fatal("signed in before the code")
	}
}

// TestTwoFactor - the administrator turns two-factor sign-in on (the key
// on the page, a code from it, the recovery codes shown once); then
// the password alone doesn't sign in: the code page asks, a wrong code is
// refused, a recovery code signs in. A role needing it: its user without
// it is sent to turn it on.
func TestTwoFactor(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		codes := enableTwoFactor(t, s)
		signInStep := func() { twoFactorPasswordStep(t, s) }
		signInStep()
		if c, _, body := s.postRead("/login/code", url.Values{"csrf_token": {s.csrf("/login/code")}, "code": {"000000"}}); c != http.StatusOK || !strings.Contains(body, "The code is wrong") {
			t.Fatalf("a wrong code: %d\n%s", c, body)
		}
		if c, where, _ := s.postRead("/login/code", url.Values{"csrf_token": {s.csrf("/login/code")}, "code": {codes[0][1]}}); c != http.StatusFound || where != "/me" {
			t.Fatalf("a recovery code: %d → %q", c, where)
		}
		if c, _ := s.get("/admin"); c != http.StatusOK {
			t.Fatalf("signed in with the code: /admin %d", c)
		}

		// Signed in again (a recovery code), the administrator's role is
		// made to need it; without it (reset) every page sends them to
		// turn it on.
		signInStep()
		if c, _, _ := s.postRead("/login/code", url.Values{"csrf_token": {s.csrf("/login/code")}, "code": {codes[1][1]}}); c != http.StatusFound {
			t.Fatalf("a second recovery code: %d", c)
		}
		var role int
		if err := db.DB.QueryRow(`SELECT role_id FROM users WHERE username = 'admin'`).Scan(&role); err != nil {
			t.Fatal(err)
		}
		if c, _, _ := s.postRead("/admin/config/account", url.Values{"csrf_token": {s.csrf("/admin/config/account")}, "registration_mode": {"visitors"},
			"two_factor_roles": {strconv.Itoa(role)}}); c != http.StatusSeeOther {
			t.Fatalf("saving the roles: %d", c)
		}
		// The reset of an administrator of users (their own, here).
		var adminID int
		if err := db.DB.QueryRow(`SELECT id FROM users WHERE username = 'admin'`).Scan(&adminID); err != nil {
			t.Fatal(err)
		}
		if c, where, _ := s.postRead(fmt.Sprintf("/admin/users/%d/two-factor/reset", adminID), url.Values{"csrf_token": {s.csrf("/admin/users")}}); c != http.StatusSeeOther || !strings.Contains(where, "two_factor_reset=1") {
			t.Fatalf("the reset: %d → %q", c, where)
		}
		var left int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM user_two_factor`).Scan(&left); err != nil || left != 0 {
			t.Fatalf("after the reset: %d %v", left, err)
		}
		resp, err := s.client.Get(s.base + "/admin")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/me/two-factor" {
			t.Errorf("a required role without it: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
		}
		if c, page := s.get("/me/two-factor"); c != http.StatusOK || !strings.Contains(page, "Your role requires two-factor sign-in") {
			t.Errorf("its page: %d", c)
		}
	})
}

// TestTwoFactorTries - five wrong codes end the sign-in, and the account
// takes no more codes for a while, a right one included: signing in with
// the password again doesn't start the count over.
func TestTwoFactorTries(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		codes := enableTwoFactor(t, s)
		twoFactorPasswordStep(t, s)
		var where string
		for i := 0; i < 5; i++ {
			_, where, _ = s.postRead("/login/code", url.Values{"csrf_token": {s.csrf("/login/code")}, "code": {"000000"}})
		}
		if where != "/login?msg=second_factor" {
			t.Fatalf("five wrong codes: → %q", where)
		}
		twoFactorPasswordStep(t, s)
		if _, where, _ := s.postRead("/login/code", url.Values{"csrf_token": {s.csrf("/login/code")}, "code": {codes[0][1]}}); where != "/login?msg=second_factor" {
			t.Fatalf("a right code after the tries ran out: → %q", where)
		}
		if c, _ := s.get("/admin"); c == http.StatusOK {
			t.Fatal("signed in after the tries ran out")
		}
	})
}
