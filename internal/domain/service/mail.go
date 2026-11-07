package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
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
	Enabled bool
	// Transport - the way of sending in effect, TransportTitle - its name
	// on the page.
	Transport      string
	TransportTitle string
	FromAddress    string
	Source         string
	// Missing - the saved way of sending when this binary has none of
	// that name (its module left the binary): nothing is sent through it.
	Missing string
	// ModuleOff - the transport in effect is a module's that is
	// disabled: no email is sent until it is enabled again.
	ModuleOff bool
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
	newSender func(entity.MailSettings) (mailsender.EmailSender, error)
	// templates - what the Mail page changed of the emails (SetTemplates).
	templates entity.MailTemplateRepository
	// log - the mail log (SetLog); nil - none.
	log entity.MailLogRepository
	// env - the modules' ways of sending's settings the server's
	// environment gives (SetTransportEnv): transport → setting → value.
	env map[string]map[string]string

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
// timeout bounds a send: an SMTP session or a module's transport's
// request.
func NewMailService(repo entity.MailSettingsRepository, config entity.MailSettings, logger *slog.Logger, timeout time.Duration) *MailService {
	s := &MailService{repo: repo, configured: config, logger: logger.With("component", "mail"), newSender: func(m entity.MailSettings) (mailsender.EmailSender, error) {
		return buildMailSender(m, timeout)
	}}
	s.apply(entity.MailSettings{Enabled: true})
	return s
}

// buildMailSender - the sender of the settings' transport: SMTP, or a
// module's (MailTransport.New).
func buildMailSender(m entity.MailSettings, timeout time.Duration) (mailsender.EmailSender, error) {
	if m.Transport == entity.MailTransportSMTP {
		return mailsender.NewSMTPEmailSender(mailsender.EmailConfig{
			SMTPHost: m.SMTPHost, SMTPPort: m.SMTPPort, SMTPUser: m.SMTPUser, SMTPPassword: m.SMTPPassword, FromAddress: m.FromAddress,
			Timeout: timeout,
		}), nil
	}
	t, ok := MailTransportByName(m.Transport)
	if !ok {
		return nil, fmt.Errorf("no mail transport %q in this binary", m.Transport)
	}
	sender, err := t.New(m.TransportSettings[m.Transport], m.FromAddress, timeout)
	if err != nil {
		return nil, fmt.Errorf("the mail transport %q: %w", m.Transport, err)
	}
	return sender, nil
}

// mailConfigured - whether settings name a usable transport: SMTP with
// its server, or a module's this binary has, with its required settings
// (whether its module is enabled is asked when sending: it may change).
func mailConfigured(m entity.MailSettings) bool {
	if strings.TrimSpace(m.FromAddress) == "" {
		return false
	}
	if m.Transport == entity.MailTransportSMTP {
		return strings.TrimSpace(m.SMTPHost) != "" && m.SMTPPort > 0
	}
	t, ok := MailTransportByName(m.Transport)
	return ok && mailTransportReady(t, m.TransportSettings[m.Transport])
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
		// The admin's, its transport's settings the environment gives
		// taken from there (a key without the config's sender address).
		effective, source = s.withEnv(saved), MailSourceAdmin
		if !mailConfigured(effective) {
			source = MailSourceNone
		}
	}
	effective.Enabled = saved.Enabled
	var sender mailsender.EmailSender
	if effective.Enabled && source != MailSourceNone {
		var err error
		if sender, err = s.newSender(effective); err != nil {
			s.logger.Error("error setting up the mail transport: no email is sent", "error", err)
			sender = nil
		}
	}
	status := MailStatus{Enabled: effective.Enabled, Source: source}
	if source != MailSourceNone {
		status.Transport, status.TransportTitle, status.FromAddress = effective.Transport, MailTransportTitle(effective.Transport), effective.FromAddress
	}
	if t := saved.Transport; t != "" && t != entity.MailTransportSMTP {
		if _, ok := MailTransportByName(t); !ok {
			status.Missing = t
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sender, s.status, s.logKeepDays = sender, status, saved.LogKeepDays
}

// SetTransportEnv - the modules' ways of sending's settings the server's
// environment gives (MailTransportEnv), set before Reload. They fill the
// settings saved in the admin too - the environment's value wins - and
// the Mail page shows them as the server's: a key in the environment
// without the config's sender address (no server transport then) still
// sends once the admin chooses its transport and a sender.
func (s *MailService) SetTransportEnv(env map[string]map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.env = env
}

// EnvSetting - whether the server's environment sets the transport's
// setting.
func (s *MailService) EnvSetting(transport, field string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.env[transport][field] != ""
}

// EnvTransport - the first module's way of sending whose required
// settings the environment all sets ("" - none): the Mail page's choice
// while none is saved.
func (s *MailService) EnvTransport() string {
	s.mu.RLock()
	env := s.env
	s.mu.RUnlock()
	for _, t := range MailTransports() {
		if len(env[t.Name]) > 0 && mailTransportReady(t, env[t.Name]) {
			return t.Name
		}
	}
	return ""
}

// withEnv - m with the environment's values over its transports'
// settings (a copy: m's maps aren't changed).
func (s *MailService) withEnv(m entity.MailSettings) entity.MailSettings {
	s.mu.RLock()
	env := s.env
	s.mu.RUnlock()
	if len(env) == 0 {
		return m
	}
	settings := make(map[string]map[string]string, len(m.TransportSettings)+len(env))
	for name, values := range m.TransportSettings {
		settings[name] = maps.Clone(values)
	}
	for name, values := range env {
		if settings[name] == nil {
			settings[name] = map[string]string{}
		}
		maps.Copy(settings[name], values)
	}
	m.TransportSettings = settings
	return m
}

// Status - the settings in effect.
func (s *MailService) Status() MailStatus {
	s.mu.RLock()
	st := s.status
	s.mu.RUnlock()
	st.ModuleOff = st.Transport != "" && !mailTransportOn(st.Transport)
	return st
}

// current - the sender in effect; nil while sending is off, not
// configured, or the transport's module is disabled.
func (s *MailService) current() mailsender.EmailSender {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.sender == nil || !mailTransportOn(s.status.Transport) {
		return nil
	}
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
