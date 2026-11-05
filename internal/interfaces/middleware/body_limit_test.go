package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestBodyLimitMiddleware - a body over the limit can't be read past it.
func TestBodyLimitMiddleware(t *testing.T) {
	var read int
	var readErr error
	h := BodyLimitMiddleware(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		read, readErr = len(b), err
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100))))
	if readErr == nil || read > 10 {
		t.Errorf("read %d, err %v", read, readErr)
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader("small")))
	if readErr != nil || read != 5 {
		t.Errorf("small body: %d %v", read, readErr)
	}
}
