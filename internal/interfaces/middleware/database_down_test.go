package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestDatabaseDownMiddleware - while the database is up every request goes
// on; while it is gone a page answers 503 at once (titled by the site in
// memory, not cached, Retry-After), JSON for /api/ - the static files too.
func TestDatabaseDownMiddleware(t *testing.T) {
	down := false
	reached := 0
	site := &entity.SiteInformation{SiteName: "Harbour <news>"}
	h := DatabaseDownMiddleware(func() bool { return down }, func() *entity.SiteInformation { return site })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached++ }))
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	if rec := get("/"); rec.Code != http.StatusOK || reached != 1 {
		t.Fatalf("up: %d, reached %d", rec.Code, reached)
	}

	down = true
	rec := get("/news/harbour")
	body := rec.Body.String()
	if rec.Code != http.StatusServiceUnavailable || reached != 1 {
		t.Fatalf("down: %d, reached %d", rec.Code, reached)
	}
	if rec.Header().Get("Retry-After") != "30" || rec.Header().Get("Cache-Control") != "no-store" ||
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Errorf("headers %v", rec.Header())
	}
	for _, want := range []string{"<title>Harbour &lt;news&gt;</title>", "The site is temporarily unavailable. Please try again in a few minutes."} {
		if !strings.Contains(body, want) {
			t.Errorf("no %s in %s", want, body)
		}
	}

	rec = get("/api/node/view")
	var answer map[string]string
	if rec.Code != http.StatusServiceUnavailable || json.Unmarshal(rec.Body.Bytes(), &answer) != nil || answer["error"] == "" {
		t.Errorf("api: %d %s", rec.Code, rec.Body.String())
	}

	if rec := get("/static/css/site.css"); rec.Code != http.StatusServiceUnavailable || reached != 1 {
		t.Errorf("a static file while down: %d", rec.Code)
	}

	site = nil
	if rec := get("/"); rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "temporarily unavailable") {
		t.Errorf("without the site in memory: %d %s", rec.Code, rec.Body.String())
	}
}
