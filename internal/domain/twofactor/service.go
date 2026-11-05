package twofactor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The errors a user sees.
var (
	// ErrWrongCode - the code (or recovery code) isn't right, or was taken.
	ErrWrongCode = i18n.Errorf("The code is wrong or already used. Try the next one.")
	// ErrRequired - it can't be turned off: the user's role needs it.
	ErrRequired = i18n.Errorf("Two-factor sign-in is required for your role: it can't be turned off.")
	// ErrAlreadyOn - it is on: another app is set up only after turning it
	// off, which takes a code of the present one.
	ErrAlreadyOn = i18n.Errorf("Two-factor sign-in is already on.")
)

// settingsSource - the account settings (the roles that need it, the
// issuer's name).
type settingsSource interface {
	Settings(ctx context.Context) (*entity.AccountSettings, error)
}

// Service - two-factor sign-in: turning it on (a secret confirmed by a
// code, recovery codes), checking a code at sign-in, new recovery codes,
// turning it off (by the user with a code, by an administrator), whether
// a user's role needs it (the account settings, kept in memory until
// Invalidate).
type Service struct {
	repo     entity.TwoFactorRepository
	settings settingsSource
	siteName func(ctx context.Context) string
	now      func() time.Time
	// baseRole - the id of the role every signed-in user has
	// (entity.RoleUser; SetBaseRole, nil - none known).
	baseRole func(ctx context.Context) (int, error)

	mu       sync.Mutex
	roles    []int
	known    bool
	baseID   int
	baseRead bool
}

// NewService creates a Service; siteName - the issuer when the settings
// name none.
func NewService(repo entity.TwoFactorRepository, settings settingsSource, siteName func(ctx context.Context) string) *Service {
	return &Service{repo: repo, settings: settings, siteName: siteName, now: time.Now}
}

// Status - the user's two-factor sign-in, nil while it is off.
func (s *Service) Status(ctx context.Context, userID int) (*entity.TwoFactor, error) {
	return s.repo.ByUser(ctx, userID)
}

// Enabled - whether the user has it on.
func (s *Service) Enabled(ctx context.Context, userID int) (bool, error) {
	tf, err := s.repo.ByUser(ctx, userID)
	return tf != nil, err
}

// EnabledAmong - which of the users have it on.
func (s *Service) EnabledAmong(ctx context.Context, userIDs []int) (map[int]bool, error) {
	return s.repo.EnabledAmong(ctx, userIDs)
}

// Setup - what the page turning it on shows for secret: the QR code (an
// SVG) and the address it holds, the secret in groups to type by hand,
// the name the app shows.
type Setup struct {
	Secret  string
	Grouped string
	URI     string
	QR      string
	Issuer  string
	Account string
}

// NewSecret - a new secret for the page turning it on (kept in the
// user's session until confirmed).
func (s *Service) NewSecret() (string, error) { return NewSecret() }

// SetupFor - the page's view of secret for user.
func (s *Service) SetupFor(ctx context.Context, user entity.User, secret string) (Setup, error) {
	issuer := s.issuer(ctx)
	uri := URI(secret, issuer, user.Username)
	qr, err := QRSVG(uri)
	if err != nil {
		return Setup{}, err
	}
	return Setup{Secret: secret, Grouped: Grouped(secret), URI: uri, QR: qr, Issuer: issuer, Account: user.Username}, nil
}

// issuer - the site's name in the app: the settings', else the site's.
func (s *Service) issuer(ctx context.Context) string {
	if s.settings != nil {
		if a, err := s.settings.Settings(ctx); err == nil && a.TwoFactorIssuer != "" {
			return a.TwoFactorIssuer
		}
	}
	if s.siteName != nil {
		if n := s.siteName(ctx); n != "" {
			return n
		}
	}
	return "Kosatka"
}

// Enable turns it on for user with secret once code proves the app has
// it: the recovery codes to show once. ErrAlreadyOn - it is on already
// (an app is replaced only by turning it off with its code first).
func (s *Service) Enable(ctx context.Context, user entity.User, secret, code string) ([]string, error) {
	on, err := s.Enabled(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("checking the two-factor sign-in of user %d: %w", user.ID, err)
	}
	if on {
		return nil, ErrAlreadyOn
	}
	step, ok := Match(secret, code, s.now())
	if !ok {
		return nil, ErrWrongCode
	}
	codes, err := NewRecoveryCodes(RecoveryCodes)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Enable(ctx, user.ID, secret, step, HashRecoveryCodes(codes), s.now()); err != nil {
		return nil, err
	}
	audit.Record(ctx, audit.Action{Type: audit.TwoFactorOn, Object: userObject(user)})
	return codes, nil
}

// Verify checks a code at sign-in (or for a change of the user's own
// setting): the app's (each taken once) or a recovery code (spent).
// false - wrong; an error - couldn't check.
func (s *Service) Verify(ctx context.Context, userID int, code string) (bool, error) {
	tf, err := s.repo.ByUser(ctx, userID)
	if err != nil || tf == nil {
		return false, err
	}
	if step, ok := Match(tf.Secret, code, s.now()); ok {
		return s.repo.AcceptStep(ctx, userID, step)
	}
	return s.repo.UseRecoveryCode(ctx, userID, HashRecoveryCode(code))
}

// NewRecoveryCodes - new recovery codes for user (the old stop working),
// once code proves it's them: the codes to show once.
func (s *Service) NewRecoveryCodes(ctx context.Context, user entity.User, code string) ([]string, error) {
	if ok, err := s.Verify(ctx, user.ID, code); err != nil || !ok {
		return nil, errors.Join(ErrWrongCode, err)
	}
	codes, err := NewRecoveryCodes(RecoveryCodes)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceRecoveryCodes(ctx, user.ID, HashRecoveryCodes(codes)); err != nil {
		return nil, err
	}
	return codes, nil
}

// Disable turns it off for user once code proves it's them - unless
// their role needs it.
func (s *Service) Disable(ctx context.Context, user entity.User, code string) error {
	if s.Required(ctx, user) {
		return ErrRequired
	}
	if ok, err := s.Verify(ctx, user.ID, code); err != nil || !ok {
		return errors.Join(ErrWrongCode, err)
	}
	if err := s.repo.Disable(ctx, user.ID); err != nil {
		return err
	}
	audit.Record(ctx, audit.Action{Type: audit.TwoFactorOff, Object: userObject(user)})
	return nil
}

// Reset turns it off for user (an administrator's: they lost the phone
// and the codes); they turn it on again after signing in.
func (s *Service) Reset(ctx context.Context, user entity.User) error {
	if err := s.repo.Disable(ctx, user.ID); err != nil {
		return err
	}
	audit.Record(ctx, audit.Action{Type: audit.TwoFactorReset, Object: userObject(user)})
	return nil
}

// RequiredRoles - the roles whose users must use it.
func (s *Service) RequiredRoles(ctx context.Context) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.known && s.settings != nil {
		if a, err := s.settings.Settings(ctx); err == nil {
			s.roles, s.known = slices.Clone(a.TwoFactorRoles), true
		}
	}
	return slices.Clone(s.roles)
}

// Invalidate forgets the roles kept (the account settings were saved).
func (s *Service) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.known = false
}

// SetBaseRole gives the id of the role every signed-in user has: when the
// account settings require two-factor sign-in for it, every account needs
// it, whatever its own role.
func (s *Service) SetBaseRole(id func(ctx context.Context) (int, error)) {
	s.baseRole = id
}

// baseRoleID - the base role's id, read once (0 - unknown).
func (s *Service) baseRoleID(ctx context.Context) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.baseRead && s.baseRole != nil {
		if id, err := s.baseRole(ctx); err == nil {
			s.baseID, s.baseRead = id, true
		}
	}
	return s.baseID
}

// Required - whether user needs it: their own role does, or the base role
// every signed-in user has (SetBaseRole).
func (s *Service) Required(ctx context.Context, user entity.User) bool {
	role := user.RoleID
	if user.Role != nil {
		role = user.Role.ID
	}
	required := s.RequiredRoles(ctx)
	if role != 0 && slices.Contains(required, role) {
		return true
	}
	base := s.baseRoleID(ctx)
	return base != 0 && slices.Contains(required, base)
}

// SetupNeeded - whether user must turn it on before anything else: their
// role needs it and it's off.
func (s *Service) SetupNeeded(ctx context.Context, user entity.User) (bool, error) {
	if !s.Required(ctx, user) {
		return false, nil
	}
	on, err := s.Enabled(ctx, user.ID)
	if err != nil {
		return false, fmt.Errorf("checking the two-factor sign-in of user %d: %w", user.ID, err)
	}
	return !on, nil
}

// userObject - the user as the action log names them.
func userObject(u entity.User) audit.Object {
	return audit.Object{Kind: audit.KindUser, ID: strconv.Itoa(u.ID), Label: u.Username, URL: "/user/" + strconv.Itoa(u.ID) + "/edit"}
}
