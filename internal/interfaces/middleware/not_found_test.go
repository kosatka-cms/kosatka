package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// TestNotFoundHooksMiddleware - a GET answered 404 reaches the hooks with
// its path and referrer; a page found, a POST answered 404 don't.
func TestNotFoundHooksMiddleware(t *testing.T) {
	t.Cleanup(service.ResetNotFoundHooks)
	service.ResetNotFoundHooks()
	var got []string
	service.OnNotFound(func(_ context.Context, path, referrer string) { got = append(got, path+" "+referrer) })
	h := NotFoundHooksMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/missing"}, {http.MethodGet, "/found"}, {http.MethodPost, "/missing"},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set("Referer", "https://example.org/")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if tc.path == "/missing" && w.Code != http.StatusNotFound {
			t.Errorf("%s %s answered %d", tc.method, tc.path, w.Code)
		}
	}
	if len(got) != 1 || got[0] != "/missing https://example.org/" {
		t.Errorf("hooks told: %q", got)
	}
}
