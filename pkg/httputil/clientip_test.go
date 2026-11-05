package httputil

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestClientIP - the forwarding headers count only from a trusted proxy
// (loopback by default), read from the right past the trusted hops; a
// client reaching the application directly gets its own address.
func TestClientIP(t *testing.T) {
	t.Cleanup(func() { SetTrustedProxies(nil) })
	req := func(remote string, headers ...string) string {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		for i := 0; i+1 < len(headers); i += 2 {
			r.Header.Add(headers[i], headers[i+1])
		}
		return ClientIP(r)
	}

	SetTrustedProxies(nil)
	for _, tc := range []struct {
		name, want, remote string
		headers            []string
	}{
		{"direct, no headers", "203.0.113.7", "203.0.113.7:5000", nil},
		{"direct, a forged header", "203.0.113.7", "203.0.113.7:5000", []string{"X-Forwarded-For", "198.51.100.1"}},
		{"direct, a forged X-Real-IP", "203.0.113.7", "203.0.113.7:5000", []string{"X-Real-IP", "198.51.100.1"}},
		{"behind nginx", "198.51.100.1", "127.0.0.1:41000", []string{"X-Forwarded-For", "198.51.100.1"}},
		{"behind nginx, a forged entry on the left", "198.51.100.1", "127.0.0.1:41000", []string{"X-Forwarded-For", "10.9.9.9, 198.51.100.1"}},
		{"behind nginx over IPv6", "2001:db8::1", "[::1]:41000", []string{"X-Forwarded-For", "2001:db8::1"}},
		{"two headers", "198.51.100.1", "127.0.0.1:41000", []string{"X-Forwarded-For", "10.9.9.9", "X-Forwarded-For", "198.51.100.1"}},
		{"X-Real-IP from nginx", "198.51.100.1", "127.0.0.1:41000", []string{"X-Real-IP", "198.51.100.1"}},
		{"garbage", "127.0.0.1", "127.0.0.1:41000", []string{"X-Forwarded-For", "not-an-ip"}},
		{"no headers from nginx", "127.0.0.1", "127.0.0.1:41000", nil},
	} {
		if got := req(tc.remote, tc.headers...); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}

	// A proxy on another host (a CDN, a load balancer) is trusted when
	// set; then loopback isn't.
	prefixes, err := ParseTrustedProxies([]string{"10.0.0.0/8", " 192.0.2.10 "})
	if err != nil {
		t.Fatal(err)
	}
	SetTrustedProxies(prefixes)
	if got := req("10.1.2.3:80", "X-Forwarded-For", "198.51.100.1, 192.0.2.10"); got != "198.51.100.1" {
		t.Errorf("past two trusted hops: %q", got)
	}
	if got := req("127.0.0.1:80", "X-Forwarded-For", "198.51.100.1"); got != "127.0.0.1" {
		t.Errorf("loopback no longer trusted: %q", got)
	}
	if got := req("10.1.2.3:80", "X-Forwarded-For", "10.0.0.1, 10.0.0.2"); got != "10.0.0.1" {
		t.Errorf("all hops trusted - the leftmost: %q", got)
	}
	if _, err := ParseTrustedProxies([]string{"10.0.0.0/33"}); err == nil {
		t.Error("a bad network accepted")
	}
}

// TestIsHTTPS - TLS, or X-Forwarded-Proto from a trusted proxy only.
func TestIsHTTPS(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("X-Forwarded-Proto", "https")
	if IsHTTPS(r) {
		t.Error("a client's own X-Forwarded-Proto believed")
	}
	r.RemoteAddr = "127.0.0.1:1234"
	if !IsHTTPS(r) {
		t.Error("the proxy's X-Forwarded-Proto ignored")
	}
	r.Header.Set("X-Forwarded-Proto", "http")
	if IsHTTPS(r) {
		t.Error("plain HTTP behind the proxy")
	}
	r.TLS = &tls.ConnectionState{}
	if !IsHTTPS(r) {
		t.Error("a TLS connection")
	}
}
