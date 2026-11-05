package middleware

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

type statusRepoStub struct {
	entity.UserRepository
	status  int
	version int
	err     error
}

func (s statusRepoStub) SessionStateByID(context.Context, int) (int, int, error) {
	return s.status, s.version, s.err
}

// loggedInRequest - a request with uid's session, carrying version 1.
func loggedInRequest(t *testing.T, store *sessions.CookieStore, uid int) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/forum", nil)
	rec := httptest.NewRecorder()
	sess, _ := store.Get(req, "user-session")
	sess.Values["user_id"] = uid
	sess.Values[service.SessionVersionKey] = 1
	if err := sess.Save(req, rec); err != nil {
		t.Fatal(err)
	}
	out := httptest.NewRequest(http.MethodPost, "/forum/new-topic/x", nil)
	for _, c := range rec.Result().Cookies() {
		out.AddCookie(c)
	}
	return out
}

func TestInactiveUserSessionMiddleware(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cases := []struct {
		name       string
		repo       statusRepoStub
		wantUser   bool
		wantCookie bool
	}{
		{"active", statusRepoStub{status: entity.UserStatusActive, version: 1}, true, false},
		{"sessions ended", statusRepoStub{status: entity.UserStatusActive, version: 2}, false, true},
		{"blocked", statusRepoStub{status: entity.UserStatusBlocked, version: 1}, false, true},
		{"pending", statusRepoStub{status: entity.UserStatusPending, version: 1}, false, true},
		{"deleted", statusRepoStub{err: entity.ErrEntityNotFound}, false, true},
		{"db error fails open", statusRepoStub{err: errors.New("db down")}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
			var sawUser bool
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Downstream code reads the session directly (like AuthService.GetUserID).
				sess, _ := store.Get(r, "user-session")
				uid, _ := sess.Values["user_id"].(int)
				sawUser = uid != 0
			})
			h := InactiveUserSessionMiddleware(store, tc.repo, log)(OptionalAuthMiddleware(store)(inner))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, loggedInRequest(t, store, 43005))

			if sawUser != tc.wantUser {
				t.Fatalf("downstream saw user=%v, want %v", sawUser, tc.wantUser)
			}
			if gotCookie := len(rec.Result().Cookies()) > 0; gotCookie != tc.wantCookie {
				t.Fatalf("response cookie rewritten=%v, want %v", gotCookie, tc.wantCookie)
			}
		})
	}
}
