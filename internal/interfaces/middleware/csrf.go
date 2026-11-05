package middleware

import (
	"errors"
	"mime"
	"net/http"
	"slices"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
)

// CSRFMiddleware rejects every state-changing request (POST, PUT, PATCH,
// DELETE) that doesn't carry the session's CSRF token: the csrf_token form
// field or the X-CSRF-Token header. Reading a page creates no session for
// a visitor without one (no cookie: a guest's page can be cached by a
// CDN); the token is created when a page's script asks for it (GET
// /api/csrf-token, csrf_head) or a controller puts it in a form. A visitor
// with a session gets the token on any request, as before. csrfExemptPaths take no token. A form
// body past the request limit (BodyLimitMiddleware) can't be checked: it
// gets tooLarge (nil - a plain 413).
func CSRFMiddleware(store *sessions.CookieStore, tooLarge http.Handler) func(http.Handler) http.Handler {
	if tooLarge == nil {
		tooLarge = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
		})
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if safeMethod(r.Method) {
				// A session gets its token as before (its pages are never
				// shared); a visitor without one gets none: no cookie.
				if _, err := r.Cookie(csrf.SessionName); err == nil {
					csrf.EnsureToken(store, w, r)
				}
				next.ServeHTTP(w, r)
				return
			}
			if csrfExempt(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if !safeMethod(r.Method) && r.Header.Get(csrf.HeaderName) == "" {
				if err := parseFormBody(r); err != nil && overLimit(r, err) {
					tooLarge.ServeHTTP(w, r)
					return
				}
			}
			if !safeMethod(r.Method) && !csrf.ValidateRequest(store, r) {
				expiredForm(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// csrfExemptPaths - POST endpoints that take no token: a node's view
// count (sent from every page, a cached guest page included; it only
// counts).
var csrfExemptPaths = []string{"/api/node/view"}

func csrfExempt(path string) bool {
	return slices.Contains(csrfExemptPaths, path)
}

// formMemory - the part of a multipart body kept in memory; the rest of
// the files go to temporary files (as r.FormValue does).
const formMemory = 32 << 20

// parseFormBody parses a form body (multipart or urlencoded) for the token
// check; the handlers then find it parsed.
func parseFormBody(r *http.Request) error {
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct == "multipart/form-data" {
		return r.ParseMultipartForm(formMemory)
	}
	return r.ParseForm()
}

// overLimit - whether a failed parse ran into the body limit: the error
// says so, or (cut inside a part's headers, the parser reports a bad
// header) the body read past it keeps failing with it.
func overLimit(r *http.Request, err error) bool {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return true
	}
	if r.Body == nil {
		return false
	}
	_, err = r.Body.Read(make([]byte, 1))
	return errors.As(err, &tooBig)
}

func safeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}
