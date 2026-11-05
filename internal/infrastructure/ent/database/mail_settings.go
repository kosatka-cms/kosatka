package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// MailSettingsRepository implements entity.MailSettingsRepository.
type MailSettingsRepository struct {
	client *generated.Client
}

var _ entity.MailSettingsRepository = (*MailSettingsRepository)(nil)

// NewMailSettingsRepository - the mail settings repository on db.
func NewMailSettingsRepository(db *sql.DB) *MailSettingsRepository {
	return &MailSettingsRepository{client: clientFor(db)}
}

// Get - the mail settings from their single row.
func (r *MailSettingsRepository) Get(ctx context.Context) (*entity.MailSettings, error) {
	m, err := GetClient(ctx, r.client).MailSettings.Get(ctx, settingsRowID)
	if err != nil {
		return nil, fmt.Errorf("MailSettingsRepository.Get: %w", err)
	}
	return &entity.MailSettings{
		Enabled: m.Enabled, Transport: m.Transport, FromAddress: m.FromAddress, ResendAPIKey: openSecret(m.ResendAPIKey),
		SMTPHost: m.SMTPHost, SMTPPort: m.SMTPPort, SMTPUser: m.SMTPUser, SMTPPassword: openSecret(m.SMTPPassword),
		LogKeepDays: m.LogKeepDays,
	}, nil
}

// Update saves the settings row (the secrets sealed), creating it if it
// isn't there.
func (r *MailSettingsRepository) Update(ctx context.Context, s *entity.MailSettings) error {
	client := GetClient(ctx, r.client)
	key, err := sealSecret(s.ResendAPIKey)
	if err != nil {
		return err
	}
	password, err := sealSecret(s.SMTPPassword)
	if err != nil {
		return err
	}
	err = client.MailSettings.UpdateOneID(settingsRowID).
		SetEnabled(s.Enabled).SetTransport(s.Transport).SetFromAddress(s.FromAddress).SetResendAPIKey(key).
		SetSMTPHost(s.SMTPHost).SetSMTPPort(s.SMTPPort).SetSMTPUser(s.SMTPUser).SetSMTPPassword(password).
		SetLogKeepDays(max(s.LogKeepDays, 0)).
		Exec(ctx)
	if !generated.IsNotFound(err) {
		return err
	}
	return client.MailSettings.Create().SetID(settingsRowID).
		SetEnabled(s.Enabled).SetTransport(s.Transport).SetFromAddress(s.FromAddress).SetResendAPIKey(key).
		SetSMTPHost(s.SMTPHost).SetSMTPPort(s.SMTPPort).SetSMTPUser(s.SMTPUser).SetSMTPPassword(password).
		SetLogKeepDays(max(s.LogKeepDays, 0)).
		Exec(ctx)
}
