package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// AccountSettingsRepository implements entity.AccountSettingsRepository.
type AccountSettingsRepository struct {
	client *generated.Client
}

// NewAccountSettingsRepository - the account settings repository on db.
func NewAccountSettingsRepository(db *sql.DB) *AccountSettingsRepository {
	return &AccountSettingsRepository{client: clientFor(db)}
}

// Get - the account settings (registration, avatars, two-factor, the
// account emails) from their single row.
func (r *AccountSettingsRepository) Get(ctx context.Context) (*entity.AccountSettings, error) {
	s, err := GetClient(ctx, r.client).AccountSettings.Get(ctx, settingsRowID)
	if err != nil {
		return nil, fmt.Errorf("AccountSettingsRepository.Get: %w", err)
	}
	return &entity.AccountSettings{
		RegistrationMode:           s.RegistrationMode,
		RequireEmailVerification:   s.RequireEmailVerification,
		UserTimezones:              s.UserTimezones,
		RegistrationBlockedDomains: s.RegistrationBlockedDomains,
		AvatarAllowedExtensions:    strings.Fields(s.AvatarAllowedExtensions),
		AvatarMaxFileSizeBytes:     s.AvatarMaxFileSizeBytes,
		TwoFactorRoles:             s.TwoFactorRoles,
		TwoFactorIssuer:            s.TwoFactorIssuer,
		CancelContent:              s.CancelContent,
		SubjWelcomeAdmin:           s.SubjWelcomeAdmin,
		BodyWelcomeAdmin:           s.BodyWelcomeAdmin,
		SubjWelcomePending:         s.SubjWelcomePending,
		BodyWelcomePending:         s.BodyWelcomePending,
		SubjWelcomeOpen:            s.SubjWelcomeOpen,
		BodyWelcomeOpen:            s.BodyWelcomeOpen,
		SubjActivation:             s.SubjActivation,
		BodyActivation:             s.BodyActivation,
		SubjPasswordReset:          s.SubjPasswordReset,
		BodyPasswordReset:          s.BodyPasswordReset,
		SubjEmailConfirm:           s.SubjEmailConfirm,
		BodyEmailConfirm:           s.BodyEmailConfirm,
		SubjAdminNotify:            s.SubjAdminNotify,
		BodyAdminNotify:            s.BodyAdminNotify,
	}, nil
}

// Update saves s into the account settings row.
func (r *AccountSettingsRepository) Update(ctx context.Context, s *entity.AccountSettings) error {
	return GetClient(ctx, r.client).AccountSettings.UpdateOneID(settingsRowID).
		SetRegistrationMode(s.RegistrationMode).
		SetRequireEmailVerification(s.RequireEmailVerification).
		SetUserTimezones(s.UserTimezones).
		SetRegistrationBlockedDomains(s.RegistrationBlockedDomains).
		SetAvatarAllowedExtensions(strings.Join(s.AvatarAllowedExtensions, " ")).
		SetAvatarMaxFileSizeBytes(s.AvatarMaxFileSizeBytes).
		SetTwoFactorRoles(s.TwoFactorRoles).
		SetTwoFactorIssuer(s.TwoFactorIssuer).
		SetCancelContent(s.CancelContent).
		SetSubjWelcomeAdmin(s.SubjWelcomeAdmin).SetBodyWelcomeAdmin(s.BodyWelcomeAdmin).
		SetSubjWelcomePending(s.SubjWelcomePending).SetBodyWelcomePending(s.BodyWelcomePending).
		SetSubjWelcomeOpen(s.SubjWelcomeOpen).SetBodyWelcomeOpen(s.BodyWelcomeOpen).
		SetSubjActivation(s.SubjActivation).SetBodyActivation(s.BodyActivation).
		SetSubjPasswordReset(s.SubjPasswordReset).SetBodyPasswordReset(s.BodyPasswordReset).
		SetSubjEmailConfirm(s.SubjEmailConfirm).SetBodyEmailConfirm(s.BodyEmailConfirm).
		SetSubjAdminNotify(s.SubjAdminNotify).SetBodyAdminNotify(s.BodyAdminNotify).
		Exec(ctx)
}
