package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// redirectsOf answers GetRedirect from a map.
type redirectsOf struct {
	service.RedirectService
	to map[string]string
}

func (r redirectsOf) GetRedirect(path string) (string, bool) {
	to, ok := r.to[path]
	return to, ok
}

// TestRedirectMiddleware - a configured path redirects; anything else,
// the front page's query string included, goes on to the page (no
// redirect to an address taken from the request).
func TestRedirectMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := RedirectMiddleware(redirectsOf{to: map[string]string{"old": "/new"}})(next)
	for target, want := range map[string]string{
		"/old":                  "/new",
		"/?q=old":               "",
		"/?q=%5Cevil.example":   "",
		"/?q=%09/evil.example":  "",
		"/?q=//evil.example":    "",
		"/articles?q=something": "",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if got := rec.Header().Get("Location"); got != want {
			t.Errorf("%s: Location %q, want %q", target, got, want)
		}
		if want == "" && rec.Code != http.StatusTeapot {
			t.Errorf("%s: %d, want the page", target, rec.Code)
		}
	}
}
