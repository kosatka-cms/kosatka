package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// TestModuleMux - a disabled module's routes answer 404 (pages through
// notFound, API paths with JSON); an enabled module's routes work.
func TestModuleMux(t *testing.T) {
	t.Cleanup(func() { service.SetModules(map[string]bool{"example": true}) })
	mux := http.NewServeMux()
	notFound := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("site 404"))
	}
	gated := ModuleMux(mux, "example", notFound)
	ok := func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }
	gated.HandleFunc("GET /example", ok)
	gated.Handle("GET /api/example/random", http.HandlerFunc(ok))

	serve := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	for _, path := range []string{"/example", "/api/example/random"} {
		if rec := serve(path); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
			t.Errorf("enabled %s: %d %q", path, rec.Code, rec.Body.String())
		}
	}

	service.SetModules(map[string]bool{"example": false})
	if rec := serve("/example"); rec.Code != http.StatusNotFound || rec.Body.String() != "site 404" {
		t.Errorf("disabled page: %d %q", rec.Code, rec.Body.String())
	}
	rec := serve("/api/example/random")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Header().Get("Content-Type"), "json") {
		t.Errorf("disabled API: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}
