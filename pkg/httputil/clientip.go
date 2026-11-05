package httputil

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
)

// defaultTrustedProxies - the peers trusted without a setting: loopback,
// a web server in front on the same host.
var defaultTrustedProxies = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
}

// trustedProxies - the peers whose X-Forwarded-For and X-Real-IP ClientIP
// believes (SetTrustedProxies).
var trustedProxies atomic.Pointer[[]netip.Prefix]

// SetTrustedProxies sets the proxies in front of the site (the config's
// trusted_proxies); none - loopback only.
func SetTrustedProxies(prefixes []netip.Prefix) {
	if len(prefixes) == 0 {
		prefixes = defaultTrustedProxies
	}
	trustedProxies.Store(&prefixes)
}

// ParseTrustedProxies - the proxies' addresses or networks ("10.0.0.5",
// "172.16.0.0/12"), for SetTrustedProxies.
func ParseTrustedProxies(values []string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if strings.Contains(v, "/") {
			p, err := netip.ParsePrefix(v)
			if err != nil {
				return nil, fmt.Errorf("trusted proxy %q: %w", v, err)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", v, err)
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

func trusted(a netip.Addr) bool {
	prefixes := defaultTrustedProxies
	if p := trustedProxies.Load(); p != nil {
		prefixes = *p
	}
	for _, p := range prefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// ClientIP - the visitor's address. The connection's peer, unless it is a
// trusted proxy (SetTrustedProxies): then the nearest address of
// X-Forwarded-For that isn't one, read from the right (the entries on the
// left are the client's to write), else X-Real-IP. A client talking to the
// application directly can't set its address this way.
func ClientIP(r *http.Request) string {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	if !fromTrustedProxy(r) {
		return peer
	}
	if entries := forwardedFor(r); len(entries) > 0 {
		for i := len(entries) - 1; i >= 0; i-- {
			a, err := netip.ParseAddr(entries[i])
			if err != nil {
				// Not an address: what's left of it isn't to be believed.
				break
			}
			if a = a.Unmap(); !trusted(a) || i == 0 {
				return a.String()
			}
		}
		return peer
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		if a, err := netip.ParseAddr(xri); err == nil {
			return a.Unmap().String()
		}
	}
	return peer
}

// forwardedFor - the X-Forwarded-For entries of every such header, in
// order.
func forwardedFor(r *http.Request) []string {
	var out []string
	for _, h := range r.Header.Values("X-Forwarded-For") {
		for _, e := range strings.Split(h, ",") {
			if e = strings.TrimSpace(e); e != "" {
				out = append(out, e)
			}
		}
	}
	return out
}

// fromTrustedProxy - whether the connection's peer is a trusted proxy.
func fromTrustedProxy(r *http.Request) bool {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(peer)
	return err == nil && trusted(addr.Unmap())
}

// IsHTTPS - whether the visitor came over HTTPS: the connection's TLS, or
// X-Forwarded-Proto from a trusted proxy (a client talking to the
// application directly can't claim it).
func IsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return fromTrustedProxy(r) && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}
