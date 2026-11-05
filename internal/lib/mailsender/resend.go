package mailsender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const resendEmailsURL = "https://api.resend.com/emails"

// ResendEmailSender sends mail via Resend HTTP API (https://resend.com/docs/api-reference/emails/send-email).
type ResendEmailSender struct {
	APIKey      string
	FromAddress string
	HTTPClient  *http.Client
}

// NewResendEmailSender creates a sender. apiKey is the Resend API key; fromAddress must be an allowed sender in Resend;
// timeout bounds a request to Resend.
func NewResendEmailSender(apiKey, fromAddress string, timeout time.Duration) *ResendEmailSender {
	return &ResendEmailSender{
		APIKey:      apiKey,
		FromAddress: fromAddress,
		HTTPClient:  &http.Client{Timeout: timeout},
	}
}

func (s *ResendEmailSender) client() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return http.DefaultClient
}

func (s *ResendEmailSender) postEmail(to, subject, body string) error {
	if s.APIKey == "" || s.FromAddress == "" {
		return fmt.Errorf("resend: api key and from_address required")
	}
	to, err := recipient(to)
	if err != nil {
		return err
	}
	payload := struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}{
		From:    s.FromAddress,
		To:      []string{to},
		Subject: oneLine(subject),
		Text:    body,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, resendEmailsURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("resend: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// SendPlain sends UTF-8 plain text email.
func (s *ResendEmailSender) SendPlain(to, subject, body string) error {
	return s.postEmail(to, subject, body)
}
