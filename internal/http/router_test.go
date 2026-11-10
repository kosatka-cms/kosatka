package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/i18ntest"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/interfaces/admin"
	"github.com/kosatka-cms/kosatka/pkg/i18n"

	actionlog "github.com/kosatka-cms/kosatka/internal/modules/action_log"
	"github.com/kosatka-cms/kosatka/internal/modules/complaints"
	errorreports "github.com/kosatka-cms/kosatka/internal/modules/error_reports"
	"github.com/kosatka-cms/kosatka/internal/modules/feeds"
	"github.com/kosatka-cms/kosatka/internal/modules/forms"
	"github.com/kosatka-cms/kosatka/internal/modules/galleries"
	"github.com/kosatka-cms/kosatka/internal/modules/languages"
	"github.com/kosatka-cms/kosatka/internal/modules/ratings"
	videoembed "github.com/kosatka-cms/kosatka/internal/modules/video_embed"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// TestRegisterAdminRoutes_NoPatternConflicts registers every admin route
// on a real *http.ServeMux. The mux (Go 1.22+) panics at startup when two
// patterns overlap without either being more specific (e.g. "{id}/delete"
// vs "menu/{id}" both match "menu/delete"); this catches it in go test.
// Registration never calls handlers, so zero-value controllers and nil
// dependencies are enough.
func TestRegisterAdminRoutes_NoPatternConflicts(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterAdminRoutes panicked (likely a route pattern conflict): %v", r)
		}
	}()

	mux := http.NewServeMux()
	// The site's catch-all (router.go): an admin pattern for every method
	// conflicts with it.
	mux.HandleFunc("GET /", func(http.ResponseWriter, *http.Request) {})
	ac := &admin.Controller{}
	dc := &interfaces.DynamicContentController{}
	RegisterAdminRoutes(mux, nil, ac, dc, &interfaces.StyleguideController{}, nil, nil)
}

// TestAdminMenuPathsAreRoutes - every link of the admin toolbar
// (entity.AdminMenu()) is a registered admin page.
func TestAdminMenuPathsAreRoutes(t *testing.T) {
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, nil, &admin.Controller{}, &interfaces.DynamicContentController{}, &interfaces.StyleguideController{}, nil, nil)
	// The recent changes register with the node routes.
	RegisterRecentChangesRoute(mux, nil, nil, nil, &interfaces.NodeController{})
	// Module pages register with their module's routes.
	forms.RegisterWebformRoutes(mux, nil, &forms.WebformController{}, nil)
	pass := func(h http.Handler) http.Handler { return h }
	errorreports.RegisterRoutes(mux, &errorreports.ErrorReportController{}, nil, pass)
	feeds.RegisterAdminRoutes(mux, &feeds.FeedController{}, pass)
	galleries.RegisterAdminRoutes(mux, &galleries.GalleryController{}, pass)
	ratings.RegisterAdminRoutes(mux, &ratings.Controller{}, pass)
	videoembed.RegisterRoutes(mux, nil, &videoembed.Controller{}, pass)
	actionlog.RegisterRoutes(mux, &actionlog.Controller{}, pass)
	languages.RegisterRoutes(mux, &languages.Controller{}, pass)
	complaints.RegisterRoutes(mux, &complaints.Controller{}, nil, pass, pass)
	var paths []string
	for _, section := range entity.AdminMenu() {
		if section.Path != "" {
			paths = append(paths, section.Path)
		}
		for _, item := range section.Items {
			paths = append(paths, item.Path)
		}
	}
	for _, p := range paths {
		_, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, p, nil))
		if pattern != "GET "+p {
			t.Errorf("%s: matched %q, not its own route", p, pattern)
		}
	}
}

// TestServeJSTranslations - /i18n/<lang>.js serves the dictionary and
// runtime for a known language, 404 otherwise.
func TestServeJSTranslations(t *testing.T) {
	tr, err := i18n.Load(i18ntest.FS)
	if err != nil {
		t.Fatal(err)
	}
	i18n.SetTranslator(tr)
	i18n.SetJSKeys([]i18n.JSKey{{Source: "Delete"}})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /i18n/{file}", serveJSTranslations)
	for _, c := range []struct {
		path string
		code int
		want string
	}{
		{"/i18n/ru.js", http.StatusOK, `"Delete":"Удалить"`},
		{"/i18n/en.js", http.StatusOK, `"lang":"en"`},
		{"/i18n/xx.js", http.StatusNotFound, ""},
		{"/i18n/ru.css", http.StatusNotFound, ""},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("%s: %d %.120s", c.path, rec.Code, rec.Body.String())
		}
		if c.code == http.StatusOK && !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") {
			t.Errorf("%s: content type %q", c.path, rec.Header().Get("Content-Type"))
		}
	}
}

// TestNodeSearchRequiresLogin - the node picker search answers 401 JSON
// without a session.
func TestNodeSearchRequiresLogin(t *testing.T) {
	mux := http.NewServeMux()
	RegisterNodeSearchRoutes(mux, sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef")), &interfaces.NodeSearchController{}, ratelimit.NewRateLimiter(60, 20))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nodes/search?q=x", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code %d: %s", rec.Code, rec.Body.String())
	}
}

// TestFormAccount - the per-account limiters' key: the first field set,
// lowercased, the same whichever field names the account.
func TestFormAccount(t *testing.T) {
	key := formAccount("login", "email")
	for form, want := range map[string]string{
		"login=Ann":                           "account:ann",
		"email=ann":                           "account:ann",
		"login=+&email=Ann%40example.com":     "account:ann@example.com",
		"pass=secret":                         "",
		"login=Bob&email=other%40example.com": "account:bob",
	} {
		r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if got := key(r); got != want {
			t.Errorf("%s: %q, want %q", form, got, want)
		}
	}
}
