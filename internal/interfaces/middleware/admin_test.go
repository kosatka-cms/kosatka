package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

type permTestUsers struct {
	entity.UserRepository
	users map[int]*entity.User
}

func (f permTestUsers) ByID(_ context.Context, id int) (*entity.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, entity.ErrEntityNotFound
}

type permTestAuth struct {
	service.AuthService
	granted map[int]string
}

func (f permTestAuth) HasPermission(_ context.Context, u *entity.User, p string) bool {
	return u != nil && f.granted[u.ID] == p
}

// PermissionMiddleware: 401 without a session user, 403 without the
// permission, the handler with it.
func TestPermissionMiddleware(t *testing.T) {
	users := permTestUsers{users: map[int]*entity.User{1: {ID: 1}, 2: {ID: 2}}}
	auth := permTestAuth{granted: map[int]string{1: service.PermAccessAdmin}}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := PermissionMiddleware(users, auth, service.PermAccessAdmin)(next)
	for _, tc := range []struct {
		name   string
		userID any
		want   int
	}{
		{"no session", nil, http.StatusUnauthorized},
		{"unknown user", 9, http.StatusUnauthorized},
		{"without the permission", 2, http.StatusForbidden},
		{"with the permission", 1, http.StatusTeapot},
	} {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		if tc.userID != nil {
			req = req.WithContext(context.WithValue(req.Context(), userIDKey, tc.userID))
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, rec.Code, tc.want)
		}
	}
}
