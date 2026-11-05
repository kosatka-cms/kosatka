package privatemessages

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"

	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// TestRoutesOffWhenDisabled - the module's pages want a signed-in user
// (a guest is sent to log in) and answer 404 while the module is off.
func TestRoutesOffWhenDisabled(t *testing.T) {
	mux := http.NewServeMux()
	notFound := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	RegisterRoutes(testkit.ModuleMux(mux, manifest.Name, notFound), store, testController(&fakeThreads{}), ratelimit.NewRateLimiter(10, 3))
	get := func(path string) int {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec.Code
	}
	for _, path := range []string{"/me/messages/new", "/me/messages/thread/5", "/user/messages"} {
		if code := get(path); code != http.StatusFound && code != http.StatusSeeOther {
			t.Errorf("a guest on %s: %d, want the login redirect", path, code)
		}
	}
	withModule(t, false)
	for _, path := range []string{"/me/messages/new", "/me/messages/thread/5", "/user/messages"} {
		if code := get(path); code != http.StatusNotFound {
			t.Errorf("the module off, %s: %d, want 404", path, code)
		}
	}
}
