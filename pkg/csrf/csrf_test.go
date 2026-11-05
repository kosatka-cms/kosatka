package csrf

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
)

func newRequestWithCookies(cookies []*http.Cookie) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	return r
}

func TestEnsureTokenPersistsAcrossRequests(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))

	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	token1 := EnsureToken(store, w1, r1)
	if token1 == "" {
		t.Fatal("expected non-empty token")
	}

	r2 := newRequestWithCookies(w1.Result().Cookies())
	token2 := Token(store, r2)
	if token2 != token1 {
		t.Fatalf("expected token to persist: got %q, want %q", token2, token1)
	}

	w2 := httptest.NewRecorder()
	token3 := EnsureToken(store, w2, r2)
	if token3 != token1 {
		t.Fatalf("expected EnsureToken to reuse existing token: got %q, want %q", token3, token1)
	}
}

func TestTokenEmptyWithoutSession(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := Token(store, r); got != "" {
		t.Fatalf("expected empty token for fresh session, got %q", got)
	}
}

func TestValidate(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	token := EnsureToken(store, w, r)
	cookies := w.Result().Cookies()

	form := url.Values{"csrf_token": {token}}
	valid := newRequestWithCookies(cookies)
	valid.Method = http.MethodPost
	valid.PostForm = form
	valid.Body = http.NoBody
	if !Validate(store, valid) {
		t.Fatal("expected Validate to succeed with matching token")
	}

	wrongForm := url.Values{"csrf_token": {"not-the-token"}}
	invalid := newRequestWithCookies(cookies)
	invalid.Method = http.MethodPost
	invalid.PostForm = wrongForm
	invalid.Body = http.NoBody
	if Validate(store, invalid) {
		t.Fatal("expected Validate to fail with mismatched token")
	}

	noCookie := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("csrf_token="+token))
	noCookie.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if Validate(store, noCookie) {
		t.Fatal("expected Validate to fail without a session")
	}
}

func TestValidateSubmittedToken(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	token := EnsureToken(store, w, r)
	cookies := w.Result().Cookies()

	valid := newRequestWithCookies(cookies)
	if !ValidateSubmittedToken(store, valid, token) {
		t.Fatal("expected matching token to validate")
	}
	if ValidateSubmittedToken(store, valid, "wrong") {
		t.Fatal("expected mismatched token to fail")
	}
	if ValidateSubmittedToken(store, valid, "") {
		t.Fatal("expected empty submitted token to fail")
	}
}

// TestValidateRequest - the token is accepted from the X-CSRF-Token header
// or the csrf_token field; a wrong header isn't rescued by the field.
func TestValidateRequest(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	w := httptest.NewRecorder()
	token := EnsureToken(store, w, httptest.NewRequest(http.MethodGet, "/", nil))
	cookies := w.Result().Cookies()
	req := func(header, field string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{"csrf_token": {field}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if header != "" {
			r.Header.Set(HeaderName, header)
		}
		for _, c := range cookies {
			r.AddCookie(c)
		}
		return r
	}
	for _, c := range []struct {
		name, header, field string
		want                bool
	}{
		{"header", token, "", true},
		{"field", "", token, true},
		{"none", "", "", false},
		{"wrong header", "nope", token, false},
	} {
		if got := ValidateRequest(store, req(c.header, c.field)); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
