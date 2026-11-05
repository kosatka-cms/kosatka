package accounts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
	"golang.org/x/crypto/bcrypt"
)

func TestMailTokenData(t *testing.T) {
	site := &entity.SiteInformation{SiteName: "Example Site", Slogan: "Slogan"}
	user := &entity.User{Username: "ishmael", Email: "ishmael@example.com"}

	d := MailTokenData("https://example.com/", site, user, "conf-token", "one-time-raw", "reset-raw")

	if d.SiteName != "Example Site" || d.SiteSlogan != "Slogan" {
		t.Errorf("site fields not copied: %+v", d)
	}
	if d.UserName != "ishmael" || d.UserEmail != "ishmael@example.com" {
		t.Errorf("user fields not copied: %+v", d)
	}
	if d.SiteLoginURL != "https://example.com/login" {
		t.Errorf("SiteLoginURL = %q, want trailing slash trimmed then /login appended", d.SiteLoginURL)
	}
	if d.ConfirmEmailURL != "https://example.com/user/confirm-email?token=conf-token" {
		t.Errorf("ConfirmEmailURL = %q", d.ConfirmEmailURL)
	}
	if d.OneTimeLoginURL != "https://example.com/user/login-once?token=one-time-raw" {
		t.Errorf("OneTimeLoginURL = %q", d.OneTimeLoginURL)
	}
	if d.ResetPasswordURL != "https://example.com/user/reset-password?token=reset-raw" {
		t.Errorf("ResetPasswordURL = %q", d.ResetPasswordURL)
	}
}

func TestMailTokenDataEmptyTokensOmitURLs(t *testing.T) {
	site := &entity.SiteInformation{SiteName: "Example Site"}
	user := &entity.User{Username: "ishmael", Email: "ishmael@example.com"}

	d := MailTokenData("https://example.com", site, user, "", "", "")

	if d.ConfirmEmailURL != "" {
		t.Errorf("ConfirmEmailURL should stay empty, got %q", d.ConfirmEmailURL)
	}
	if d.OneTimeLoginURL != "" {
		t.Errorf("OneTimeLoginURL should stay empty, got %q", d.OneTimeLoginURL)
	}
	if d.ResetPasswordURL != "" {
		t.Errorf("ResetPasswordURL should stay empty, got %q", d.ResetPasswordURL)
	}
}

// fakeSignupUsers - the accounts the sign-up checks and creates.
type fakeSignupUsers struct {
	service.UserService
	existing map[string]*entity.User // by username and by email
	created  []entity.User
}

func (f *fakeSignupUsers) FindByUsername(_ context.Context, username string) (*entity.User, error) {
	return f.existing[username], nil
}

func (f *fakeSignupUsers) Find(_ context.Context, filter entity.UserFilter) (*entity.User, error) {
	return f.existing[filter.Email], nil
}

func (f *fakeSignupUsers) Create(_ context.Context, u entity.User) (*entity.User, error) {
	u.ID = 100 + len(f.created)
	f.created = append(f.created, u)
	return &u, nil
}

type fakeSignupRoles struct{ entity.RoleRepository }

func (fakeSignupRoles) ByName(_ context.Context, name string) (*entity.Role, error) {
	return &entity.Role{ID: 2, Name: name}, nil
}

type fakeSignupSite struct {
	entity.SiteInformationRepository
}

func (fakeSignupSite) Get(context.Context) (*entity.SiteInformation, error) {
	return &entity.SiteInformation{SiteName: "Example", SiteEmail: "admin@example.com"}, nil
}

type fakeSignupAccount struct {
	entity.AccountSettingsRepository
	acc entity.AccountSettings
}

func (f fakeSignupAccount) Get(context.Context) (*entity.AccountSettings, error) {
	acc := f.acc
	return &acc, nil
}

type fakeSignupMail struct {
	to, keys []string
	data     []mailtemplate.TokenData
}

func (f *fakeSignupMail) Send(_ context.Context, key, to string, data mailtemplate.TokenData) error {
	f.to = append(f.to, to)
	f.keys = append(f.keys, key)
	f.data = append(f.data, data)
	return nil
}

// TestRegister - the account a visitor's sign-up creates, by the site's
// registration mode: active and welcomed; awaiting its email's
// confirmation (the link valid a day); awaiting approval, the site's
// email told; none when accounts are the administrators' to create, or
// the form has something to correct.
func TestRegister(t *testing.T) {
	signup := Signup{Username: "Ishmael", Email: " Ishmael@Example.com ", Password: "whale123", PasswordConfirm: "whale123",
		Timezone: "Europe/Berlin", IP: "192.0.2.1", BaseURL: "https://example.com"}
	// tokens - the last sign-up's one-time tokens.
	var tokens *fakeResetTokens
	register := func(t *testing.T, acc entity.AccountSettings, in Signup) (*fakeSignupUsers, *fakeSignupMail, *entity.User, SignupOutcome, error) {
		users := &fakeSignupUsers{existing: map[string]*entity.User{"starbuck": {ID: 1}, "starbuck@example.com": {ID: 1}}}
		mail := &fakeSignupMail{}
		tokens = &fakeResetTokens{}
		s := NewRegistrationService(RegistrationDeps{Users: users, Roles: fakeSignupRoles{}, Site: fakeSignupSite{},
			Account: fakeSignupAccount{acc: acc}, Tokens: NewUserAuthTokenService(tokens), Mail: mail, Logger: slog.New(slog.DiscardHandler)})
		u, outcome, err := s.Register(t.Context(), in)
		return users, mail, u, outcome, err
	}

	t.Run("visitors", func(t *testing.T) {
		users, mail, u, outcome, err := register(t, entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitors}, signup)
		if err != nil || outcome != SignupActive {
			t.Fatalf("outcome %v, err %v", outcome, err)
		}
		if u.Email != "ishmael@example.com" || u.Slug != entity.UserSlug("Ishmael") || u.Status != entity.UserStatusActive || !u.EmailVerified ||
			u.Role.Name != entity.RoleUser || u.Timezone != "Europe/Berlin" || u.Metadata[entity.UserMetadataKeyRegistrationIP] != "192.0.2.1" {
			t.Errorf("account = %+v", u)
		}
		if bcrypt.CompareHashAndPassword([]byte(users.created[0].PasswordHash), []byte("whale123")) != nil {
			t.Error("the password is not the hash's")
		}
		if fmt.Sprint(mail.to) != "[ishmael@example.com]" {
			t.Errorf("emails to %v", mail.to)
		}
	})

	t.Run("visitors with email confirmation", func(t *testing.T) {
		_, mail, u, outcome, err := register(t, entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitors, RequireEmailVerification: true}, signup)
		if err != nil || outcome != SignupConfirmEmail {
			t.Fatalf("outcome %v, err %v", outcome, err)
		}
		if u.EmailVerified || u.Status != entity.UserStatusActive {
			t.Errorf("account = %+v", u)
		}
		// The link's token is kept hashed, as the other one-time links.
		if tokens.issued != 1 || tokens.kind != entity.UserAuthTokenKindEmailConfirm || tokens.userID != u.ID ||
			time.Until(tokens.expires).Round(time.Hour) != emailConfirmationTTL {
			t.Errorf("the confirmation token: %+v", tokens)
		}
		if fmt.Sprint(mail.to) != "[ishmael@example.com]" {
			t.Errorf("emails to %v", mail.to)
		}
	})

	t.Run("visitors with approval", func(t *testing.T) {
		_, mail, u, outcome, err := register(t, entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitorsApproval}, signup)
		if err != nil || outcome != SignupPendingApproval {
			t.Fatalf("outcome %v, err %v", outcome, err)
		}
		if u.Status != entity.UserStatusPending || u.EmailVerified {
			t.Errorf("account = %+v", u)
		}
		if fmt.Sprint(mail.to) != "[ishmael@example.com admin@example.com]" {
			t.Errorf("emails to %v", mail.to)
		}
	})

	t.Run("administrators only", func(t *testing.T) {
		users, _, _, _, err := register(t, entity.AccountSettings{RegistrationMode: entity.RegistrationModeAdminOnly}, signup)
		if !errors.Is(err, ErrRegistrationClosed) || len(users.created) != 0 {
			t.Fatalf("err %v, created %v", err, users.created)
		}
	})

	t.Run("what to correct", func(t *testing.T) {
		in := signup
		in.Username, in.Email, in.PasswordConfirm = "starbuck", "Starbuck@example.com", "other"
		users, _, _, _, err := register(t, entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitors}, in)
		var invalid SignupInvalid
		if !errors.As(err, &invalid) || len(users.created) != 0 {
			t.Fatalf("err %v, created %v", err, users.created)
		}
		for _, field := range []string{"username", "email", "password_confirm"} {
			if invalid[field] == "" {
				t.Errorf("no message for %s: %v", field, invalid)
			}
		}
		if _, ok := invalid["password"]; ok {
			t.Errorf("the password is fine: %v", invalid)
		}
	})

	t.Run("a blocked email domain", func(t *testing.T) {
		_, _, _, _, err := register(t, entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitors, RegistrationBlockedDomains: "example.com"}, signup)
		var invalid SignupInvalid
		if !errors.As(err, &invalid) || invalid["email"] == "" {
			t.Fatalf("err %v", err)
		}
	})
}

// Without the site URL the e-mails aren't sent: their links would point
// at the request's host.
func TestRegisterWithoutSiteURL(t *testing.T) {
	users, mail := &fakeSignupUsers{existing: map[string]*entity.User{}}, &fakeSignupMail{}
	s := NewRegistrationService(RegistrationDeps{Users: users, Roles: fakeSignupRoles{}, Site: fakeSignupSite{},
		Account: fakeSignupAccount{acc: entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitorsApproval}}, Mail: mail, Logger: slog.New(slog.DiscardHandler)})
	_, outcome, err := s.Register(t.Context(), Signup{Username: "Daggoo", Email: "daggoo@example.com", Password: "harpoon1", PasswordConfirm: "harpoon1"})
	if err != nil || outcome != SignupPendingApproval || len(users.created) != 1 {
		t.Fatalf("outcome %v, err %v, created %d", outcome, err, len(users.created))
	}
	if len(mail.to) != 0 {
		t.Errorf("e-mails sent to %v", mail.to)
	}
}

// TestCreateAccountAndApprove - an administrator's account: the sign-up's
// checks plus the caller's (a role), active or blocked, its e-mail
// verified, welcomed; an account awaiting approval activated, a second
// approval refused.
func TestCreateAccountAndApprove(t *testing.T) {
	users := &fakeSignupUsers{existing: map[string]*entity.User{"starbuck": {ID: 1}}}
	mail := &fakeSignupMail{}
	tokens := NewUserAuthTokenService(&fakeResetTokens{})
	s := NewRegistrationService(RegistrationDeps{Users: users, Roles: fakeSignupRoles{}, Site: fakeSignupSite{}, Account: fakeSignupAccount{},
		Tokens: tokens, Mail: mail, Logger: slog.New(slog.DiscardHandler)})
	ctx := t.Context()

	_, err := s.CreateAccount(ctx, NewAccount{Username: "starbuck", Email: "mate@example.com", Password: "harpoon1", PasswordConfirm: "harpoon1",
		Invalid: SignupInvalid{"status": "Invalid status"}})
	var invalid SignupInvalid
	if !errors.As(err, &invalid) || invalid["username"] == "" || invalid["role_id"] == "" || invalid["status"] == "" {
		t.Fatalf("what to correct: %v", err)
	}
	role := &entity.Role{ID: 5, Name: "editors"}
	created, err := s.CreateAccount(ctx, NewAccount{Username: "Flask", Email: " Flask@Example.com", Password: "harpoon1", PasswordConfirm: "harpoon1",
		Role: role, Blocked: true, BaseURL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != entity.UserStatusBlocked || !created.EmailVerified || created.Role != role || created.Email != "flask@example.com" {
		t.Errorf("created %+v", created)
	}
	if fmt.Sprint(mail.to) != "[flask@example.com]" {
		t.Errorf("mailed %v", mail.to)
	}

	pending := &fakeApproveUsers{fakeSignupUsers: users, user: entity.User{ID: 9, Email: "pip@example.com", Status: entity.UserStatusPending}}
	s.users = pending
	if err := s.Approve(ctx, 9, "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if pending.user.Status != entity.UserStatusActive || !pending.user.EmailVerified {
		t.Errorf("approved %+v", pending.user)
	}
	if err := s.Approve(ctx, 9, "https://example.com"); !errors.Is(err, ErrNotPending) {
		t.Errorf("approved twice: %v", err)
	}
}

type fakeApproveUsers struct {
	*fakeSignupUsers
	user entity.User
}

func (f *fakeApproveUsers) ByID(context.Context, int) (*entity.User, error) {
	u := f.user
	return &u, nil
}

func (f *fakeApproveUsers) Edit(_ context.Context, u entity.User) (*entity.User, error) {
	f.user = u
	return &u, nil
}
