package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
)

// TestCSRFMiddleware - state-changing requests need the session's token
// (form field or header); safe methods pass.
func TestCSRFMiddleware(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	h := CSRFMiddleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	// The session with a token (as /api/csrf-token issues it).
	rec := httptest.NewRecorder()
	csrf.EnsureToken(store, rec, httptest.NewRequest(http.MethodGet, "/", nil))
	cookies := rec.Result().Cookies()
	tokenReq := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		tokenReq.AddCookie(c)
	}
	token := csrf.Token(store, tokenReq)
	if token == "" {
		t.Fatal("no token issued")
	}

	post := func(body, header string) int {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if header != "" {
			req.Header.Set(csrf.HeaderName, header)
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, c := range []struct {
		name, body, header string
		want               int
	}{
		{"no token", "a=1", "", http.StatusForbidden},
		{"wrong form token", url.Values{"csrf_token": {"nope"}}.Encode(), "", http.StatusForbidden},
		{"form token", url.Values{"csrf_token": {token}}.Encode(), "", http.StatusNoContent},
		{"header token", "", token, http.StatusNoContent},
		{"wrong header token", url.Values{"csrf_token": {token}}.Encode(), "nope", http.StatusForbidden},
	} {
		if got := post(c.body, c.header); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}

	// No session at all: rejected.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/x", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("DELETE without session: %d", rec.Code)
	}
}

// TestCSRFMiddlewareTooLarge - a form body past the request limit gets the
// too-large handler, not a CSRF failure; one within it is checked as usual.
// TestCSRFMiddlewareSessions - reading a page starts no session for a
// visitor without one (no cookie: the page can be shared by a CDN); a
// session without a token gets one; the view count takes a POST without
// a token; other POSTs without one are rejected.
func TestCSRFMiddlewareSessions(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	h := CSRFMiddleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, path := range []string{"/", "/news/x", "/robots.txt"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if len(rec.Result().Cookies()) > 0 {
			t.Errorf("GET %s without a session set a cookie", path)
		}
	}

	// A session (signed in, say) without a token gets one on a read.
	sess := httptest.NewRecorder()
	session, _ := store.Get(httptest.NewRequest(http.MethodGet, "/", nil), csrf.SessionName)
	session.Values["user_id"] = 7
	_ = session.Save(httptest.NewRequest(http.MethodGet, "/", nil), sess)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range sess.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	tokenReq := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		tokenReq.AddCookie(c)
	}
	if csrf.Token(store, tokenReq) == "" {
		t.Error("a session got no token")
	}

	for path, want := range map[string]int{"/api/node/view": http.StatusOK, "/rss.xml": http.StatusForbidden, "/comment/create": http.StatusForbidden} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		if rec.Code != want {
			t.Errorf("POST %s without a token: %d, want %d", path, rec.Code, want)
		}
	}
}

func TestCSRFMiddlewareTooLarge(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	tooLarge := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusRequestEntityTooLarge) })
	h := BodyLimitMiddleware(64)(CSRFMiddleware(store, tooLarge)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	post := func(contentType, body string) int {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	big := "--b\r\nContent-Disposition: form-data; name=\"f\"; filename=\"a.txt\"\r\n\r\n" + strings.Repeat("a", 200) + "\r\n--b--\r\n"
	if code := post("multipart/form-data; boundary=b", big); code != http.StatusRequestEntityTooLarge {
		t.Errorf("multipart: %d, want 413", code)
	}
	if code := post("application/x-www-form-urlencoded", "a="+strings.Repeat("a", 200)); code != http.StatusRequestEntityTooLarge {
		t.Errorf("urlencoded: %d, want 413", code)
	}
	if code := post("application/x-www-form-urlencoded", "a=1"); code != http.StatusForbidden {
		t.Errorf("small without a token: %d, want 403", code)
	}
}
