package interfaces

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPublicBaseURL - links in mail and feeds: the site URL when set;
// else the request's host with the scheme a trusted proxy saw
// (X-Forwarded-Proto), else the connection's.
func TestPublicBaseURL(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://example.com/foo", nil)
	r.Host = "example.com"
	if got := publicBaseURL(r); got != "http://example.com" {
		t.Errorf("no TLS: %q", got)
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := publicBaseURL(r); got != "http://example.com" {
		t.Errorf("a client claiming HTTPS: %q", got)
	}
	r.RemoteAddr = "127.0.0.1:4321"
	if got := publicBaseURL(r); got != "https://example.com" {
		t.Errorf("behind a TLS proxy: %q", got)
	}
	r2 := httptest.NewRequest(http.MethodGet, "https://example.com/foo", nil)
	r2.Host = "example.com"
	r2.TLS = &tls.ConnectionState{}
	if got := publicBaseURL(r2); got != "https://example.com" {
		t.Errorf("TLS: %q", got)
	}
	withSiteDomain(t, "https://www.example.org")
	if got := publicBaseURL(r); got != "https://www.example.org" {
		t.Errorf("site URL: %q", got)
	}
}
