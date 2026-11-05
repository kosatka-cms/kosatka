package accounts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"golang.org/x/crypto/bcrypt"
)

// ErrInviteInvalid - the invitation's link is no good: unknown, used or
// expired.
var ErrInviteInvalid = errors.New("the invitation link is invalid or expired")

// ErrNotInvited - the account has a password (not entity.PasswordUnset):
// nothing to invite to.
var ErrNotInvited = errors.New("the account is not waiting for an invitation")

// freeUsername - a username made of email's address part, free on the
// site (a number added when taken).
func freeUsername(ctx context.Context, users UserLookup, email string) (string, error) {
	base, _, _ := strings.Cut(email, "@")
	base = strings.TrimSpace(base)
	if utf8.RuneCountInString(base) > 40 {
		base = string([]rune(base)[:40])
	}
	if base == "" || !UsernameRegex.MatchString(base) {
		base = "user"
	}
	for i := 0; i < 100; i++ {
		name := base
		if i > 0 {
			name = fmt.Sprintf("%s%d", base, i)
		}
		existing, err := users.FindByUsername(ctx, name)
		if err != nil && !service.IsNotFound(err) {
			return "", fmt.Errorf("checking the username %q: %w", name, err)
		}
		if existing == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no free username for %q", base)
}

// sendInvite mails u the invitation with a new link (InviteTTL) to set
// their password; invitedBy - who invited. A failure is logged: the
// account is saved, the invitation can be sent again.
func (s *RegistrationService) sendInvite(ctx context.Context, u *entity.User, baseURL, invitedBy string) {
	service.LogFallback(s.logger, s.SendInvite(ctx, u, baseURL, invitedBy), "sending an invitation", "user_id", u.ID)
}

// SendInvite mails u - an account without a password yet
// (entity.PasswordUnset) - the invitation with a new link (the earlier
// one stops working). ErrNotInvited - u has a password; baseURL "" (the
// site URL isn't set) - none sent.
func (s *RegistrationService) SendInvite(ctx context.Context, u *entity.User, baseURL, invitedBy string) error {
	if u.PasswordHash != entity.PasswordUnset {
		return ErrNotInvited
	}
	if s.tokens == nil || s.mail == nil {
		return fmt.Errorf("inviting user %d: no tokens or mail", u.ID)
	}
	if baseURL == "" {
		return fmt.Errorf("inviting user %d: the site URL isn't set", u.ID)
	}
	site, _, err := LoadSiteAndAccount(ctx, s.site, s.account)
	if err != nil {
		return fmt.Errorf("inviting user %d: %w", u.ID, err)
	}
	raw, err := s.tokens.IssueInvite(ctx, u.ID)
	if err != nil {
		return fmt.Errorf("inviting user %d: %w", u.ID, err)
	}
	role := ""
	if u.Role != nil {
		role = u.Role.Label()
	}
	data := MailTokenData(baseURL, site, u, "", "", "")
	data.Extra = map[string]string{
		"[user:invite-url]": strings.TrimSuffix(baseURL, "/") + "/user/invite?token=" + raw,
		"[invite:by]":       invitedBy,
		"[invite:role]":     role,
	}
	if err := s.mail.Send(ctx, MailInvite, u.Email, data); err != nil {
		return fmt.Errorf("inviting user %d: %w", u.ID, err)
	}
	return nil
}

// InvitedUser - the account of an invitation's link that still works
// (the form shown before it is accepted). ErrInviteInvalid - none.
func (s *RegistrationService) InvitedUser(ctx context.Context, rawToken string) (*entity.User, error) {
	if rawToken == "" || s.tokens == nil {
		return nil, ErrInviteInvalid
	}
	uid, ok, err := s.tokens.ValidInvite(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("checking an invitation: %w", err)
	}
	if !ok {
		return nil, ErrInviteInvalid
	}
	u, err := s.users.ByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("loading the invited user %d: %w", uid, err)
	}
	return u, nil
}

// AcceptInvite - the invited person sets their username and password by
// the link: the link used and the account saved in one transaction (a
// failed save leaves the link working); the address taken as confirmed.
// The account, to sign in. SignupInvalid - what to correct (checked
// before the link is touched); ErrInviteInvalid - the link is no good.
func (s *RegistrationService) AcceptInvite(ctx context.Context, rawToken, username, password, confirm string) (*entity.User, error) {
	u, err := s.InvitedUser(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	username = strings.TrimSpace(username)
	invalid := SignupInvalid{}
	if msg := ValidateUsername(ctx, s.users, username, u.ID); msg != "" {
		invalid["username"] = msg
	}
	passwordErr, confirmErr := ValidatePassword(ctx, password, confirm)
	if passwordErr != "" {
		invalid["password"] = passwordErr
	}
	if confirmErr != "" {
		invalid["password_confirm"] = confirmErr
	}
	if len(invalid) > 0 {
		return nil, invalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing the password: %w", err)
	}
	if s.uow == nil {
		return nil, fmt.Errorf("accepting an invitation: no transaction")
	}
	var accepted *entity.User
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		uid, ok, err := s.tokens.ConsumeInvite(ctx, rawToken)
		if err != nil {
			return fmt.Errorf("using the invitation: %w", err)
		}
		if !ok || uid != u.ID {
			return ErrInviteInvalid
		}
		u, err := s.users.ByID(ctx, uid)
		if err != nil {
			return fmt.Errorf("loading the invited user %d: %w", uid, err)
		}
		if u.Username != username {
			u.Username, u.Slug = username, entity.UserSlug(username)
		}
		u.PasswordHash, u.EmailVerified = string(hash), true
		if accepted, err = s.users.Edit(ctx, *u); err != nil {
			return fmt.Errorf("saving the invited user %d: %w", uid, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return accepted, nil
}

// PendingInvites - when the invitation of each of userIDs not accepted
// yet expires (an expired one too); the others - absent.
func (s *RegistrationService) PendingInvites(ctx context.Context, userIDs []int) (map[int]time.Time, error) {
	return s.tokens.PendingInvites(ctx, userIDs)
}
