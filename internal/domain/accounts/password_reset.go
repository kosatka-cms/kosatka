package accounts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
	"golang.org/x/crypto/bcrypt"
)

// ErrResetLinkInvalid - the reset link's token is unknown, used or
// expired.
var ErrResetLinkInvalid = errors.New("the password reset link is invalid or has expired")

// ErrResetPasswordInvalid - the new password isn't acceptable
// (ValidatePassword) or its confirmation differs.
var ErrResetPasswordInvalid = errors.New("the new password is not acceptable")

// PasswordResetService - a forgotten password: the reset link mailed to
// the account of an e-mail, a new password set by it.
type PasswordResetService struct {
	users   service.UserService
	tokens  *UserAuthTokenService
	site    entity.SiteInformationRepository
	account entity.AccountSettingsRepository
	mail    mailtemplate.Mailer
	uow     entity.UnitOfWork
	logger  *slog.Logger
}

// PasswordResetDeps - what NewPasswordResetService needs.
type PasswordResetDeps struct {
	Users   service.UserService
	Tokens  *UserAuthTokenService
	Site    entity.SiteInformationRepository
	Account entity.AccountSettingsRepository
	Mail    mailtemplate.Mailer
	UoW     entity.UnitOfWork
	Logger  *slog.Logger
}

// NewPasswordResetService - the password reset on d's services.
func NewPasswordResetService(d PasswordResetDeps) *PasswordResetService {
	return &PasswordResetService{users: d.Users, tokens: d.Tokens, site: d.Site, account: d.Account, mail: d.Mail, uow: d.UoW, logger: d.Logger}
}

// Request mails the reset link to the account of email (one not
// blocked), if there is one; baseURL - the site's address for the link
// ("" - the site URL isn't set: not sent). The visitor is told the same
// whatever happens (no account enumeration), so nothing is returned: a
// failure is logged.
func (s *PasswordResetService) Request(ctx context.Context, email, baseURL string) {
	u, err := s.users.Find(ctx, entity.UserFilter{Email: email})
	if err != nil || u == nil {
		service.LogFallback(s.logger, err, "finding the account of a password reset (none mailed)")
		return
	}
	if u.Status == entity.UserStatusBlocked {
		return
	}
	if baseURL == "" {
		service.LogFallback(s.logger, errors.New("the site URL isn't set"), "password reset e-mail not sent", "user_id", u.ID)
		return
	}
	site, _, err := LoadSiteAndAccount(ctx, s.site, s.account)
	if err != nil {
		service.LogFallback(s.logger, err, "loading the site for a password reset (not mailed)", "user_id", u.ID)
		return
	}
	raw, err := s.tokens.IssuePasswordReset(ctx, u.ID)
	if err != nil {
		service.LogFallback(s.logger, err, "issuing a password reset token (not mailed)", "user_id", u.ID)
		return
	}
	md := MailTokenData(baseURL, site, u, "", "", raw)
	service.LogFallback(s.logger, s.mail.Send(ctx, MailPasswordReset, u.Email, md),
		"sending the password reset e-mail", "user_id", u.ID)
}

// Reset sets the new password of the account the reset link's token
// names: the token used and the password saved in one transaction - a
// failed save leaves the link working. ErrResetPasswordInvalid - the
// password to correct (checked before the token is touched);
// ErrResetLinkInvalid - the link is no good.
func (s *PasswordResetService) Reset(ctx context.Context, rawToken, password, confirm string) error {
	if passwordErr, confirmErr := ValidatePassword(ctx, password, confirm); passwordErr != "" || confirmErr != "" {
		return ErrResetPasswordInvalid
	}
	if rawToken == "" {
		return ErrResetLinkInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing the new password: %w", err)
	}
	return s.uow.Do(ctx, func(ctx context.Context) error {
		uid, ok, err := s.tokens.ConsumePasswordReset(ctx, rawToken)
		if err != nil {
			return fmt.Errorf("using the reset token: %w", err)
		}
		if !ok {
			return ErrResetLinkInvalid
		}
		u, err := s.users.ByID(ctx, uid)
		if err != nil {
			return fmt.Errorf("loading the account of a reset (user %d): %w", uid, err)
		}
		u.PasswordHash = string(hash)
		// The link came to the address: following it confirms it.
		u.EmailVerified = true
		if _, err := s.users.Edit(ctx, *u); err != nil {
			return fmt.Errorf("saving the new password (user %d): %w", uid, err)
		}
		// The account's sessions end: whoever knew the old password is
		// signed out.
		if _, err := s.users.BumpSessionVersion(ctx, uid); err != nil {
			return fmt.Errorf("ending the sessions after a reset (user %d): %w", uid, err)
		}
		return nil
	})
}
