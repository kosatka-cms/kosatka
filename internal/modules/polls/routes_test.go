package polls

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"

	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// TestPollVoteRateLimit - both vote endpoints (the form and the API) share
// the limiter, counted per client before sign-in is checked: past the
// burst a client gets 429, another client is still let through.
func TestPollVoteRateLimit(t *testing.T) {
	mux := http.NewServeMux()
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	RegisterPollVoteRoutes(mux, store, &PollController{}, ratelimit.NewRateLimiter(1, 2))
	vote := func(path, ip string) int {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		r.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}
	for i, path := range []string{"/node/5/poll/vote", "/api/node/5/poll/vote"} {
		if code := vote(path, "192.0.2.1"); code == http.StatusTooManyRequests {
			t.Fatalf("request %d within the burst: 429", i+1)
		}
	}
	if code := vote("/node/5/poll/vote", "192.0.2.1"); code != http.StatusTooManyRequests {
		t.Errorf("past the burst: %d, want 429", code)
	}
	if code := vote("/api/node/5/poll/vote", "192.0.2.1"); code != http.StatusTooManyRequests {
		t.Errorf("API past the burst: %d, want 429", code)
	}
	if code := vote("/node/5/poll/vote", "192.0.2.2"); code == http.StatusTooManyRequests {
		t.Error("another client must not be limited")
	}
}
