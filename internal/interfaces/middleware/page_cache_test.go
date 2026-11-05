package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPageCacheMiddleware - a guest's 200 HTML page without cookies is
// shareable for maxAge; a signed-in visitor's page, a page setting a
// cookie, an error and a POST are private; non-HTML answers and answers
// with their own Cache-Control are left alone; maxAge 0 - off.
func TestPageCacheMiddleware(t *testing.T) {
	handler := func(status int, contentType string, cookie, cacheControl bool) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			if cookie {
				http.SetCookie(w, &http.Cookie{Name: "user-session", Value: "x"})
			}
			if cacheControl {
				w.Header().Set("Cache-Control", "no-cache")
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte("<!doctype html><p>page</p>"))
		})
	}
	const shared = "public, max-age=0, s-maxage=60"
	for _, c := range []struct {
		name        string
		method      string
		session     bool
		status      int
		contentType string
		setsCookie  bool
		ownCache    bool
		want        string
	}{
		{"guest page", http.MethodGet, false, 200, "text/html; charset=utf-8", false, false, shared},
		{"guest page, type sniffed", http.MethodGet, false, 200, "", false, false, shared},
		{"guest HEAD", http.MethodHead, false, 200, "text/html", false, false, shared},
		{"signed in", http.MethodGet, true, 200, "text/html", false, false, "private, no-store"},
		{"sets a cookie", http.MethodGet, false, 200, "text/html", true, false, "private, no-store"},
		{"not found", http.MethodGet, false, 404, "text/html", false, false, "private, no-store"},
		{"POST", http.MethodPost, false, 200, "text/html", false, false, "private, no-store"},
		{"JSON", http.MethodGet, false, 200, "application/json", false, false, ""},
		{"own Cache-Control", http.MethodGet, false, 200, "text/html", false, true, "no-cache"},
	} {
		req := httptest.NewRequest(c.method, "/x", nil)
		if c.session {
			req.AddCookie(&http.Cookie{Name: "user-session", Value: "s"})
		}
		rec := httptest.NewRecorder()
		PageCacheMiddleware(60, "user-session")(handler(c.status, c.contentType, c.setsCookie, c.ownCache)).ServeHTTP(rec, req)
		if got := rec.Header().Get("Cache-Control"); got != c.want {
			t.Errorf("%s: Cache-Control %q, want %q", c.name, got, c.want)
		}
	}
	rec := httptest.NewRecorder()
	PageCacheMiddleware(0, "user-session")(handler(200, "text/html", false, false)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("off: Cache-Control %q", got)
	}
}
