package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// ThemePreviewMiddleware: the session's theme renders only for a user
// who may change the theme; the others, and requests without one, keep
// the active theme.
func TestThemePreviewMiddleware(t *testing.T) {
	paths.SetTheme("kosatka")
	t.Cleanup(func() { paths.SetTheme("") })
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	users := permTestUsers{users: map[int]*entity.User{1: {ID: 1}, 2: {ID: 2}}}
	auth := permTestAuth{granted: map[int]string{1: service.PermAccessAdmin}}
	var previewed string
	var ok bool
	h := ThemePreviewMiddleware(store, users, auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		previewed, ok = paths.PreviewTheme(r.Context())
	}))
	// The engine's templates ("") are always available.
	sessionCookie := func(theme *string) *http.Cookie {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		session, _ := store.Get(req, "user-session")
		if theme != nil {
			session.Values[themes.PreviewSessionKey] = *theme
		}
		if err := session.Save(req, rec); err != nil {
			t.Fatal(err)
		}
		return rec.Result().Cookies()[0]
	}
	engine := ""
	for _, tc := range []struct {
		name   string
		userID any
		theme  *string
		want   bool
	}{
		{"an administrator previewing", 1, &engine, true},
		{"an administrator not previewing", 1, nil, false},
		{"a user without the permission", 2, &engine, false},
		{"a guest", nil, &engine, false},
	} {
		previewed, ok = "", false
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(sessionCookie(tc.theme))
		if tc.userID != nil {
			req = req.WithContext(context.WithValue(req.Context(), userIDKey, tc.userID))
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		if ok != tc.want || previewed != "" {
			t.Errorf("%s: previewed %q %v, want %v", tc.name, previewed, ok, tc.want)
		}
	}
}
