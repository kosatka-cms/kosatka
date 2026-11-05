package accounts

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// AccountPolicyService - the account settings (account_settings): the
// registration policy, the settings page; saving applies the users' own
// time zones.
type AccountPolicyService struct {
	repo entity.AccountSettingsRepository
	// userZonesChanged runs after a save turns the users' own time zones
	// on or off (OnUserZonesChanged).
	userZonesChanged func()
}

// NewAccountPolicyService - the account settings' service on repo.
func NewAccountPolicyService(repo entity.AccountSettingsRepository) *AccountPolicyService {
	return &AccountPolicyService{repo: repo}
}

// OnUserZonesChanged sets what runs after a save turns the users' own time
// zones on or off: the View block cache, which knows the blocks depending
// on the zone.
func (s *AccountPolicyService) OnUserZonesChanged(fn func()) { s.userZonesChanged = fn }

func (s *AccountPolicyService) settings(ctx context.Context) (*entity.AccountSettings, error) {
	if s == nil || s.repo == nil {
		return &entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitors}, nil
	}
	a, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	MergeAccountSettingsDefaults(a)
	return a, nil
}

// Settings - the saved settings, the defaults where none are saved.
func (s *AccountPolicyService) Settings(ctx context.Context) (*entity.AccountSettings, error) {
	a, err := s.settings(ctx)
	return entity.Wrapped(a, err, "loading the account settings")
}

// Save stores the settings but the emails' texts (the Mail page's: kept
// as saved); the users' own time zones and the pages' "Register" links go
// on or off right away.
func (s *AccountPolicyService) Save(ctx context.Context, a *entity.AccountSettings) error {
	current, err := s.repo.Get(ctx)
	if err != nil {
		return fmt.Errorf("loading the account settings: %w", err)
	}
	KeepMailTexts(a, current)
	if err := s.repo.Update(ctx, a); err != nil {
		return fmt.Errorf("saving the account settings: %w", err)
	}
	entity.SetRegistrationMode(a.RegistrationMode)
	if sitetime.UserZonesEnabled() != a.UserTimezones {
		sitetime.SetUserZones(a.UserTimezones)
		if s.userZonesChanged != nil {
			s.userZonesChanged()
		}
	}
	return nil
}

// RequireEmailVerification - whether only accounts with a confirmed e-mail
// may sign in (the account settings).
func (s *AccountPolicyService) RequireEmailVerification(ctx context.Context) (bool, error) {
	a, err := s.settings(ctx)
	if err != nil {
		return false, err
	}
	return a.RequireEmailVerification, nil
}

// RegistrationMode - who may create accounts (the account settings'
// entity.RegistrationMode…); RegistrationModeVisitors on an error.
func (s *AccountPolicyService) RegistrationMode(ctx context.Context) (string, error) {
	a, err := s.settings(ctx)
	if err != nil {
		return entity.RegistrationModeVisitors, err
	}
	return a.RegistrationMode, nil
}
