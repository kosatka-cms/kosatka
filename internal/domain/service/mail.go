package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/mailsender"
)

// Where the mail settings in effect come from (MailStatus.Source).
const (
	MailSourceAdmin  = "admin"
	MailSourceConfig = "config"
	MailSourceNone   = "none"
)

// ErrMailOff - email is switched off or not configured.
var ErrMailOff = errors.New("email sending is off or not configured")

// MailStatus - the mail settings in effect, for the settings page (no
// secrets): FromAddress - the sender's address in effect.
type MailStatus struct {
	Enabled     bool
	Transport   string
	FromAddress string
	Source      string
}

// MailService sends the site's email through the transport in effect:
// the server config's when it sets one (it can't be changed in the admin
// then), else the settings saved in the admin (entity.MailSettings). It
// is the site's mailsender.EmailSender; Reload applies new settings
// without a restart.
type MailService struct {
	repo       entity.MailSettingsRepository
	configured entity.MailSettings
	logger     *slog.Logger
	// newSender builds the transport for settings (tests replace it).
	newSender func(entity.MailSettings) mailsender.EmailSender
	// templates - what the Mail page changed of the emails (SetTemplates).
	templates entity.MailTemplateRepository
	// log - the mail log (SetLog); nil - none.
	log entity.MailLogRepository

	mu     sync.RWMutex
	sender mailsender.EmailSender
	status MailStatus
	// logKeepDays - how long the log keeps an email (the saved settings'
	// LogKeepDays; 0 - no log).
	logKeepDays int
}

var _ mailsender.EmailSender = (*MailService)(nil)

// NewMailService - the service with the server config's settings (config:
// Transport set when the config has a transport), which win over the
// admin's.
// timeout bounds a send: a request to an HTTP mail API (Resend) or an
// SMTP session.
func NewMailService(repo entity.MailSettingsRepository, config entity.MailSettings, logger *slog.Logger, timeout time.Duration) *MailService {
	s := &MailService{repo: repo, configured: config, logger: logger.With("component", "mail"), newSender: func(m entity.MailSettings) mailsender.EmailSender {
		return buildMailSender(m, timeout)
	}}
	s.apply(entity.MailSettings{Enabled: true})
	return s
}

func buildMailSender(m entity.MailSettings, timeout time.Duration) mailsender.EmailSender {
	switch m.Transport {
	case entity.MailTransportResend:
		return mailsender.NewResendEmailSender(m.ResendAPIKey, m.FromAddress, timeout)
	case entity.MailTransportSMTP:
		return mailsender.NewSMTPEmailSender(mailsender.EmailConfig{
			SMTPHost: m.SMTPHost, SMTPPort: m.SMTPPort, SMTPUser: m.SMTPUser, SMTPPassword: m.SMTPPassword, FromAddress: m.FromAddress,
			Timeout: timeout,
		})
	}
	return nil
}

// mailConfigured - whether settings name a usable transport.
func mailConfigured(m entity.MailSettings) bool {
	if strings.TrimSpace(m.FromAddress) == "" {
		return false
	}
	switch m.Transport {
	case entity.MailTransportResend:
		return strings.TrimSpace(m.ResendAPIKey) != ""
	case entity.MailTransportSMTP:
		return strings.TrimSpace(m.SMTPHost) != "" && m.SMTPPort > 0
	}
	return false
}

// Reload reads the saved settings and makes them current.
func (s *MailService) Reload(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}
	saved, err := s.repo.Get(ctx)
	if err != nil {
		return err
	}
	s.apply(*saved)
	return nil
}

// Saved - the settings saved in the admin (not the server config's).
func (s *MailService) Saved(ctx context.Context) (*entity.MailSettings, error) {
	saved, err := s.repo.Get(ctx)
	return entity.Wrapped(saved, err, "loading the mail settings")
}

// Save stores the settings and makes them current.
func (s *MailService) Save(ctx context.Context, settings entity.MailSettings) error {
	if err := s.repo.Update(ctx, &settings); err != nil {
		return fmt.Errorf("saving the mail settings: %w", err)
	}
	s.apply(settings)
	return nil
}

func (s *MailService) apply(saved entity.MailSettings) {
	// The server config's transport first: the admin can't replace what
	// the server's owner set (and send the site's email elsewhere).
	effective, source := s.configured, MailSourceConfig
	if !mailConfigured(effective) {
		effective, source = saved, MailSourceAdmin
		if !mailConfigured(effective) {
			source = MailSourceNone
		}
	}
	effective.Enabled = saved.Enabled
	var sender mailsender.EmailSender
	if effective.Enabled && source != MailSourceNone {
		sender = s.newSender(effective)
	}
	status := MailStatus{Enabled: effective.Enabled, Source: source}
	if source != MailSourceNone {
		status.Transport, status.FromAddress = effective.Transport, effective.FromAddress
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sender, s.status, s.logKeepDays = sender, status, saved.LogKeepDays
}

// Status - the settings in effect.
func (s *MailService) Status() MailStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *MailService) current() mailsender.EmailSender {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sender
}

// SendPlain implements mailsender.EmailSender: a message as given, of
// none of the site's emails (a form's notice); with email off nothing is
// sent.
func (s *MailService) SendPlain(to, subject, body string) error {
	return s.deliver("", to, subject, body, false)
}

// SendTest sends a test email; unlike SendPlain it reports ErrMailOff when
// nothing would be sent.
func (s *MailService) SendTest(to, subject, body string) error {
	return s.deliver("", to, subject, body, true)
}

// deliver sends a message through the transport in effect and records it
// in the mail log (key - the site's email it is, "" - none). With email
// off nothing is sent: a test gets ErrMailOff, the others nil.
func (s *MailService) deliver(key, to, subject, body string, test bool) error {
	entry := entity.MailLogEntry{SentAt: time.Now(), Key: key, To: to, Subject: subject, Status: entity.MailStatusSent}
	var err error
	if sender := s.current(); sender == nil {
		entry.Status = entity.MailStatusUnsent
		if test {
			err = ErrMailOff
		} else {
			s.logger.Warn("email not sent: sending is off or not configured", "subject", subject)
		}
	} else if err = sender.SendPlain(to, subject, body); err != nil {
		entry.Status, entry.Error = entity.MailStatusFailed, err.Error()
	}
	s.record(entry)
	return err
}

// record adds e to the mail log, unless there is none (SetLog, the
// settings' LogKeepDays); a failure is logged: the email is gone anyway.
func (s *MailService) record(e entity.MailLogEntry) {
	if s.log == nil || s.LogKeepDays() <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	LogFallback(s.logger, s.log.Add(ctx, e), "recording an email in the mail log", "email", e.Key)
}

// SetLog gives the service the mail log.
func (s *MailService) SetLog(repo entity.MailLogRepository) { s.log = repo }

// Log - the mail log (nil - none).
func (s *MailService) Log() entity.MailLogRepository { return s.log }

// LogKeepDays - how many days the mail log keeps an email (0 - no log).
func (s *MailService) LogKeepDays() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logKeepDays
}

// LogPurgeJob - the scheduler's job deleting the mail log's entries older
// than LogKeepDays (all of them with no log), hourly.
func (s *MailService) LogPurgeJob(now func() time.Time) Job {
	return Job{Name: "mail_log_purge", Every: time.Hour, Run: func(ctx context.Context) error {
		if s.log == nil {
			return nil
		}
		return s.log.DeleteBefore(ctx, now().AddDate(0, 0, -s.LogKeepDays()))
	}}
}
