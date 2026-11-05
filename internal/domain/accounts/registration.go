package accounts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
	"golang.org/x/crypto/bcrypt"
)

// emailConfirmationTTL - how long the link confirming a new account's
// email works.
const emailConfirmationTTL = 24 * time.Hour

// ErrRegistrationClosed - the site's accounts are created by its
// administrators only (entity.RegistrationModeAdminOnly).
var ErrRegistrationClosed = errors.New("registration is closed")

// SignupInvalid - what the visitor is to correct, by form field
// (username, email, password, password_confirm, user_fields).
type SignupInvalid map[string]string

func (e SignupInvalid) Error() string {
	return fmt.Sprintf("invalid sign-up: %v", map[string]string(e))
}

// Signup - what a visitor submits on the sign-up form, and where from.
type Signup struct {
	Username, Email, Password, PasswordConfirm string
	// Timezone - the visitor's IANA time zone ("" - the site's).
	Timezone string
	// IP - the address registering (the account's metadata).
	IP string
	// BaseURL - the site's address for the links in the emails: the site
	// URL set, never the request's host (empty - the emails aren't sent).
	BaseURL string
	// FieldValue - the form's value by input name (the required profile
	// fields, UserFieldFormName).
	FieldValue func(name string) string
}

// SignupOutcome - what a new account can do next.
type SignupOutcome int

const (
	// SignupActive - the account is active: the visitor can log in now.
	SignupActive SignupOutcome = iota
	// SignupConfirmEmail - the account is active once its email is
	// confirmed by the link sent to it.
	SignupConfirmEmail
	// SignupPendingApproval - an administrator is to approve the account.
	SignupPendingApproval
)

// RegistrationService - a visitor creating an account (the sign-up
// form): the site's registration mode (entity.AccountSettings), the
// checks, the account with the "user" role, its required profile fields
// and the emails the mode sends.
type RegistrationService struct {
	users   service.UserService
	roles   entity.RoleRepository
	site    entity.SiteInformationRepository
	account entity.AccountSettingsRepository
	// fields - the required profile fields (nil - none).
	fields *UserFieldService
	// tokens - the one-time login link of the welcome email (nil - none).
	tokens *UserAuthTokenService
	mail   mailtemplate.Mailer
	// uow - the transaction accepting an invitation (nil - none: only
	// AcceptInvite needs it).
	uow    entity.UnitOfWork
	logger *slog.Logger
}

// RegistrationDeps - what NewRegistrationService needs.
type RegistrationDeps struct {
	Users   service.UserService
	Roles   entity.RoleRepository
	Site    entity.SiteInformationRepository
	Account entity.AccountSettingsRepository
	Fields  *UserFieldService
	Tokens  *UserAuthTokenService
	Mail    mailtemplate.Mailer
	UoW     entity.UnitOfWork
	Logger  *slog.Logger
}

// NewRegistrationService - the sign-up on d's services.
func NewRegistrationService(d RegistrationDeps) *RegistrationService {
	return &RegistrationService{users: d.Users, roles: d.Roles, site: d.Site, account: d.Account, fields: d.Fields,
		tokens: d.Tokens, mail: d.Mail, uow: d.UoW, logger: d.Logger}
}

// Open - whether visitors can create accounts.
func (s *RegistrationService) Open(ctx context.Context) (bool, error) {
	_, acc, err := LoadSiteAndAccount(ctx, s.site, s.account)
	if err != nil {
		return false, err
	}
	return acc.RegistrationMode != entity.RegistrationModeAdminOnly, nil
}

// Register creates the visitor's account as the site's registration mode
// says:
//   - visitors (with approval): pending, the welcome email saying so, and
//     the site's email told of it;
//   - visitors with email confirmation: active once confirmed by the
//     link of the email sent (valid emailConfirmationTTL);
//   - visitors: active, the welcome email with a one-time login link.
//
// SignupInvalid - what to correct; ErrRegistrationClosed - accounts are
// the administrators' to create. An email not sent, profile fields not
// saved are logged: the account is there.
func (s *RegistrationService) Register(ctx context.Context, in Signup) (*entity.User, SignupOutcome, error) {
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	fieldValues, err := s.check(ctx, in.Username, in.Email, in.Password, in.PasswordConfirm, true, in.FieldValue, SignupInvalid{})
	if err != nil {
		return nil, 0, err
	}

	site, acc, err := LoadSiteAndAccount(ctx, s.site, s.account)
	if err != nil {
		return nil, 0, fmt.Errorf("loading the site's account settings: %w", err)
	}
	if acc.RegistrationMode == entity.RegistrationModeAdminOnly {
		return nil, 0, ErrRegistrationClosed
	}
	if acc.EmailDomainBlocked(in.Email) {
		return nil, 0, SignupInvalid{"email": i18n.T(ctx, "Registration from this email domain is not allowed.")}
	}

	role, err := EnsureUserRole(ctx, s.roles)
	if err != nil {
		return nil, 0, fmt.Errorf("ensuring the user role: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, 0, fmt.Errorf("hashing the password: %w", err)
	}
	user := entity.User{
		Username:     in.Username,
		Slug:         entity.UserSlug(in.Username),
		Email:        in.Email,
		PasswordHash: string(hash),
		Role:         role,
		Timezone:     in.Timezone,
		Status:       entity.UserStatusActive,
		Metadata:     map[string]any{entity.UserMetadataKeyRegistrationIP: in.IP},
	}
	outcome := SignupActive
	switch {
	case acc.RegistrationMode == entity.RegistrationModeVisitorsApproval:
		outcome = SignupPendingApproval
		user.Status = entity.UserStatusPending
	case acc.RequireEmailVerification:
		outcome = SignupConfirmEmail
	default:
		user.EmailVerified = true
	}

	created, err := s.users.Create(ctx, user)
	if err != nil {
		return nil, 0, fmt.Errorf("creating the account: %w", err)
	}
	if s.fields != nil {
		service.LogFallback(s.logger, s.fields.SaveValues(ctx, created.ID, fieldValues), "saving the new user's profile fields (completed later)", "user_id", created.ID)
	}

	switch outcome {
	case SignupPendingApproval:
		s.send(ctx, MailWelcomePending, created.Email, MailTokenData(in.BaseURL, site, created, "", "", ""))
		if strings.TrimSpace(site.SiteEmail) == "" {
			if s.logger != nil {
				s.logger.Warn("the site's email is empty: administrators not told of the registration", "user_id", created.ID)
			}
		} else {
			s.send(ctx, MailAdminNotify, site.SiteEmail, MailTokenData(in.BaseURL, site, created, "", "", ""))
		}
	case SignupConfirmEmail:
		// Without the link the account can't be confirmed: a failure is
		// the sign-up's (a password reset confirms the address too).
		if err := s.mailEmailConfirmation(ctx, in.BaseURL, site, acc, created); err != nil {
			return nil, 0, err
		}
	default:
		var oneTime string
		if s.tokens != nil {
			oneTime, err = s.tokens.IssueOneTimeLogin(ctx, created.ID)
			service.LogFallback(s.logger, err, "issuing the welcome email's one-time login (sent without it)", "user_id", created.ID)
		}
		s.send(ctx, MailWelcomeOpen, created.Email, MailTokenData(in.BaseURL, site, created, "", oneTime, ""))
	}
	return created, outcome, nil
}

// check - what to correct in an account's name, e-mail, password and
// required profile fields, added to invalid (the caller's own findings);
// the fields' values when all is well. A failure to read them is
// returned as is.
func (s *RegistrationService) check(ctx context.Context, username, email, password, confirm string, withPassword bool, fieldValue func(string) string, invalid SignupInvalid) (map[int]string, error) {
	if msg := ValidateUsername(ctx, s.users, username, 0); msg != "" {
		invalid["username"] = msg
	}
	if msg := ValidateEmail(ctx, s.users, email, 0); msg != "" {
		invalid["email"] = msg
	}
	// An invitation (no password: the person sets theirs) has none.
	if withPassword {
		passwordErr, confirmErr := ValidatePassword(ctx, password, confirm)
		if passwordErr != "" {
			invalid["password"] = passwordErr
		}
		if confirmErr != "" {
			invalid["password_confirm"] = confirmErr
		}
	}
	var fieldValues map[int]string
	if s.fields != nil && fieldValue != nil {
		var err error
		fieldValues, err = s.fields.RequiredFormValues(ctx, fieldValue)
		var fieldErr UserFieldFormError
		if errors.As(err, &fieldErr) {
			invalid["user_fields"] = fieldErr.Message(ctx)
		} else if err != nil {
			return nil, fmt.Errorf("reading the required profile fields: %w", err)
		}
	}
	if len(invalid) > 0 {
		return nil, invalid
	}
	return fieldValues, nil
}

// NewAccount - an account an administrator creates
// (/admin/people/create).
type NewAccount struct {
	Username, Email, Password, PasswordConfirm string
	// Invite - no password: the person gets the invitation and sets their
	// own (AcceptInvite); a blank Username - one made of the address.
	Invite bool
	// InvitedBy - the administrator's name in the invitation.
	InvitedBy string
	// Role - the role given; the caller checks it's the administrator's
	// to give (MayAssignRole). Nil - none chosen.
	Role *entity.Role
	// Blocked - created blocked; else active.
	Blocked bool
	// FieldValue - the form's value by input name (the required profile
	// fields).
	FieldValue func(name string) string
	// BaseURL - the site's address for the welcome e-mail's links ("" -
	// not sent).
	BaseURL string
	// Invalid - what the caller already found to correct (merged).
	Invalid SignupInvalid
}

// CreateAccount creates the account an administrator fills in: the
// checks of a sign-up (not the registration mode or the blocked e-mail
// domains - an administrator's choice), its e-mail taken as verified,
// its required profile fields; then the invitation (Invite: no password
// yet) or the welcome e-mail with a one-time login link. SignupInvalid -
// what to correct.
func (s *RegistrationService) CreateAccount(ctx context.Context, in NewAccount) (*entity.User, error) {
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	invalid := SignupInvalid{}
	maps.Copy(invalid, in.Invalid)
	if in.Role == nil && invalid["role_id"] == "" {
		invalid["role_id"] = i18n.T(ctx, "Select a role")
	}
	if in.Invite {
		if in.Username == "" {
			name, err := freeUsername(ctx, s.users, in.Email)
			if err != nil {
				return nil, err
			}
			in.Username = name
		}
	}
	fieldValues, err := s.check(ctx, in.Username, in.Email, in.Password, in.PasswordConfirm, !in.Invite, in.FieldValue, invalid)
	if err != nil {
		return nil, err
	}
	hash := []byte(entity.PasswordUnset)
	if !in.Invite {
		if hash, err = bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost); err != nil {
			return nil, fmt.Errorf("hashing the password: %w", err)
		}
	}
	user := entity.User{
		Username:      in.Username,
		Slug:          entity.UserSlug(in.Username),
		Email:         in.Email,
		PasswordHash:  string(hash),
		Status:        entity.UserStatusActive,
		EmailVerified: true,
		Role:          in.Role,
	}
	if in.Blocked {
		user.Status = entity.UserStatusBlocked
	}
	created, err := s.users.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("creating the account: %w", err)
	}
	if s.fields != nil {
		service.LogFallback(s.logger, s.fields.SaveValues(ctx, created.ID, fieldValues), "saving the new user's profile fields (completed later)", "user_id", created.ID)
	}
	if in.Invite {
		s.sendInvite(ctx, created, in.BaseURL, in.InvitedBy)
	} else {
		s.sendWithLogin(ctx, created, in.BaseURL, MailWelcomeAdmin)
	}
	return created, nil
}

// mailEmailConfirmation mails u the link confirming its e-mail address
// (a token of emailConfirmationTTL).
func (s *RegistrationService) mailEmailConfirmation(ctx context.Context, baseURL string, site *entity.SiteInformation, acc *entity.AccountSettings, u *entity.User) error {
	raw, err := s.tokens.IssueEmailConfirm(ctx, u.ID)
	if err != nil {
		return fmt.Errorf("issuing the e-mail confirmation of user %d: %w", u.ID, err)
	}
	s.send(ctx, MailEmailConfirm, u.Email, MailTokenData(baseURL, site, u, raw, "", ""))
	return nil
}

// MailEmailConfirmation - u's address isn't confirmed (a changed one, a
// social network's it doesn't vouch for): when the site signs in
// confirmed addresses only, it is mailed its confirmation link (baseURL -
// the site's address; "" - not sent). A failure is logged: the account is
// there.
func (s *RegistrationService) MailEmailConfirmation(ctx context.Context, u *entity.User, baseURL string) {
	site, acc, err := LoadSiteAndAccount(ctx, s.site, s.account)
	if err != nil {
		service.LogFallback(s.logger, err, "loading the account settings for an e-mail confirmation (none mailed)", "user_id", u.ID)
		return
	}
	if !acc.RequireEmailVerification || u.Email == "" {
		return
	}
	if baseURL == "" {
		service.LogFallback(s.logger, errors.New("the site URL isn't set"), "the e-mail confirmation not mailed", "user_id", u.ID)
		return
	}
	service.LogFallback(s.logger, s.mailEmailConfirmation(ctx, baseURL, site, acc, u), "mailing the e-mail confirmation", "user_id", u.ID)
}

// ErrNotPending - the account isn't awaiting approval.
var ErrNotPending = errors.New("the account is not awaiting approval")

// Approve activates an account awaiting approval (its e-mail taken as
// verified) and mails it the activation e-mail with a one-time login
// link (baseURL - the site's address; "" - not sent).
func (s *RegistrationService) Approve(ctx context.Context, userID int, baseURL string) error {
	u, err := s.users.ByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("loading the account (user %d): %w", userID, err)
	}
	if u.Status != entity.UserStatusPending {
		return ErrNotPending
	}
	u.Status = entity.UserStatusActive
	u.EmailVerified = true
	if _, err := s.users.Edit(ctx, *u); err != nil {
		return fmt.Errorf("activating the account (user %d): %w", userID, err)
	}
	service.AuditUserStatus(ctx, *u, entity.UserStatusPending, entity.UserStatusActive)
	s.sendWithLogin(ctx, u, baseURL, MailActivation)
	return nil
}

// sendWithLogin mails u the account e-mail key with a one-time login
// link. A failure is logged: the account is saved.
func (s *RegistrationService) sendWithLogin(ctx context.Context, u *entity.User, baseURL string, key string) {
	what := key
	if s.tokens == nil {
		return
	}
	site, _, err := LoadSiteAndAccount(ctx, s.site, s.account)
	if err != nil {
		service.LogFallback(s.logger, err, "loading the site for an account e-mail (not sent)", "email", what, "user_id", u.ID)
		return
	}
	if baseURL == "" {
		service.LogFallback(s.logger, errors.New("the site URL isn't set"), "account e-mail not sent", "email", what, "user_id", u.ID)
		return
	}
	oneTime, err := s.tokens.IssueOneTimeLogin(ctx, u.ID)
	if err != nil {
		service.LogFallback(s.logger, err, "issuing the one-time login of an account e-mail (not sent)", "email", what, "user_id", u.ID)
		return
	}
	s.send(ctx, key, u.Email, MailTokenData(baseURL, site, u, "", oneTime, ""))
}

// send - an email of the registration (key); not sent is logged.
func (s *RegistrationService) send(ctx context.Context, key, to string, data mailtemplate.TokenData) {
	// No site URL - no links to put in it: the e-mail isn't sent.
	if data.SiteURLBrief == "" {
		if s.logger != nil {
			s.logger.Warn("e-mail not sent: the site URL isn't set (Site information)", "email", key)
		}
		return
	}
	if s.mail == nil {
		return
	}
	if err := s.mail.Send(ctx, key, to, data); err != nil && s.logger != nil {
		s.logger.Error("sending a registration email", "email", key, "error", err)
	}
}

// LoadSiteAndAccount - the site's information and account settings (the
// defaults filled in; without a repository - empty information, the
// default settings).
func LoadSiteAndAccount(ctx context.Context, siteRepo entity.SiteInformationRepository, accountRepo entity.AccountSettingsRepository) (*entity.SiteInformation, *entity.AccountSettings, error) {
	var site entity.SiteInformation
	if siteRepo != nil {
		s, err := siteRepo.Get(ctx)
		if err != nil {
			return nil, nil, err
		}
		site = *s
	}
	acc := DefaultAccountSettings()
	if accountRepo != nil {
		var err error
		if acc, err = accountRepo.Get(ctx); err != nil {
			return nil, nil, err
		}
	}
	MergeAccountSettingsDefaults(acc)
	return &site, acc, nil
}

// MailTokenData - the tokens of an account email (mailtemplate) for u:
// the site's name and addresses under baseURL, and the links of the
// tokens given (an empty one - no link).
func MailTokenData(baseURL string, site *entity.SiteInformation, u *entity.User, confirmToken, oneTimeRaw, resetRaw string) mailtemplate.TokenData {
	base := strings.TrimSuffix(baseURL, "/")
	d := mailtemplate.TokenData{
		SiteName:             site.SiteName,
		SiteSlogan:           site.Slogan,
		SiteLoginURL:         base + "/login",
		SiteURLBrief:         baseURL,
		SitePeoplePendingURL: base + "/admin/people?status=pending",
		UserName:             u.Username,
		UserEmail:            u.Email,
		UserEditURL:          base + "/me/edit",
	}
	if confirmToken != "" {
		d.ConfirmEmailURL = base + "/user/confirm-email?token=" + confirmToken
	}
	if oneTimeRaw != "" {
		d.OneTimeLoginURL = base + "/user/login-once?token=" + oneTimeRaw
	}
	if resetRaw != "" {
		d.ResetPasswordURL = base + "/user/reset-password?token=" + resetRaw
	}
	return d
}
