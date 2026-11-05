package ratelimit

import (
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

type rateLimitEntry struct {
	tokens    float64
	lastCheck time.Time
}

// maxEntries - the keys a limiter keeps at most: when full, the keys
// whose bucket has refilled go (they are as good as new), and all of
// them when none has.
const maxEntries = 100_000

// RateLimiter is a token bucket per key (a client's address, a user, an
// account's name). An IPv6 address counts as its /64 network: one host
// is given a whole one, and would get a new bucket per address.
type RateLimiter struct {
	mu        sync.Mutex
	entries   map[string]*rateLimitEntry
	rate      float64 // tokens per second
	burst     int     // max tokens (bucket size)
	lastClean time.Time
}

// NewRateLimiter allows burst requests at once, then requestsPerMinute.
func NewRateLimiter(requestsPerMinute int, burst int) *RateLimiter {
	return newRateLimiter(float64(requestsPerMinute)/60.0, burst)
}

// NewRateLimiterEvery allows burst requests at once, then one every
// interval (for rare actions: an hour, a day).
func NewRateLimiterEvery(interval time.Duration, burst int) *RateLimiter {
	return newRateLimiter(1/interval.Seconds(), burst)
}

func newRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]*rateLimitEntry),
		rate:    rate,
		burst:   burst,
	}
}

// Allow takes a request of key from its bucket: false when it is empty.
// For a key a middleware can't tell (one from the form, after a check).
func (rl *RateLimiter) Allow(key string) bool {
	return rl.allow(key)
}

// bucketKey - key, or the /64 network of an IPv6 address.
func bucketKey(key string) string {
	if a, err := netip.ParseAddr(key); err == nil && a.Is6() && !a.Is4In6() {
		if p, err := a.Prefix(64); err == nil {
			return p.String()
		}
	}
	return key
}

// full - whether e's bucket has refilled by now.
func (rl *RateLimiter) full(e *rateLimitEntry, now time.Time) bool {
	return e.tokens+now.Sub(e.lastCheck).Seconds()*rl.rate >= float64(rl.burst)
}

// dropFull forgets the keys whose bucket has refilled.
func (rl *RateLimiter) dropFull(now time.Time) {
	for k, e := range rl.entries {
		if rl.full(e, now) {
			delete(rl.entries, k)
		}
	}
}

func (rl *RateLimiter) allow(key string) bool {
	key = bucketKey(key)
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastClean) > 5*time.Minute {
		rl.dropFull(now)
		rl.lastClean = now
	}

	e, exists := rl.entries[key]
	if !exists {
		if len(rl.entries) >= maxEntries {
			rl.dropFull(now)
			if len(rl.entries) >= maxEntries {
				clear(rl.entries)
			}
		}
		rl.entries[key] = &rateLimitEntry{
			tokens:    float64(rl.burst) - 1,
			lastCheck: now,
		}
		return true
	}

	elapsed := now.Sub(e.lastCheck).Seconds()
	e.lastCheck = now
	e.tokens += elapsed * rl.rate
	if e.tokens > float64(rl.burst) {
		e.tokens = float64(rl.burst)
	}

	if e.tokens < 1 {
		return false
	}
	e.tokens--
	return true
}

// RateLimitMiddleware returns middleware that limits requests, keyed by keyFunc (e.g. client IP).
func RateLimitMiddleware(rl *RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(keyFunc(r)) {
				rl.Reject(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Reject answers a request over the limit: 429, Retry-After (the seconds
// until a request is allowed again) and a message in the request's
// language - {"error": …} to a script (wantsJSON), the text to a form.
func (rl *RateLimiter) Reject(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(1/rl.rate))))
	msg := i18n.T(r.Context(), "Too many requests. Please wait a little and try again.")
	if wantsJSON(r) {
		httputil.JSONError(w, msg, http.StatusTooManyRequests)
		return
	}
	http.Error(w, msg, http.StatusTooManyRequests)
}

// wantsJSON - the request is a script's: it sends or accepts JSON, or is
// marked as one (X-Requested-With).
func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest"
}
