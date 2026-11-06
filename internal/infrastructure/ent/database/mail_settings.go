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

// Get - the mail settings from their single row, with the modules' ways
// of sending's settings (mail_transport_settings).
func (r *MailSettingsRepository) Get(ctx context.Context) (*entity.MailSettings, error) {
	client := GetClient(ctx, r.client)
	m, err := client.MailSettings.Get(ctx, settingsRowID)
	if err != nil {
		return nil, fmt.Errorf("MailSettingsRepository.Get: %w", err)
	}
	rows, err := client.MailTransportSetting.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("MailSettingsRepository.Get: the transports' settings: %w", err)
	}
	settings := map[string]map[string]string{}
	for _, row := range rows {
		if settings[row.Transport] == nil {
			settings[row.Transport] = map[string]string{}
		}
		settings[row.Transport][row.Name] = openSecret(row.Value)
	}
	return &entity.MailSettings{
		Enabled: m.Enabled, Transport: m.Transport, FromAddress: m.FromAddress,
		SMTPHost: m.SMTPHost, SMTPPort: m.SMTPPort, SMTPUser: m.SMTPUser, SMTPPassword: openSecret(m.SMTPPassword),
		TransportSettings: settings,
		LogKeepDays:       m.LogKeepDays,
	}, nil
}

// Update saves the settings row (the secrets sealed), creating it if it
// isn't there, and replaces the transports' settings with s's (every
// value sealed; an empty one isn't kept).
func (r *MailSettingsRepository) Update(ctx context.Context, s *entity.MailSettings) error {
	password, err := sealSecret(s.SMTPPassword)
	if err != nil {
		return err
	}
	type setting struct{ transport, name, value string }
	var settings []setting
	for transport, values := range s.TransportSettings {
		for name, value := range values {
			if value == "" {
				continue
			}
			sealed, err := sealSecret(value)
			if err != nil {
				return err
			}
			settings = append(settings, setting{transport, name, sealed})
		}
	}
	return inTx(ctx, r.client, func(client *generated.Client) error {
		err := client.MailSettings.UpdateOneID(settingsRowID).
			SetEnabled(s.Enabled).SetTransport(s.Transport).SetFromAddress(s.FromAddress).
			SetSMTPHost(s.SMTPHost).SetSMTPPort(s.SMTPPort).SetSMTPUser(s.SMTPUser).SetSMTPPassword(password).
			SetLogKeepDays(max(s.LogKeepDays, 0)).
			Exec(ctx)
		if generated.IsNotFound(err) {
			err = client.MailSettings.Create().SetID(settingsRowID).
				SetEnabled(s.Enabled).SetTransport(s.Transport).SetFromAddress(s.FromAddress).
				SetSMTPHost(s.SMTPHost).SetSMTPPort(s.SMTPPort).SetSMTPUser(s.SMTPUser).SetSMTPPassword(password).
				SetLogKeepDays(max(s.LogKeepDays, 0)).
				Exec(ctx)
		}
		if err != nil {
			return fmt.Errorf("MailSettingsRepository.Update: %w", err)
		}
		if _, err := client.MailTransportSetting.Delete().Exec(ctx); err != nil {
			return fmt.Errorf("MailSettingsRepository.Update: the transports' settings: %w", err)
		}
		for _, st := range settings {
			if err := client.MailTransportSetting.Create().SetTransport(st.transport).SetName(st.name).SetValue(st.value).Exec(ctx); err != nil {
				return fmt.Errorf("MailSettingsRepository.Update: the setting %s.%s: %w", st.transport, st.name, err)
			}
		}
		return nil
	})
}
