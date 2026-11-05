package sociallogin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
	"github.com/kosatka-cms/kosatka/internal/lib/oauth"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrRegisterDisabled - the provider's account is connected to no
	// account, and only administrators create accounts.
	ErrRegisterDisabled = errors.New("oauth: registration disabled")
	// ErrStateMismatch - the callback's state isn't the one the session
	// keeps (no sign-in started here, or a forged callback).
	ErrStateMismatch = errors.New("oauth: state mismatch")
	// ErrProviderUnknown - the provider isn't one enabled on the site.
	ErrProviderUnknown = errors.New("oauth: unknown provider")
	// ErrEmailTaken - the provider's e-mail is an existing account's that
	// isn't connected to the provider: its owner signs in with the
	// password and connects the provider in the profile.
	ErrEmailTaken = errors.New("oauth: the e-mail belongs to an existing account")
	// ErrEmailNeeded - the site signs in confirmed addresses only, and the
	// provider gave none: the visitor signs up with the form.
	ErrEmailNeeded = errors.New("oauth: no e-mail address to confirm")
	// ErrConnectedElsewhere - the provider's account is connected to
	// another account of the site.
	ErrConnectedElsewhere = errors.New("oauth: the provider's account is another user's")
	// ErrProviderConnected - the signed-in account has another account of
	// this provider connected.
	ErrProviderConnected = errors.New("oauth: an account of this provider is connected already")
)

// Result describes where to redirect after OAuth callback.
type Result struct {
	RedirectTo      string
	PendingApproval bool
	// ConfirmEmail - a new account to confirm by the link mailed to
	// PendingEmail.
	ConfirmEmail bool
	PendingEmail string
	// Connected - the provider's account was connected to the signed-in
	// user's now.
	Connected bool
}

type oauthProviderRegistry interface {
	Get(name string) (oauth.Provider, bool)
	EnabledNames() []string
}

// Service - sign-in with an OAuth provider: its sign-in page, then its
// callback signing in the account connected to the provider's, connecting
// it to the signed-in user's or signing a new account up.
type Service struct {
	registry            oauthProviderRegistry
	oauthRepo           entity.UserOAuthIdentityRepository
	userService         service.UserService
	roleRepo            entity.RoleRepository
	authService         service.AuthService
	accountSettingsRepo entity.AccountSettingsRepository
	siteInformationRepo entity.SiteInformationRepository
	emailSender         mailtemplate.Mailer
	usernamePattern     *regexp.Regexp
	// userFields - the profile fields a user fills (SetUserFields): an
	// OAuth sign-up asks none, so the user completes the profile after.
	userFields *accounts.UserFieldService
	// registration - mails a new account's e-mail confirmation
	// (SetRegistration).
	registration *accounts.RegistrationService
	// logger - the fallbacks (e-mails not sent).
	logger *slog.Logger
}

// SetUserFields sets the profile fields an OAuth sign-up's user completes.
func (s *Service) SetUserFields(f *accounts.UserFieldService) { s.userFields = f }

// SetRegistration sets what mails a new account its e-mail confirmation.
func (s *Service) SetRegistration(r *accounts.RegistrationService) { s.registration = r }

// UserFields - the profile fields (nil when not set).
func (s *Service) UserFields() *accounts.UserFieldService { return s.userFields }

// NeedsCompleteProfile - the user has a required profile field to fill
// (an OAuth sign-up asks none). A failed check doesn't hold the sign-in.
func (s *Service) NeedsCompleteProfile(ctx context.Context, user *entity.User) bool {
	if user == nil || s.userFields == nil {
		return false
	}
	missing, err := s.userFields.MissingRequired(ctx, user.ID)
	service.LogFallback(s.logger, err, "checking the user's required profile fields", "user_id", user.ID)
	return missing
}

// NewService - the social sign-in with registry's providers, the links of
// their accounts in oauthRepo, and the site's users, roles, sign-in,
// account settings and mail.
func NewService(
	registry oauthProviderRegistry,
	oauthRepo entity.UserOAuthIdentityRepository,
	userService service.UserService,
	roleRepo entity.RoleRepository,
	authService service.AuthService,
	accountSettingsRepo entity.AccountSettingsRepository,
	siteInformationRepo entity.SiteInformationRepository,
	emailSender mailtemplate.Mailer,
	logger *slog.Logger,
) *Service {
	return &Service{
		logger:              logger.With("component", "oauth.service"),
		registry:            registry,
		oauthRepo:           oauthRepo,
		userService:         userService,
		roleRepo:            roleRepo,
		authService:         authService,
		accountSettingsRepo: accountSettingsRepo,
		siteInformationRepo: siteInformationRepo,
		emailSender:         emailSender,
		usernamePattern:     regexp.MustCompile(`^[\p{L}\p{N}\s.\-_']+$`),
	}
}

// EnabledProviders - the names of the providers one can sign in with.
func (s *Service) EnabledProviders() []string {
	if s.registry == nil {
		return nil
	}
	return s.registry.EnabledNames()
}

// NewState - a random state for a provider's sign-in, kept in the session
// for HandleCallback to check.
func (s *Service) NewState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthURL - the provider's sign-in page for state; ErrProviderUnknown when
// the provider isn't enabled.
func (s *Service) AuthURL(providerName, state string) (string, error) {
	provider, ok := s.provider(providerName)
	if !ok {
		return "", ErrProviderUnknown
	}
	return provider.AuthCodeURL(state), nil
}

// HandleCallback completes a provider's sign-in on its callback: checks
// state against sessionState, exchanges code for the provider's profile,
// then signs in the account connected to it, connects it to the signed-in
// user's or signs a new account up as the registration settings allow
// (clientIP and timezone - the new account's). Result - where to go next
// (destination, else /me) or what the visitor is to do.
func (s *Service) HandleCallback(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	providerName, code, state, sessionState, destination, clientIP, timezone string,
) (*Result, error) {
	if sessionState == "" || state == "" || state != sessionState {
		return nil, ErrStateMismatch
	}
	provider, ok := s.provider(providerName)
	if !ok {
		return nil, ErrProviderUnknown
	}

	token, err := provider.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth exchange: %w", err)
	}
	profile, err := provider.UserInfo(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("oauth userinfo: %w", err)
	}
	if profile.ProviderUserID == "" {
		return nil, fmt.Errorf("oauth: empty provider user id")
	}

	redirectTo := service.SafeRedirectPath(destination)
	if redirectTo == "" {
		redirectTo = "/me"
	}

	identity, err := s.oauthRepo.FindByProviderUser(ctx, providerName, profile.ProviderUserID)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}
	// Signed in: the provider's account is connected to theirs.
	if userID, ok := s.authService.GetUserID(r); ok {
		return s.connect(ctx, userID, providerName, profile, identity, redirectTo)
	}
	if identity != nil {
		user, err := s.userService.ByID(ctx, identity.UserID)
		if err != nil {
			return nil, err
		}
		// Signed up with the provider, not approved yet: the same page
		// as right after the sign-up.
		if user.Status == entity.UserStatusPending {
			return &Result{PendingApproval: true, PendingEmail: user.Email}, nil
		}
		if err := s.authService.LoginUser(ctx, w, r, user); err != nil {
			return secondFactorOr(err, redirectTo)
		}
		if s.NeedsCompleteProfile(ctx, user) {
			redirectTo = oauthCompleteProfileURL(redirectTo)
		}
		return &Result{RedirectTo: redirectTo}, nil
	}

	email := strings.TrimSpace(strings.ToLower(profile.Email))
	if email == "" && providerName == entity.OAuthProviderVK {
		email = fmt.Sprintf("vk_%s@oauth.local", profile.ProviderUserID)
	}

	acc, err := s.accountSettings(ctx)
	if err != nil {
		return nil, err
	}
	if email != "" {
		existing, findErr := s.userService.Find(ctx, entity.UserFilter{Email: email})
		// A failed lookup (not "no such user") must not go on to create a
		// second account for the same e-mail.
		if findErr != nil && !service.IsNotFound(findErr) {
			return nil, fmt.Errorf("finding the user with the provider's e-mail: %w", findErr)
		}
		// The address is an account's: its owner signs in with the
		// password and connects the provider in the profile. Linking by
		// the address would sign the provider's user in to whoever
		// registered it here first.
		if findErr == nil && existing != nil {
			return nil, ErrEmailTaken
		}
	}

	// No account to sign in to: a new one, as the registration settings
	// allow (with "administrators only", none; an account connected to the
	// provider signs in whatever they are).
	if acc.RegistrationMode == entity.RegistrationModeAdminOnly {
		return nil, ErrRegisterDisabled
	}
	if email != "" && acc.EmailDomainBlocked(email) {
		return nil, errors.New("registration from this email domain is not allowed")
	}

	user, outcome, err := s.createOAuthUser(ctx, acc, providerName, profile, email, clientIP, timezone)
	if err != nil {
		return nil, err
	}

	// The link is made for an account awaiting approval too: once
	// approved, it signs in with the provider directly.
	_, err = s.oauthRepo.Create(ctx, entity.UserOAuthIdentity{
		UserID:         user.ID,
		Provider:       providerName,
		ProviderUserID: profile.ProviderUserID,
		Email:          email,
		CreatedAt:      time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("linking %s account to the new user (id %d): %w", providerName, user.ID, err)
	}
	switch outcome {
	case accounts.SignupPendingApproval:
		return &Result{PendingApproval: true, PendingEmail: user.Email}, nil
	case accounts.SignupConfirmEmail:
		if s.registration != nil {
			s.registration.MailEmailConfirmation(ctx, user, entity.NewSystemSettings().Domain)
		}
		return &Result{ConfirmEmail: true, PendingEmail: user.Email}, nil
	}

	if err := s.authService.LoginUser(ctx, w, r, user); err != nil {
		return secondFactorOr(err, redirectTo)
	}
	if s.NeedsCompleteProfile(ctx, user) {
		redirectTo = oauthCompleteProfileURL(redirectTo)
	}
	return &Result{RedirectTo: redirectTo}, nil
}

// secondFactorOr - a sign-in waiting for the two-factor code goes to the
// code page, then to dest; any other error is the error.
func secondFactorOr(err error, dest string) (*Result, error) {
	if errors.Is(err, service.ErrSecondFactorNeeded) {
		return &Result{RedirectTo: service.SecondFactorURL(dest)}, nil
	}
	return nil, err
}

func oauthCompleteProfileURL(dest string) string {
	q := url.Values{}
	if d := service.SafeRedirectPath(dest); d != "" {
		q.Set("destination", d)
	}
	if enc := q.Encode(); enc != "" {
		return "/user/complete-profile?" + enc
	}
	return "/user/complete-profile"
}

// createOAuthUser creates the account of a provider's user, as the
// registration settings say: awaiting approval, active once its address
// is confirmed (one the provider doesn't vouch for, where the site signs
// in confirmed addresses only), or active. ErrEmailNeeded - confirmed
// addresses only, and the provider gave none.
func (s *Service) createOAuthUser(
	ctx context.Context,
	acc *entity.AccountSettings,
	providerName string,
	profile *oauth.Profile,
	email, clientIP, timezone string,
) (*entity.User, accounts.SignupOutcome, error) {
	placeholder := email == "" || strings.HasSuffix(email, "@oauth.local")
	if acc.RegistrationMode != entity.RegistrationModeVisitorsApproval && acc.RequireEmailVerification && placeholder {
		return nil, 0, ErrEmailNeeded
	}
	role, err := accounts.EnsureUserRole(ctx, s.roleRepo)
	if err != nil {
		return nil, 0, err
	}
	hash, err := randomPasswordHash()
	if err != nil {
		return nil, 0, err
	}
	username, err := s.uniqueUsername(ctx, profile.DisplayName, email)
	if err != nil {
		return nil, 0, err
	}
	user := entity.User{
		Username:     username,
		Slug:         entity.UserSlug(username),
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		// Confirmed as far as the provider vouches for it.
		EmailVerified: !placeholder && profile.EmailVerified,
		Status:        entity.UserStatusActive,
		// Timezone - the browser's, as the sign-in link carried it
		// (already checked by the caller; "" - the site's).
		Timezone: timezone,
		Metadata: map[string]any{
			entity.UserMetadataKeyRegistrationIP: clientIP,
			entity.UserMetadataKeyOAuthProvider:  providerName,
		},
	}
	outcome := accounts.SignupActive
	switch {
	case acc.RegistrationMode == entity.RegistrationModeVisitorsApproval:
		user.Status = entity.UserStatusPending
		outcome = accounts.SignupPendingApproval
	case acc.RequireEmailVerification && !user.EmailVerified:
		outcome = accounts.SignupConfirmEmail
	}
	created, err := s.userService.Create(ctx, user)
	if err != nil {
		return nil, 0, err
	}
	if outcome == accounts.SignupPendingApproval {
		s.sendPendingEmails(ctx, created)
	}
	return created, outcome, nil
}

// connect connects the provider's account (profile; identity - its link,
// if any) to the signed-in user userID: they sign in with it from then
// on.
func (s *Service) connect(ctx context.Context, userID int, providerName string, profile *oauth.Profile, identity *entity.UserOAuthIdentity, redirectTo string) (*Result, error) {
	if identity != nil {
		if identity.UserID != userID {
			return nil, ErrConnectedElsewhere
		}
		return &Result{RedirectTo: redirectTo}, nil
	}
	switch _, err := s.oauthRepo.FindByUserAndProvider(ctx, userID, providerName); {
	case err == nil:
		return nil, ErrProviderConnected
	case !errors.Is(err, apperrors.ErrNotFound):
		return nil, fmt.Errorf("the %s account connected to user %d: %w", providerName, userID, err)
	}
	if _, err := s.oauthRepo.Create(ctx, entity.UserOAuthIdentity{
		UserID:         userID,
		Provider:       providerName,
		ProviderUserID: profile.ProviderUserID,
		Email:          strings.TrimSpace(strings.ToLower(profile.Email)),
		CreatedAt:      time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("connecting the %s account to user %d: %w", providerName, userID, err)
	}
	return &Result{RedirectTo: redirectTo, Connected: true}, nil
}

// ErrLastSignIn - disconnecting the provider would leave the account no
// way to sign in: no address a password reset could reach.
var ErrLastSignIn = errors.New("oauth: the account's only way to sign in")

// Connected - the providers connected to userID's account.
func (s *Service) Connected(ctx context.Context, userID int) ([]string, error) {
	ids, err := s.oauthRepo.ByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("the providers connected to user %d: %w", userID, err)
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.Provider)
	}
	return out, nil
}

// Disconnect disconnects the provider from user's account. ErrLastSignIn -
// the account has no address to reset its password by (a provider's
// placeholder).
func (s *Service) Disconnect(ctx context.Context, user *entity.User, providerName string) error {
	if user.Email == "" || strings.HasSuffix(user.Email, "@oauth.local") {
		return ErrLastSignIn
	}
	if err := s.oauthRepo.Delete(ctx, user.ID, providerName); err != nil {
		return fmt.Errorf("disconnecting %s from user %d: %w", providerName, user.ID, err)
	}
	return nil
}

// sendPendingEmails tells a new account awaiting approval and the site's
// e-mail of it (as RegistrationService does); a failure is logged: the
// account is there.
func (s *Service) sendPendingEmails(ctx context.Context, user *entity.User) {
	if s.emailSender == nil {
		return
	}
	base := entity.NewSystemSettings().Domain
	if base == "" {
		service.LogFallback(s.logger, errors.New("the site URL isn't set"), "e-mails of an account awaiting approval not sent", "user_id", user.ID)
		return
	}
	var site *entity.SiteInformation
	if s.siteInformationRepo != nil {
		var err error
		if site, err = s.siteInformationRepo.Get(ctx); err != nil {
			service.LogFallback(s.logger, err, "loading the site for the e-mails of an account awaiting approval (not sent)", "user_id", user.ID)
			return
		}
	} else {
		site = &entity.SiteInformation{}
	}
	md := accounts.MailTokenData(base, site, user, "", "", "")
	service.LogFallback(s.logger, s.emailSender.Send(ctx, accounts.MailWelcomePending, user.Email, md),
		"sending the welcome (pending approval) e-mail", "user_id", user.ID)
	if strings.TrimSpace(site.SiteEmail) != "" {
		service.LogFallback(s.logger, s.emailSender.Send(ctx, accounts.MailAdminNotify, site.SiteEmail, md),
			"sending the administrators' registration notice", "user_id", user.ID)
	}
}

func (s *Service) uniqueUsername(ctx context.Context, displayName, email string) (string, error) {
	base := strings.TrimSpace(displayName)
	if base == "" && email != "" {
		if at := strings.Index(email, "@"); at > 0 {
			base = email[:at]
		}
	}
	if base == "" {
		base = "user"
	}
	base = strings.TrimSpace(base)
	if len([]rune(base)) > 40 {
		base = string([]rune(base)[:40])
	}
	if !s.usernamePattern.MatchString(base) {
		base = "user"
	}
	candidate := base
	for i := 0; i < 100; i++ {
		if i > 0 {
			candidate = fmt.Sprintf("%s%d", base, i)
		}
		existing, err := s.userService.FindByUsername(ctx, candidate)
		if err != nil && !service.IsNotFound(err) {
			return "", fmt.Errorf("checking the username %q: %w", candidate, err)
		}
		if existing == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not allocate username")
}

func randomPasswordHash() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword(b, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *Service) provider(name string) (oauth.Provider, bool) {
	if s.registry == nil {
		return nil, false
	}
	return s.registry.Get(name)
}

func (s *Service) accountSettings(ctx context.Context) (*entity.AccountSettings, error) {
	if s.accountSettingsRepo == nil {
		return &entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitors}, nil
	}
	acc, err := s.accountSettingsRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	accounts.MergeAccountSettingsDefaults(acc)
	return acc, nil
}
