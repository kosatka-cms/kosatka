package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
)

// TestErrorPages - with the site's pages set, an expired form gets the
// page (a script, which sent the token header, JSON), a missing node the
// 404 page and a request without a signed-in user the 401 page.
func TestErrorPages(t *testing.T) {
	var got []string
	SetErrorPages(ErrorPages{
		Forbidden: func(w http.ResponseWriter, _ *http.Request) {
			got = append(got, "403")
			w.WriteHeader(http.StatusForbidden)
		},
		NotFound: func(w http.ResponseWriter, _ *http.Request) {
			got = append(got, "404")
			w.WriteHeader(http.StatusNotFound)
		},
		Rejected: func(w http.ResponseWriter, _ *http.Request, status int, message string) {
			got = append(got, message)
			w.WriteHeader(status)
		},
		ExpiredForm: func(*http.Request) string { return "expired" },
	})
	t.Cleanup(func() { SetErrorPages(ErrorPages{}) })

	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	h := CSRFMiddleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	post := func(header string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("a=1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if header != "" {
			req.Header.Set(csrf.HeaderName, header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := post(""); rec.Code != http.StatusForbidden || len(got) != 1 || got[0] != "expired" {
		t.Errorf("form: %d %v", rec.Code, got)
	}
	got = nil
	if rec := post("stale"); rec.Code != http.StatusForbidden || len(got) != 0 || !strings.Contains(rec.Body.String(), `"error":"expired"`) {
		t.Errorf("script: %d %q %v", rec.Code, rec.Body.String(), got)
	}

	got = nil
	rec := httptest.NewRecorder()
	unauthorized(rec, httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(context.Background()))
	if rec.Code != http.StatusUnauthorized || len(got) != 1 {
		t.Errorf("unauthorized: %d %v", rec.Code, got)
	}
	got = nil
	rec = httptest.NewRecorder()
	notFoundPage(rec, httptest.NewRequest(http.MethodGet, "/node/x/edit", nil))
	if rec.Code != http.StatusNotFound || len(got) != 1 || got[0] != "404" {
		t.Errorf("not found: %d %v", rec.Code, got)
	}
}
