package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
)

// TestActorMiddleware - the request's context carries the signed-in user
// and the client's address as who acts.
func TestActorMiddleware(t *testing.T) {
	var got audit.Actor
	h := ActorMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = audit.ActorFrom(r.Context()) }))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.9:1234"
	r = r.WithContext(context.WithValue(r.Context(), userIDKey, 5))
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got != (audit.Actor{UserID: 5, IP: "203.0.113.9"}) {
		t.Errorf("actor: %+v", got)
	}
}

// TestAdminSaveAuditMiddleware - a settings page's POST redirected without
// an error is recorded, named by its admin menu item; an error, a page
// answered (not redirected), a GET, and a page with actions of its own
// (the Modules page) aren't.
func TestAdminSaveAuditMiddleware(t *testing.T) {
	audit.ResetListeners()
	t.Cleanup(audit.ResetListeners)
	var got []audit.Event
	audit.OnAction(func(_ context.Context, e audit.Event) { got = append(got, e) })
	answer := func(status int, location string) http.Handler {
		return AdminSaveAuditMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if location != "" {
				w.Header().Set("Location", location)
			}
			w.WriteHeader(status)
		}))
	}
	for _, tc := range []struct {
		method, path, location string
		status                 int
		recorded               bool
	}{
		{http.MethodPost, "/admin/config/text-formats/3", "/admin/config/text-formats/3/edit?success=1", http.StatusSeeOther, true},
		{http.MethodPost, "/admin/config/text-formats/3", "/admin/config/text-formats/3/edit?error=update", http.StatusSeeOther, false},
		{http.MethodPost, "/admin/performance/clear-thumbnails", "/admin/config/performance?perf=err", http.StatusSeeOther, false},
		{http.MethodPost, "/admin/config/text-formats/3", "", http.StatusOK, false},
		{http.MethodGet, "/admin/config/text-formats", "", http.StatusOK, false},
		{http.MethodPost, "/admin/config/modules", "/admin/config/modules?saved=1", http.StatusSeeOther, false},
		{http.MethodPost, "/admin/content/bulk-confirm", "/admin/content", http.StatusSeeOther, false},
	} {
		got = nil
		answer(tc.status, tc.location).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.path, nil))
		if (len(got) == 1) != tc.recorded {
			t.Errorf("%s %s → %d %s: recorded %v", tc.method, tc.path, tc.status, tc.location, got)
		}
	}
	answer(http.StatusSeeOther, "/admin/config/text-formats/3/edit?success=1").ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/config/text-formats/3", nil))
	if e := got[len(got)-1]; e.Type != audit.SettingsSave || e.Object.Label != "Text formats" || e.Object.URL != "/admin/config/text-formats" || e.Object.ID != "/admin/config/text-formats/3" {
		t.Errorf("event: %+v", e)
	}
}
