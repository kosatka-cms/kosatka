package entity

import "context"

// Mail transports (MailSettings.Transport).
const (
	MailTransportResend = "resend"
	MailTransportSMTP   = "smtp"
)

// MailSettings - how the site sends email (one row). An empty Transport
// means nothing is saved and the server config's settings apply.
type MailSettings struct {
	// Enabled - the master switch: off means no email is sent.
	Enabled      bool
	Transport    string
	FromAddress  string
	ResendAPIKey string
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	// LogKeepDays - how many days the mail log keeps an email (0 - no
	// log).
	LogKeepDays int
}

// MailSettingsRepository stores the mail settings row.
type MailSettingsRepository interface {
	Get(ctx context.Context) (*MailSettings, error)
	Update(ctx context.Context, s *MailSettings) error
}
