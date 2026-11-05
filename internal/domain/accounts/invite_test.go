package accounts

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// fakeInviteUsers - the accounts an invitation creates, finds and saves.
type fakeInviteUsers struct {
	service.UserService
	byID map[int]*entity.User
}

func (f *fakeInviteUsers) FindByUsername(_ context.Context, name string) (*entity.User, error) {
	for _, u := range f.byID {
		if u.Username == name {
			return u, nil
		}
	}
	return nil, nil
}

func (f *fakeInviteUsers) Find(_ context.Context, filter entity.UserFilter) (*entity.User, error) {
	for _, u := range f.byID {
		if u.Email == filter.Email {
			return u, nil
		}
	}
	return nil, nil
}

func (f *fakeInviteUsers) Create(_ context.Context, u entity.User) (*entity.User, error) {
	u.ID = 100 + len(f.byID)
	f.byID[u.ID] = &u
	c := u
	return &c, nil
}

func (f *fakeInviteUsers) ByID(_ context.Context, id int) (*entity.User, error) {
	if u, ok := f.byID[id]; ok {
		c := *u
		return &c, nil
	}
	return nil, entity.ErrEntityNotFound
}

func (f *fakeInviteUsers) Edit(_ context.Context, u entity.User) (*entity.User, error) {
	c := u
	f.byID[u.ID] = &c
	return &u, nil
}

// TestInvite - an administrator invites a person: no password asked, the
// login made of the address when left blank (a number added when taken),
// the invitation mailed with its link, who invited and the role (not the
// welcome); the person accepts by the link once - their login and
// password saved, the address confirmed; what to correct leaves the link
// working; an account with a password isn't invited again.
func TestInvite(t *testing.T) {
	ctx := context.Background()
	users := &fakeInviteUsers{byID: map[int]*entity.User{1: {ID: 1, Username: "maria", Email: "old@example.com", PasswordHash: "x"}}}
	tokens := &fakeResetTokens{}
	mail := &fakeSignupMail{}
	s := NewRegistrationService(RegistrationDeps{Users: users, Site: fakeSignupSite{}, Account: fakeSignupAccount{},
		Tokens: NewUserAuthTokenService(tokens), Mail: mail, UoW: noTxUOW{}, Logger: slog.New(slog.DiscardHandler)})

	created, err := s.CreateAccount(ctx, NewAccount{Email: "Maria@Example.com", Invite: true, InvitedBy: "admin",
		Role: &entity.Role{ID: 3, Name: "editor", Description: "Editor"}, BaseURL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Username != "maria1" || created.PasswordHash != entity.PasswordUnset || !created.EmailVerified {
		t.Errorf("the invited account: %+v", created)
	}
	if len(mail.keys) != 1 || mail.keys[0] != MailInvite || mail.to[0] != "maria@example.com" {
		t.Fatalf("mailed %v to %v", mail.keys, mail.to)
	}
	extra := mail.data[0].Extra
	link := extra["[user:invite-url]"]
	raw, ok := strings.CutPrefix(link, "https://example.com/user/invite?token=")
	if !ok || raw == "" || extra["[invite:by]"] != "admin" || extra["[invite:role]"] != "Editor" || tokens.kind != entity.UserAuthTokenKindInvite {
		t.Fatalf("the invitation: %v (token kind %q)", extra, tokens.kind)
	}

	if u, err := s.InvitedUser(ctx, raw); err != nil || u.ID != created.ID {
		t.Errorf("the link's account: %v %v", u, err)
	}
	var toCorrect SignupInvalid
	if _, err := s.AcceptInvite(ctx, raw, "maria", "short", "short"); !errors.As(err, &toCorrect) || toCorrect["username"] == "" || toCorrect["password"] == "" {
		t.Errorf("a taken login and a short password: %v", err)
	}
	accepted, err := s.AcceptInvite(ctx, raw, "Maria P", "a long enough password", "a long enough password")
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Username != "Maria P" || accepted.Slug != entity.UserSlug("Maria P") || accepted.PasswordHash == "" || !accepted.EmailVerified {
		t.Errorf("accepted: %+v", accepted)
	}
	if _, err := s.AcceptInvite(ctx, raw, "Maria P", "a long enough password", "a long enough password"); !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("the link used again: %v", err)
	}
	if err := s.SendInvite(ctx, accepted, "https://example.com", "admin"); !errors.Is(err, ErrNotInvited) {
		t.Errorf("an account with a password invited: %v", err)
	}

	// A password typed now: no invitation, the welcome.
	mail.keys = nil
	if _, err := s.CreateAccount(ctx, NewAccount{Username: "bob", Email: "bob@example.com", Password: "a long enough password",
		PasswordConfirm: "a long enough password", Role: &entity.Role{ID: 3}, BaseURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if len(mail.keys) != 1 || mail.keys[0] != MailWelcomeAdmin {
		t.Errorf("with a password mailed %v", mail.keys)
	}
}
