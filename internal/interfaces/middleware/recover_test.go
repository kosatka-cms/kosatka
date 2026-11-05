package middleware

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRecoverMiddleware - a handler's panic is a 500 and a logged error
// with its stack; ErrAbortHandler still aborts.
func TestRecoverMiddleware(t *testing.T) {
	var logs bytes.Buffer
	mw := RecoverMiddleware(slog.New(slog.NewTextHandler(&logs, nil)))
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/me/notifications", nil))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(logs.String(), "boom") || !strings.Contains(logs.String(), "/me/notifications") {
		t.Fatalf("%d %s", rec.Code, logs.String())
	}
	defer func() {
		if err, _ := recover().(error); !errors.Is(err, http.ErrAbortHandler) {
			t.Errorf("ErrAbortHandler: %v", err)
		}
	}()
	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
