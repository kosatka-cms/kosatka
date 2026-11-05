package notfoundlog

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestReferrer - this site's page is shown as its path, another site's as
// its host and path, what isn't a web address as is.
func TestReferrer(t *testing.T) {
	for _, tc := range []struct {
		raw, link, label string
		own              bool
	}{
		{"https://example.com/news/?page=2", "/news/?page=2", "/news/?page=2", true},
		{"https://search.example.org/search/?q=spider", "https://search.example.org/search/?q=spider", "search.example.org/search/?q=spider", false},
		{"https://other.example.net/", "https://other.example.net/", "other.example.net", false},
		{"android-app://com.example", "", "android-app://com.example", false},
		{"", "", "", false},
	} {
		link, label, own := referrer(tc.raw, "example.com")
		if link != tc.link || label != tc.label || own != tc.own {
			t.Errorf("%q: %q %q %v", tc.raw, link, label, own)
		}
	}
}

// TestBack - an action returns to the list it came from, with its result
// in place of the last one; anything else returns to the log.
func TestBack(t *testing.T) {
	for back, want := range map[string]string{
		"/admin/config/redirects/not-found?q=news&status=all&done=ignored": "/admin/config/redirects/not-found?done=redirect&q=news&status=all",
		"/admin/config/redirects/not-found":                                "/admin/config/redirects/not-found?done=redirect",
		"https://evil.example/admin/config/redirects/not-found":            "/admin/config/redirects/not-found?done=redirect",
		"/admin/config/redirects/not-found-x":                              "/admin/config/redirects/not-found?done=redirect",
	} {
		r := httptest.NewRequest(http.MethodPost, "/admin/config/redirects/not-found/1/redirect", strings.NewReader(url.Values{"back": {back}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if got := backTo(r, "done", "redirect"); got != want {
			t.Errorf("%s: %s, want %s", back, got, want)
		}
	}
}
