package ratelimit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRateLimitMiddlewareAllowsWithinBurst(t *testing.T) {
	rl := NewRateLimiter(60, 3)
	handler := RateLimitMiddleware(rl, func(r *http.Request) string { return "client-a" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	for i := range 3 {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 within burst, got %d", i+1, w.Code)
		}
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once burst is exhausted, got %d", w.Code)
	}
}

func TestRateLimitMiddlewareKeysIndependently(t *testing.T) {
	rl := NewRateLimiter(60, 1)
	handler := RateLimitMiddleware(rl, func(r *http.Request) string { return r.Header.Get("X-Client") })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	for _, client := range []string{"a", "b", "c"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Client", client)
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("client %q: expected first request to succeed, got %d", client, w.Code)
		}
	}

	// Second request for an already-used client should be limited (burst=1).
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Client", "a")
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected client 'a' second request to be limited, got %d", w.Code)
	}
}

// TestRateLimitRejection - over the limit, a script's request gets the
// message as JSON, a form's as text, both with Retry-After (the seconds
// until the next request: 60/20 - 3).
func TestRateLimitRejection(t *testing.T) {
	const msg = "Too many requests. Please wait a little and try again."
	for _, tc := range []struct {
		name, header, value, contentType, body string
	}{
		{"a script (Accept)", "Accept", "application/json", "application/json", `{"error":"` + msg + `"}` + "\n"},
		{"a script (Content-Type)", "Content-Type", "application/json", "application/json", `{"error":"` + msg + `"}` + "\n"},
		{"a form", "Content-Type", "application/x-www-form-urlencoded", "text/plain; charset=utf-8", msg + "\n"},
	} {
		rl := NewRateLimiter(20, 1)
		handler := RateLimitMiddleware(rl, func(*http.Request) string { return "client" })(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
		var w *httptest.ResponseRecorder
		for range 2 {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Header.Set(tc.header, tc.value)
			w = httptest.NewRecorder()
			handler.ServeHTTP(w, r)
		}
		if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") != "3" ||
			w.Header().Get("Content-Type") != tc.contentType || w.Body.String() != tc.body {
			t.Errorf("%s: %d, Retry-After %q, %q, %q", tc.name, w.Code, w.Header().Get("Retry-After"), w.Header().Get("Content-Type"), w.Body.String())
		}
	}
}

// TestIPv6Network - the addresses of one IPv6 /64 share a bucket; IPv4
// addresses and other keys don't.
func TestIPv6Network(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	if !rl.Allow("2001:db8:1:2::1") || rl.Allow("2001:db8:1:2:ffff::9") {
		t.Fatal("two addresses of one /64 got buckets of their own")
	}
	if !rl.Allow("2001:db8:1:3::1") {
		t.Fatal("another /64 shares the bucket")
	}
	if !rl.Allow("192.0.2.1") || !rl.Allow("192.0.2.2") || !rl.Allow("user:5") {
		t.Fatal("IPv4 addresses or plain keys grouped")
	}
}

// TestEvery - a burst, then one request an interval.
func TestEvery(t *testing.T) {
	rl := NewRateLimiterEvery(time.Hour, 2)
	var got []bool
	for range 3 {
		got = append(got, rl.Allow("a"))
	}
	if fmt.Sprint(got) != "[true true false]" {
		t.Fatalf("got %v, want a burst of 2", got)
	}
}

// TestEntriesBounded - the keys kept stay within maxEntries.
func TestEntriesBounded(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	for i := range maxEntries + 10 {
		rl.Allow(strconv.Itoa(i))
	}
	if len(rl.entries) > maxEntries {
		t.Fatalf("%d entries", len(rl.entries))
	}
}
