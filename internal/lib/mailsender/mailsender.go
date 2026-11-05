// Package mailsender sends transactional email via SMTP or the Resend HTTP API.
package mailsender

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// EmailConfig holds SMTP configuration (can be populated from config package)
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	FromAddress  string
	// Timeout bounds the whole SMTP session (0 - defaultSMTPTimeout).
	Timeout time.Duration
}

// defaultSMTPTimeout - the session's limit when the config sets none.
const defaultSMTPTimeout = 30 * time.Second

// EmailSender sends emails (password reset, notifications, etc.)
type EmailSender interface {
	// SendPlain sends a transactional email with explicit subject and body (UTF-8 plain text).
	SendPlain(to, subject, body string) error
}

// ErrBadAddress - the recipient isn't one e-mail address.
var ErrBadAddress = errors.New("not an e-mail address")

// recipient - to as a bare address; ErrBadAddress for a list, a name with
// it, or anything with a line break (it would add headers).
func recipient(to string) (string, error) {
	a, err := mail.ParseAddress(to)
	if err != nil || a.Name != "" || a.Address != strings.TrimSpace(to) {
		return "", fmt.Errorf("%w: %q", ErrBadAddress, to)
	}
	return a.Address, nil
}

// oneLine - subject with its line breaks turned into spaces: a subject
// is one header line.
func oneLine(subject string) string {
	return strings.Join(strings.FieldsFunc(subject, func(r rune) bool { return r == '\r' || r == '\n' }), " ")
}

// buildMessage - the message from sender to to: the headers (the subject
// encoded when it isn't ASCII, Date, Message-ID) and body as quoted-
// printable UTF-8 text with CRLF line ends.
func buildMessage(sender *mail.Address, to, subject, body string, now time.Time) ([]byte, error) {
	_, domain, _ := strings.Cut(sender.Address, "@")
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	for _, h := range [][2]string{
		{"From", sender.String()},
		{"To", to},
		{"Subject", mime.QEncoding.Encode("utf-8", oneLine(subject))},
		{"Date", now.Format(time.RFC1123Z)},
		{"Message-ID", "<" + hex.EncodeToString(id) + "@" + domain + ">"},
		{"MIME-Version", "1.0"},
		{"Content-Type", "text/plain; charset=UTF-8"},
		{"Content-Transfer-Encoding", "quoted-printable"},
	} {
		b.WriteString(h[0] + ": " + h[1] + "\r\n")
	}
	b.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&b)
	if _, err := qp.Write([]byte(strings.ReplaceAll(body, "\r\n", "\n"))); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// SMTPEmailSender sends emails via SMTP
type SMTPEmailSender struct {
	cfg EmailConfig
}

// NewSMTPEmailSender creates a new SMTP email sender
func NewSMTPEmailSender(cfg EmailConfig) *SMTPEmailSender {
	return &SMTPEmailSender{cfg: cfg}
}

// SendPlain sends UTF-8 plain text email.
func (s *SMTPEmailSender) SendPlain(to, subject, body string) error {
	if s.cfg.SMTPHost == "" || s.cfg.FromAddress == "" {
		return fmt.Errorf("email not configured: smtp_host and from_address required")
	}
	to, err := recipient(to)
	if err != nil {
		return err
	}
	sender, err := mail.ParseAddress(s.cfg.FromAddress)
	if err != nil {
		return fmt.Errorf("the sender %q: %w", s.cfg.FromAddress, err)
	}
	msg, err := buildMessage(sender, to, subject, body, time.Now())
	if err != nil {
		return err
	}
	return s.send(sender.Address, to, msg)
}

// send - smtp.SendMail within the config's timeout: STARTTLS when the
// server offers it, then AUTH when there are credentials.
func (s *SMTPEmailSender) send(from, to string, msg []byte) error {
	timeout := s.cfg.Timeout
	if timeout <= 0 {
		timeout = defaultSMTPTimeout
	}
	addr := net.JoinHostPort(s.cfg.SMTPHost, strconv.Itoa(s.cfg.SMTPPort))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		_ = conn.Close()
		return err
	}
	c, err := smtp.NewClient(conn, s.cfg.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.SMTPHost}); err != nil {
			return err
		}
	}
	if s.cfg.SMTPUser != "" && s.cfg.SMTPPassword != "" {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPassword, s.cfg.SMTPHost)); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// NoopEmailSender is a no-op implementation for when email is not configured
type NoopEmailSender struct{}

// SendPlain does nothing
func (NoopEmailSender) SendPlain(to, subject, body string) error {
	return nil
}
