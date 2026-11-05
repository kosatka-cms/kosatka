package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// oneUserService knows one user.
type oneUserService struct {
	UserService
	user entity.User
}

func (s oneUserService) ByID(_ context.Context, id int) (*entity.User, error) {
	if id != s.user.ID {
		return nil, entity.ErrEntityNotFound
	}
	u := s.user
	return &u, nil
}

// codeSecondFactor takes one code.
type codeSecondFactor struct{ code string }

func (codeSecondFactor) Enabled(context.Context, int) (bool, error) { return true, nil }

func (f codeSecondFactor) Verify(_ context.Context, _ int, code string) (bool, error) {
	return code == f.code, nil
}

// TestSecondFactorTriesKeptOnServer - wrong codes are counted per account
// on the server: sending the pending sign-in's first cookie again doesn't
// give more tries, and once they're spent even the right code is refused.
func TestSecondFactorTriesKeptOnServer(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	s := NewAuthService(oneUserService{user: entity.User{ID: 7, Status: entity.UserStatusActive}}, *store, nil).(*DefaultAuthService)
	s.SetSecondFactor(codeSecondFactor{code: "123456"})

	// The pending sign-in's cookie, kept to be sent again each time.
	rec := httptest.NewRecorder()
	if err := s.savePending(httptest.NewRequest(http.MethodGet, "/", nil), rec, 7); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()
	try := func(code string) error {
		req := httptest.NewRequest(http.MethodPost, "/login/code", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		_, err := s.CompleteSecondFactor(context.Background(), httptest.NewRecorder(), req, code)
		return err
	}
	for i := 1; i < secondFactorTries; i++ {
		if err := try("000000"); !errors.Is(err, errSecondFactorWrong) {
			t.Fatalf("wrong code %d: %v", i, err)
		}
	}
	if err := try("000000"); !errors.Is(err, errSecondFactorGone) {
		t.Fatalf("the last wrong code: %v", err)
	}
	if err := try("123456"); !errors.Is(err, errSecondFactorGone) {
		t.Fatalf("the right code with the first cookie after the tries ran out: %v", err)
	}
}
