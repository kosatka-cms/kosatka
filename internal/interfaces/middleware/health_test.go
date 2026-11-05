package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealthMiddleware - /healthz answers by the database's ping, ahead of
// the handler; other requests reach it.
func TestHealthMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	for _, tc := range []struct {
		name, method, path string
		ping               error
		want               int
	}{
		{"up", http.MethodGet, HealthPath, nil, http.StatusOK},
		{"head", http.MethodHead, HealthPath, nil, http.StatusOK},
		{"database down", http.MethodGet, HealthPath, errors.New("refused"), http.StatusServiceUnavailable},
		{"another page", http.MethodGet, "/", nil, http.StatusTeapot},
		{"a post", http.MethodPost, HealthPath, nil, http.StatusTeapot},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := HealthMiddleware(func(context.Context) error { return tc.ping })(next)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.want {
				t.Errorf("%s %s: %d, want %d", tc.method, tc.path, rec.Code, tc.want)
			}
		})
	}
}
