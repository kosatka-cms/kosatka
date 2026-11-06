package entity

import "context"

// MailTransportSMTP - MailSettings.Transport of the engine's own way of
// sending: an SMTP server. The others are the modules' (their names).
const MailTransportSMTP = "smtp"

// MailSettings - how the site sends email (one row). An empty Transport
// means nothing is saved and the server config's settings apply.
type MailSettings struct {
	// Enabled - the master switch: off means no email is sent.
	Enabled      bool
	Transport    string
	FromAddress  string
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	// TransportSettings - the settings of the modules' ways of sending:
	// by transport, by setting.
	TransportSettings map[string]map[string]string
	// LogKeepDays - how many days the mail log keeps an email (0 - no
	// log).
	LogKeepDays int
}

// TransportSetting - the setting name of the transport ("" - unset).
func (s MailSettings) TransportSetting(transport, name string) string {
	return s.TransportSettings[transport][name]
}

// MailSettingsRepository stores the mail settings row.
type MailSettingsRepository interface {
	Get(ctx context.Context) (*MailSettings, error)
	Update(ctx context.Context, s *MailSettings) error
}
