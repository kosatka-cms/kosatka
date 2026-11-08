package accounts

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/randtoken"
	"golang.org/x/crypto/bcrypt"
)

// fakeResetTokens - one password reset token, used once.
type fakeResetTokens struct {
	hash    string
	userID  int
	kind    string
	expires time.Time
	used    bool
	issued  int
}

func (f *fakeResetTokens) InvalidateUserKind(context.Context, int, string) error { return nil }

func (f *fakeResetTokens) Insert(_ context.Context, userID int, hash, kind string, expires time.Time) error {
	f.hash, f.userID, f.kind, f.expires, f.used = hash, userID, kind, expires, false
	f.issued++
	return nil
}

func (f *fakeResetTokens) Consume(_ context.Context, hash, _ string) (int, bool, error) {
	if f.used || hash != f.hash {
		return 0, false, nil
	}
	f.used = true
	return f.userID, true, nil
}

func (f *fakeResetTokens) Valid(_ context.Context, hash, _ string) (int, bool, error) {
	if f.used || hash != f.hash || !f.expires.After(time.Now()) {
		return 0, false, nil
	}
	return f.userID, true, nil
}

func (f *fakeResetTokens) Pending(_ context.Context, kind string, userIDs []int) (map[int]time.Time, error) {
	out := map[int]time.Time{}
	if !f.used && f.kind == kind && slices.Contains(userIDs, f.userID) {
		out[f.userID] = f.expires
	}
	return out, nil
}

// fakeResetUsers - the accounts a reset finds and saves.
type fakeResetUsers struct {
	service.UserService
	byEmail map[string]*entity.User
	saved   []entity.User
	editErr error
	// ended - the accounts whose sessions were ended.
	ended []int
}

func (f *fakeResetUsers) BumpSessionVersion(_ context.Context, id int) (int, error) {
	f.ended = append(f.ended, id)
	return len(f.ended), nil
}

func (f *fakeResetUsers) Find(_ context.Context, filter entity.UserFilter) (*entity.User, error) {
	if u, ok := f.byEmail[filter.Email]; ok {
		return u, nil
	}
	return nil, entity.ErrEntityNotFound
}

func (f *fakeResetUsers) ByID(_ context.Context, id int) (*entity.User, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			c := *u
			return &c, nil
		}
	}
	return nil, entity.ErrEntityNotFound
}

func (f *fakeResetUsers) Edit(_ context.Context, u entity.User) (*entity.User, error) {
	if f.editErr != nil {
		return nil, f.editErr
	}
	f.saved = append(f.saved, u)
	return &u, nil
}

// TestPasswordReset - the link mailed to an active account only (and
// only with the site URL); a new password set by it once, in the action
// log with the account's user as who did it; a password to correct
// leaves the link unused.
func TestPasswordReset(t *testing.T) {
	ctx := t.Context()
	audit.ResetListeners()
	t.Cleanup(audit.ResetListeners)
	var events []audit.Event
	audit.OnAction(func(_ context.Context, e audit.Event) { events = append(events, e) })
	setup := func() (*PasswordResetService, *fakeResetTokens, *fakeResetUsers, *fakeSignupMail) {
		tokens := &fakeResetTokens{}
		users := &fakeResetUsers{byEmail: map[string]*entity.User{
			"ishmael@example.com": {ID: 1, Email: "ishmael@example.com", Status: entity.UserStatusActive},
			"ahab@example.com":    {ID: 2, Email: "ahab@example.com", Status: entity.UserStatusBlocked},
		}}
		mail := &fakeSignupMail{}
		return NewPasswordResetService(PasswordResetDeps{Users: users, Tokens: NewUserAuthTokenService(tokens), Site: fakeSignupSite{},
			Account: fakeSignupAccount{}, Mail: mail, UoW: noTxUOW{}, Logger: slog.New(slog.DiscardHandler)}), tokens, users, mail
	}

	s, tokens, _, mail := setup()
	for _, email := range []string{"ahab@example.com", "nobody@example.com"} {
		s.Request(ctx, email, "https://example.com")
	}
	s.Request(ctx, "ishmael@example.com", "")
	if tokens.issued != 0 || len(mail.to) != 0 {
		t.Fatalf("blocked, unknown or without the site URL: %d tokens, mailed %v", tokens.issued, mail.to)
	}
	s.Request(ctx, "ishmael@example.com", "https://example.com")
	if tokens.issued != 1 || len(mail.to) != 1 || mail.to[0] != "ishmael@example.com" {
		t.Fatalf("the active account: %d tokens, mailed %v", tokens.issued, mail.to)
	}

	// The raw token isn't kept: issue one whose secret the test knows.
	s, tokens, users, _ := setup()
	raw := "the-raw-secret"
	tokens.hash, tokens.userID = randtoken.Hash(raw), 1
	if err := s.Reset(ctx, raw, "short", "short"); !errors.Is(err, ErrResetPasswordInvalid) || tokens.used {
		t.Fatalf("a short password: %v, token used %v", err, tokens.used)
	}
	if err := s.Reset(ctx, "another", "harpoon1", "harpoon1"); !errors.Is(err, ErrResetLinkInvalid) {
		t.Fatalf("an unknown token: %v", err)
	}
	if err := s.Reset(ctx, raw, "harpoon1", "harpoon1"); err != nil {
		t.Fatal(err)
	}
	if len(users.saved) != 1 || bcrypt.CompareHashAndPassword([]byte(users.saved[0].PasswordHash), []byte("harpoon1")) != nil {
		t.Fatalf("saved %+v", users.saved)
	}
	// The link reached the address: it is confirmed.
	if !users.saved[0].EmailVerified {
		t.Error("the address not confirmed by the reset")
	}
	if len(users.ended) != 1 || users.ended[0] != 1 {
		t.Fatalf("the account's sessions not ended: %v", users.ended)
	}
	if len(events) != 1 || events[0].Type != audit.UserPasswordReset || events[0].Actor.UserID != 1 || events[0].Object.ID != "1" {
		t.Fatalf("the action log: %+v", events)
	}
	if err := s.Reset(ctx, raw, "harpoon2", "harpoon2"); !errors.Is(err, ErrResetLinkInvalid) {
		t.Fatalf("the link used again: %v", err)
	}
}

// noTxUOW runs the unit of work's function without a transaction.
type noTxUOW struct{}

func (noTxUOW) Do(ctx context.Context, f func(context.Context) error) error { return f(ctx) }
