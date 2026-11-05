package csrf

import (
	"crypto/subtle"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/pkg/randtoken"
)

const (
	sessionKey  = "csrf_token"
	sessionName = SessionName
)

// SessionName - the session cookie the token lives in (the site's session).
const SessionName = "user-session"

func generateToken() string {
	token, err := randtoken.Generate(32)
	if err != nil {
		panic("csrf: failed to generate random token: " + err.Error())
	}
	return token
}

// EnsureToken returns the CSRF token from the session, generating a new one if needed.
// If a new token was generated, the session is saved to persist it in the cookie.
func EnsureToken(store *sessions.CookieStore, w http.ResponseWriter, r *http.Request) string {
	session, _ := store.Get(r, sessionName)
	if token, ok := session.Values[sessionKey].(string); ok && token != "" {
		return token
	}
	token := generateToken()
	session.Values[sessionKey] = token
	_ = session.Save(r, w)
	return token
}

// Token reads the CSRF token from the session without generating a new one.
func Token(store *sessions.CookieStore, r *http.Request) string {
	session, _ := store.Get(r, sessionName)
	token, _ := session.Values[sessionKey].(string)
	return token
}

// Validate checks if the provided form token matches the session token using constant-time comparison.
func Validate(store *sessions.CookieStore, r *http.Request) bool {
	expected := Token(store, r)
	formToken := r.FormValue("csrf_token")
	if expected == "" || formToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(formToken)) == 1
}

// ValidateSubmittedToken checks a CSRF token from JSON or other non-form sources.
func ValidateSubmittedToken(store *sessions.CookieStore, r *http.Request, formToken string) bool {
	expected := Token(store, r)
	if expected == "" || formToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(formToken)) == 1
}

// HeaderName - the request header scripts send the token in (forms send
// the csrf_token field).
const HeaderName = "X-CSRF-Token"

// ValidateRequest checks the token sent with a state-changing request: the
// X-CSRF-Token header (fetch/XHR) or the csrf_token form field.
func ValidateRequest(store *sessions.CookieStore, r *http.Request) bool {
	if t := r.Header.Get(HeaderName); t != "" {
		return ValidateSubmittedToken(store, r, t)
	}
	return Validate(store, r)
}
